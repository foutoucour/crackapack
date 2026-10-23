package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// newSweepMemOutput returns a newOutput func that collects bytes in a map keyed
// by relative path. Suitable for in-memory sweep tests.
func newSweepMemOutput() (func(string) (io.WriteCloser, error), map[string]*bytes.Buffer) {
	store := make(map[string]*bytes.Buffer)
	fn := func(name string) (io.WriteCloser, error) {
		buf := &bytes.Buffer{}
		store[name] = buf
		return memWriteCloser{buf}, nil
	}
	return fn, store
}

// buildSweepArgs constructs a sweepArgs backed by an fstest.MapFS and in-memory output.
func buildSweepArgs(
	t *testing.T,
	cats []domain.SetCatalogue,
	supps []domain.SetCatalogue,
	configYAML string,
	rates []domain.CoinRate,
	stdout io.Writer,
) (sweepArgs, map[string]*bytes.Buffer) {
	t.Helper()

	allSnaps := make([]domain.PriceSnapshot, 0, len(cats)+len(supps))
	for _, c := range cats {
		allSnaps = append(allSnaps, domaintest.NewPrices(c, 1))
	}
	for _, c := range supps {
		allSnaps = append(allSnaps, domaintest.NewPrices(c, 2))
	}
	prices := domaintest.MergePrices(allSnaps...)

	mapFS, err := domaintest.NewMapFS(cats, supps, prices)
	if err != nil {
		t.Fatalf("NewMapFS: %v", err)
	}

	codes := make([]string, 0, len(cats))
	for _, c := range cats {
		codes = append(codes, c.Code)
	}
	suppCodes := make([]string, 0, len(supps))
	for _, c := range supps {
		suppCodes = append(suppCodes, c.Code)
	}

	newOutput, store := newSweepMemOutput()

	return sweepArgs{
		configReader: strings.NewReader(configYAML),
		catFS:        mapFS,
		setCodes:     codes,
		suppCodes:    suppCodes,
		pricesFS:     mapFS,
		pricesName:   "prices.json",
		player:       "collector",
		policies:     buildPolicies(),
		rates:        rates,
		newOutput:    newOutput,
		stdout:       stdout,
	}, store
}

// goldenConfig is the YAML used in the golden test.
// Grid [10, 100, 250]; the targets split the grid: only 100 passes both.
//
// Observed per-rate values (RAV, seed=42, run_days=5, prices seed=1):
//
//	rate  sub_coin_cards  final_balance
//	  10           0            55156
//	 100           0           547907
//	 250           0          1369344
//
// Target 1 (sub_coin_cards ≤ 0): passes at every rate, since the Counter pays one coin for a
// card priced at one cent, see ADR-0044.
// Target 2 (final_balance between 100000 and 1000000): passes at 100 only.
const goldenConfig = `run_days: 5
seed: 42
expensive_booster_value: 100000000
targets:
  - metric: sub_coin_cards
    comparison: at_most
    max: 0
  - metric: final_balance
    comparison: between
    min: 100000
    max: 1000000
`

// TestRunSweep_happyPath verifies the happy path: files are written and stdout contains the table.
func TestRunSweep_happyPath(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	a, store := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, shortConfig, []domain.CoinRate{10, 100}, &stdout)

	if err := runSweep(a); err != nil {
		t.Fatalf("runSweep: %v", err)
	}

	// Check per-rate files.
	for _, rate := range []int{10, 100} {
		key := fmt.Sprintf("rate-%d", rate)
		for _, name := range []string{key + "/result.json", key + "/days.csv"} {
			if store[name] == nil || store[name].Len() == 0 {
				t.Errorf("%s: missing or empty", name)
			}
		}
	}

	// sweep.json must exist.
	if store["sweep.json"] == nil || store["sweep.json"].Len() == 0 {
		t.Error("sweep.json: missing or empty")
	}

	// stdout must contain the table.
	if !strings.Contains(stdout.String(), "rate") {
		t.Errorf("stdout missing table header; got:\n%s", stdout.String())
	}
}

// TestRunSweep_deterministicOutput verifies that two identical runs produce byte-identical output.
func TestRunSweep_deterministicOutput(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	rates := []domain.CoinRate{10, 250}

	var stdout1, stdout2 strings.Builder
	a1, store1 := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, shortConfig, rates, &stdout1)
	a2, store2 := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, shortConfig, rates, &stdout2)

	if err := runSweep(a1); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if err := runSweep(a2); err != nil {
		t.Fatalf("run 2: %v", err)
	}

	// All output files must be byte-identical across runs.
	for _, name := range []string{
		"rate-10/result.json", "rate-10/days.csv",
		"rate-250/result.json", "rate-250/days.csv",
		"sweep.json",
	} {
		b1 := store1[name]
		b2 := store2[name]
		if b1 == nil {
			t.Errorf("run 1: %s missing", name)
			continue
		}
		if b2 == nil {
			t.Errorf("run 2: %s missing", name)
			continue
		}
		if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
			t.Errorf("%s differs between runs (not deterministic)", name)
		}
	}
	if stdout1.String() != stdout2.String() {
		t.Errorf("stdout differs between runs (not deterministic)")
	}
}

