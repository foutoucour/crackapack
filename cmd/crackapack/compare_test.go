package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// buildResultJSON runs a 5-day simulation with the given coin_rate and returns
// the bytes of result.json written by runSimulate.
func buildResultJSON(t *testing.T, coinRate int) []byte {
	t.Helper()
	cat := domaintest.NewRavnicaCatalogue()
	var stdout strings.Builder
	a, store := buildTestArgs(t, []domain.SetCatalogue{cat}, nil, &stdout)
	a.configReader = strings.NewReader(
		fmt.Sprintf("run_days: 5\nseed: 42\ncoin_rate: %d\n", coinRate),
	)
	if err := runSimulate(a); err != nil {
		t.Fatalf("runSimulate(coin_rate=%d): %v", coinRate, err)
	}
	return store["result.json"].Bytes()
}

// TestCompare_success runs two simulations with different coin rates and verifies
// that runCompare produces non-empty output containing the metric table header.
func TestCompare_success(t *testing.T) {
	t.Parallel()

	jsonA := buildResultJSON(t, 100)
	jsonB := buildResultJSON(t, 200)

	mapFS := fstest.MapFS{
		"a.json": {Data: jsonA},
		"b.json": {Data: jsonB},
	}

	var stdout strings.Builder
	a := compareArgs{
		fsA:    mapFS,
		nameA:  "a.json",
		fsB:    mapFS,
		nameB:  "b.json",
		stdout: &stdout,
	}

	if err := runCompare(a); err != nil {
		t.Fatalf("runCompare: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"metric", "delta", "final_balance"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout does not contain %q;\ngot:\n%s", want, out)
		}
	}
}

// TestCompare_withDaysCSV verifies that runCompare writes a delta CSV when
// daysCsvName is set, with columns prefixed "delta_".
func TestCompare_withDaysCSV(t *testing.T) {
	t.Parallel()

	jsonA := buildResultJSON(t, 100)
	jsonB := buildResultJSON(t, 200)

	mapFS := fstest.MapFS{
		"a.json": {Data: jsonA},
		"b.json": {Data: jsonB},
	}

	var stdout strings.Builder
	newOutput, store := newMemOutput()

	a := compareArgs{
		fsA:         mapFS,
		nameA:       "a.json",
		fsB:         mapFS,
		nameB:       "b.json",
		newOutput:   newOutput,
		daysCsvName: "delta.csv",
		stdout:      &stdout,
	}

	if err := runCompare(a); err != nil {
		t.Fatalf("runCompare: %v", err)
	}

	csvData := store["delta.csv"].Bytes()
	if len(csvData) == 0 {
		t.Fatal("delta.csv is empty")
	}
	if !strings.HasPrefix(string(csvData), "day,delta_") {
		t.Errorf("delta.csv header does not start with 'day,delta_';\ngot: %s",
			string(csvData)[:min(120, len(csvData))],
		)
	}
}

// TestCompare_missingFileA verifies that runCompare returns an error naming
// the missing file when file A does not exist.
func TestCompare_missingFileA(t *testing.T) {
	t.Parallel()

	jsonB := buildResultJSON(t, 100)
	mapFS := fstest.MapFS{
		"b.json": {Data: jsonB},
		// "a.json" intentionally absent.
	}

	var stdout strings.Builder
	a := compareArgs{
		fsA:    mapFS,
		nameA:  "a.json",
		fsB:    mapFS,
		nameB:  "b.json",
		stdout: &stdout,
	}

	err := runCompare(a)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "a.json") {
		t.Errorf("error %q does not mention 'a.json'", err.Error())
	}
}

// TestCompare_malformedFile verifies that runCompare returns an error naming
// the file when file A contains invalid JSON.
func TestCompare_malformedFile(t *testing.T) {
	t.Parallel()

	mapFS := fstest.MapFS{
		"a.json": {Data: []byte(`{invalid json`)},
		"b.json": {Data: []byte(`{invalid json`)},
	}

	var stdout strings.Builder
	a := compareArgs{
		fsA:    mapFS,
		nameA:  "a.json",
		fsB:    mapFS,
		nameB:  "b.json",
		stdout: &stdout,
	}

	err := runCompare(a)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "a.json") {
		t.Errorf("error %q does not mention 'a.json'", err.Error())
	}
}