// TestRunSweep_unknownMetric_noReportsWritten verifies that an unknown metric in a
// target causes an error and no files are written (not even sweep.json or rate dirs).
func TestRunSweep_unknownMetric_noReportsWritten(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	configYAML := `run_days: 5
seed: 42
expensive_booster_value: 100000000
targets:
  - metric: no_such_metric
    comparison: at_most
    max: 0
`
	a, store := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, configYAML, []domain.CoinRate{10, 100}, &stdout)

	err := runSweep(a)
	if err == nil {
		t.Fatal("want error for unknown metric, got nil")
	}
	if !strings.Contains(err.Error(), "no_such_metric") {
		t.Errorf("error %q does not mention the unknown metric", err.Error())
	}
	if len(store) != 0 {
		t.Errorf("want no files written on unknown metric error; got keys: %v", storeKeys(store))
	}
}

// TestRunSweep_noTargets verifies that a sweep with no targets produces the
// "no targets configured" notice and never lists passing rates in sweep.json.
func TestRunSweep_noTargets(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	a, store := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, shortConfig, []domain.CoinRate{10, 100}, &stdout)

	if err := runSweep(a); err != nil {
		t.Fatalf("runSweep: %v", err)
	}

	if !strings.Contains(stdout.String(), "no targets configured") {
		t.Errorf("stdout missing 'no targets configured'; got:\n%s", stdout.String())
	}

	// sweep.json must have no_targets:true and passing_rates:[].
	sweepJSON := store["sweep.json"]
	if sweepJSON == nil {
		t.Fatal("sweep.json missing")
	}
	var report struct {
		NoTargets    bool              `json:"no_targets"`
		PassingRates []domain.CoinRate `json:"passing_rates"`
	}
	if err := json.Unmarshal(sweepJSON.Bytes(), &report); err != nil {
		t.Fatalf("parse sweep.json: %v", err)
	}
	if !report.NoTargets {
		t.Error("sweep.json: no_targets should be true")
	}
	if len(report.PassingRates) != 0 {
		t.Errorf("sweep.json: passing_rates should be empty, got %v", report.PassingRates)
	}
}

// TestRunSweep_ratesFlagOverride verifies that a.rates overrides the config sweep_rates.
func TestRunSweep_ratesFlagOverride(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	// Override to only rate 250.
	a, store := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, shortConfig, []domain.CoinRate{250}, &stdout)

	if err := runSweep(a); err != nil {
		t.Fatalf("runSweep: %v", err)
	}

	if store["rate-250/result.json"] == nil {
		t.Error("rate-250/result.json missing")
	}
	// No other rate directories should exist.
	for name := range store {
		if strings.HasPrefix(name, "rate-") && !strings.HasPrefix(name, "rate-250") {
			t.Errorf("unexpected file for non-override rate: %s", name)
		}
	}
}

// TestCmdSweep_missingRequiredFlags verifies that missing required flags produce
// errors mentioning the flag name.
func TestCmdSweep_missingRequiredFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{
			name:    "missing --catalogue-dir",
			args:    []string{"--sets", "RAV", "--prices", "p.json", "--out", "out"},
			wantMsg: "--catalogue-dir",
		},
		{
			name:    "missing --sets",
			args:    []string{"--catalogue-dir", ".", "--prices", "p.json", "--out", "out"},
			wantMsg: "--sets",
		},
		{
			name:    "missing --prices",
			args:    []string{"--catalogue-dir", ".", "--sets", "RAV", "--out", "out"},
			wantMsg: "--prices",
		},
		{
			name:    "missing --out",
			args:    []string{"--catalogue-dir", ".", "--sets", "RAV", "--prices", "p.json"},
			wantMsg: "--out",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr strings.Builder
			err := cmdSweep(tc.args, io.Discard, &stderr)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantMsg)
			}
			if !strings.HasPrefix(err.Error(), "crackapack sweep:") {
				t.Errorf("error %q does not have 'crackapack sweep:' prefix", err.Error())
			}
		})
	}
}

// TestParseRates covers the --rates flag parser.
func TestParseRates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		raw       string
		want      []domain.CoinRate
		wantErrOf string
	}{
		{name: "single", raw: "100", want: []domain.CoinRate{100}},
		{name: "multiple", raw: "10,50,250", want: []domain.CoinRate{10, 50, 250}},
		{name: "spaces", raw: " 10 , 50 ", want: []domain.CoinRate{10, 50}},
		{name: "invalid token", raw: "10,abc,50", wantErrOf: "abc"},
		{name: "empty string", raw: ",,,", wantErrOf: "sweep_rates must not be empty"},
		{name: "duplicate", raw: "10,50,50", wantErrOf: "sweep_rates[2]"},
		{name: "descending", raw: "250,50", wantErrOf: "sweep_rates[1]"},
		{name: "zero rate", raw: "0,50", wantErrOf: "sweep_rates[0]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rates, err := parseRates(tc.raw)
			if tc.wantErrOf != "" {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrOf) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrOf)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rates) != len(tc.want) {
				t.Fatalf("rates len: got %d, want %d", len(rates), len(tc.want))
			}
			for i := range tc.want {
				if rates[i] != tc.want[i] {
					t.Errorf("rates[%d]: got %d, want %d", i, rates[i], tc.want[i])
				}
			}
		})
	}
}