// TestCmdCompare_wrongArgCount verifies that cmdCompare returns an error with
// the "crackapack compare:" prefix for wrong argument counts.
func TestCmdCompare_wrongArgCount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"no args", []string{}},
		{"one arg", []string{"a.json"}},
		{"three args", []string{"a.json", "b.json", "c.json"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr strings.Builder
			err := cmdCompare(tc.args, io.Discard, &stderr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "crackapack compare:") {
				t.Errorf("error %q does not have 'crackapack compare:' prefix", err.Error())
			}
		})
	}
}

// TestCmdCompare_flagAfterPositional verifies that placing a flag after the
// positional file paths returns a clear error.
func TestCmdCompare_flagAfterPositional(t *testing.T) {
	t.Parallel()

	var stderr strings.Builder
	// Go's flag package stops at the first non-flag arg; "--days-csv x" appears
	// after the two file paths and would be silently ignored without the check.
	err := cmdCompare([]string{"a.json", "b.json", "--days-csv", "x"}, io.Discard, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "flags must come before") {
		t.Errorf("error %q does not mention 'flags must come before'", err.Error())
	}
}

// TestCmdCompare_helpFlag verifies that -h returns nil and writes usage to stderr.
func TestCmdCompare_helpFlag(t *testing.T) {
	t.Parallel()
	var stderr strings.Builder
	err := cmdCompare([]string{"-h"}, io.Discard, &stderr)
	if err != nil {
		t.Errorf("want nil for -h, got %v", err)
	}
	if !strings.Contains(stderr.String(), "crackapack compare") {
		t.Errorf("usage not written to stderr; got: %s", stderr.String())
	}
}

// TestCmdCompare_endToEnd writes synthetic catalogues and real result files to
// a temp directory, runs cmdSimulate twice with different coin rates, and then
// runs cmdCompare with OS-level paths to verify the full OS-level path works.
func TestCmdCompare_endToEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Build synthetic test data and write to disk.
	cat := domaintest.NewRavnicaCatalogue()
	prices := domaintest.NewPrices(cat, 1)
	mapFS, err := domaintest.NewMapFS([]domain.SetCatalogue{cat}, nil, prices)
	if err != nil {
		t.Fatalf("NewMapFS: %v", err)
	}
	for name, f := range mapFS {
		if err := os.WriteFile(filepath.Join(dir, name), f.Data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	pricesPath := filepath.Join(dir, "prices.json")

	// Run two simulations with different coin rates.
	runSim := func(outDir, coinRate string) {
		t.Helper()
		cfgPath := filepath.Join(dir, "config_"+coinRate+".yaml")
		cfgYAML := "run_days: 5\nseed: 42\ncoin_rate: " + coinRate + "\n"
		if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		simArgs := []string{
			"--config", cfgPath,
			"--catalogue-dir", dir,
			"--sets", "RAV",
			"--prices", pricesPath,
			"--out", outDir,
		}
		if err := cmdSimulate(simArgs, io.Discard, io.Discard); err != nil {
			t.Fatalf("cmdSimulate(coin_rate=%s): %v", coinRate, err)
		}
	}

	outA := filepath.Join(dir, "outA")
	outB := filepath.Join(dir, "outB")
	runSim(outA, "100")
	runSim(outB, "200")

	pathA := filepath.Join(outA, "result.json")
	pathB := filepath.Join(outB, "result.json")
	deltaPath := filepath.Join(dir, "delta.csv")

	var stdout, stderr strings.Builder
	compareArgs := []string{"--days-csv", deltaPath, pathA, pathB}
	if err := cmdCompare(compareArgs, &stdout, &stderr); err != nil {
		t.Fatalf("cmdCompare: %v\nstderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "final_balance") {
		t.Errorf("stdout does not contain 'final_balance';\ngot:\n%s", stdout.String())
	}

	deltaData, err := os.ReadFile(deltaPath)
	if err != nil {
		t.Fatalf("read delta.csv: %v", err)
	}
	if !strings.HasPrefix(string(deltaData), "day,delta_") {
		t.Errorf("delta.csv header unexpected;\ngot: %s",
			string(deltaData)[:min(120, len(deltaData))],
		)
	}
}