// TestRunSweep_sweepJSONContainsPerRateSummary verifies that sweep.json contains
// the per-rate summary map with expected metric keys.
func TestRunSweep_sweepJSONContainsPerRateSummary(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	a, store := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, shortConfig, []domain.CoinRate{100}, &stdout)

	if err := runSweep(a); err != nil {
		t.Fatalf("runSweep: %v", err)
	}

	var report struct {
		Rates []struct {
			Rate    int                `json:"rate"`
			Summary map[string]float64 `json:"summary"`
		} `json:"rates"`
	}
	if err := json.Unmarshal(store["sweep.json"].Bytes(), &report); err != nil {
		t.Fatalf("parse sweep.json: %v", err)
	}
	if len(report.Rates) != 1 {
		t.Fatalf("sweep.json rates: got %d, want 1", len(report.Rates))
	}
	for _, key := range []string{"final_balance", "sub_coin_cards", "total_refused_trades"} {
		if _, ok := report.Rates[0].Summary[key]; !ok {
			t.Errorf("sweep.json summary missing key %q", key)
		}
	}
}

// TestCmdSweep_goldenReport runs the sweep on a known setup and compares the
// stdout text table with the golden file in testdata/sweep_golden.txt.
//
// Run with -update to regenerate the golden file.
//
// Setup (RAV catalogue, seed=42, run_days=5, prices seed=1, grid [10, 100, 250]):
//
//	rate  sub_coin_cards  final_balance
//	  10           0            55156     → FAIL final_balance
//	 100           0           547907     → PASS both
//	 250           0          1369344     → FAIL final_balance
//
// Passing rates: 100 (range 100–100, contiguous).
func TestCmdSweep_goldenReport(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	a, _ := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, goldenConfig, []domain.CoinRate{10, 100, 250}, &stdout)

	if err := runSweep(a); err != nil {
		t.Fatalf("runSweep: %v", err)
	}

	got := stdout.String()
	goldenPath := filepath.Join("testdata", "sweep_golden.txt")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden file updated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
	}

	if string(want) != got {
		t.Errorf("stdout does not match golden file %s\n\n--- want ---\n%s\n--- got ---\n%s",
			goldenPath, string(want), got)
	}
}

// TestCmdSweep_sweepJSONDeterministicAcrossRuns verifies that sweep.json is
// byte-identical across two runs of the golden setup.
func TestCmdSweep_sweepJSONDeterministicAcrossRuns(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	rates := []domain.CoinRate{10, 100, 250}

	var stdout1, stdout2 strings.Builder
	a1, store1 := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, goldenConfig, rates, &stdout1)
	a2, store2 := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, goldenConfig, rates, &stdout2)

	if err := runSweep(a1); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if err := runSweep(a2); err != nil {
		t.Fatalf("run 2: %v", err)
	}

	b1 := store1["sweep.json"]
	b2 := store2["sweep.json"]
	if b1 == nil || b2 == nil {
		t.Fatal("sweep.json missing in one or both runs")
	}
	if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
		t.Error("sweep.json differs between runs (not deterministic)")
	}
}

// TestCmdSweep_sweepJSONGoldenPassingRates verifies the sweep.json for the golden
// setup carries the expected passing_rates and no_targets fields.
func TestCmdSweep_sweepJSONGoldenPassingRates(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	a, store := buildSweepArgs(t, []domain.SetCatalogue{rav}, nil, goldenConfig, []domain.CoinRate{10, 100, 250}, &stdout)

	if err := runSweep(a); err != nil {
		t.Fatalf("runSweep: %v", err)
	}

	var report struct {
		NoTargets    bool              `json:"no_targets"`
		PassingRates []domain.CoinRate `json:"passing_rates"`
		RangeMin     domain.CoinRate   `json:"range_min"`
		RangeMax     domain.CoinRate   `json:"range_max"`
	}
	if err := json.Unmarshal(store["sweep.json"].Bytes(), &report); err != nil {
		t.Fatalf("parse sweep.json: %v", err)
	}
	if report.NoTargets {
		t.Error("no_targets should be false")
	}
	if len(report.PassingRates) != 1 || report.PassingRates[0] != 100 {
		t.Errorf("passing_rates: got %v, want [100]", report.PassingRates)
	}
	if report.RangeMin != 100 || report.RangeMax != 100 {
		t.Errorf("range: got %d–%d, want 100–100", report.RangeMin, report.RangeMax)
	}
}

// storeKeys returns sorted keys of the output store for error messages.
func storeKeys(store map[string]*bytes.Buffer) []string {
	keys := make([]string, 0, len(store))
	for k := range store {
		keys = append(keys, k)
	}
	return keys
}
