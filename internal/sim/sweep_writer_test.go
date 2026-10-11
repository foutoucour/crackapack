package sim_test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// buildSweepResult builds a SweepResult from a list of (rate, metricValue, pass) tuples.
func buildSweepResult(
	grid []domain.CoinRate,
	metricName string,
	values map[domain.CoinRate]float64,
	passMap map[domain.CoinRate]bool,
	targets []config.CalibrationTarget,
) sim.SweepResult {
	rateResults := make([]sim.RateResult, len(grid))
	for i, rate := range grid {
		v := values[rate]
		pass := passMap[rate]
		var outcomes []sim.TargetOutcome
		if len(targets) > 0 {
			outcomes = []sim.TargetOutcome{
				{Metric: metricName, Comparison: targets[0].Comparison, Max: targets[0].Max, Min: targets[0].Min, Value: v, Pass: pass},
			}
		}
		rateResults[i] = sim.RateResult{
			Rate:     rate,
			Result:   sim.Result{Summary: sim.Summary{metricName: v, "final_balance": 1000, "sub_coin_cards": v, "daily_reward_commons": 2, "unsold_duplicate_copies": 0}},
			Outcomes: outcomes,
			Pass:     pass,
		}
	}

	var passing []domain.CoinRate
	for _, rate := range grid {
		if passMap[rate] {
			passing = append(passing, rate)
		}
	}

	sr := sim.SweepResult{
		Rates:        rateResults,
		PassingRates: passing,
		NoTargets:    len(targets) == 0,
	}
	if len(passing) > 0 {
		sr.RangeMin = passing[0]
		sr.RangeMax = passing[len(passing)-1]
		sr.Contiguous = true
	}
	return sr
}

// TestWriteSweepText_noTargets verifies the "no targets configured" report.
func TestWriteSweepText_noTargets(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100}
	sr := buildSweepResult(grid, "sub_coin_cards", map[domain.CoinRate]float64{10: 5, 100: 0}, map[domain.CoinRate]bool{10: false, 100: false}, nil)

	var out strings.Builder
	if err := sim.WriteSweepText(&out, sr, nil); err != nil {
		t.Fatalf("WriteSweepText: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "no targets configured") {
		t.Errorf("output does not contain 'no targets configured'; got:\n%s", text)
	}
	// Must not contain "Passing rates:" when no targets.
	if strings.Contains(text, "Passing rates:") {
		t.Errorf("output should not contain 'Passing rates:' when no targets; got:\n%s", text)
	}
}

// TestWriteSweepText_nonePass verifies the "Passing rates: none" line.
func TestWriteSweepText_nonePass(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	sr := buildSweepResult(grid, "sub_coin_cards", map[domain.CoinRate]float64{10: 5, 100: 1}, map[domain.CoinRate]bool{10: false, 100: false}, targets)

	var out strings.Builder
	if err := sim.WriteSweepText(&out, sr, targets); err != nil {
		t.Fatalf("WriteSweepText: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Passing rates: none") {
		t.Errorf("output does not contain 'Passing rates: none'; got:\n%s", text)
	}
}

// TestWriteSweepText_somePass verifies the passing-rate range line.
func TestWriteSweepText_somePass(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100, 250}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	sr := buildSweepResult(grid, "sub_coin_cards",
		map[domain.CoinRate]float64{10: 5, 100: 1, 250: 0},
		map[domain.CoinRate]bool{10: false, 100: false, 250: true},
		targets,
	)
	sr.PassingRates = []domain.CoinRate{250}
	sr.RangeMin, sr.RangeMax = 250, 250
	sr.Contiguous = true

	var out strings.Builder
	if err := sim.WriteSweepText(&out, sr, targets); err != nil {
		t.Fatalf("WriteSweepText: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Passing rates: 250") {
		t.Errorf("output does not contain 'Passing rates: 250'; got:\n%s", text)
	}
	if !strings.Contains(text, "250") {
		t.Errorf("output does not contain range; got:\n%s", text)
	}
}

// TestWriteSweepText_noRepeatHeadline verifies that a metric used as a target
// column is not repeated as a headline column.
func TestWriteSweepText_noRepeatHeadline(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{100}
	// sub_coin_cards is both the target metric and a headline metric.
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	sr := buildSweepResult(grid, "sub_coin_cards",
		map[domain.CoinRate]float64{100: 1},
		map[domain.CoinRate]bool{100: false},
		targets,
	)

	var out strings.Builder
	if err := sim.WriteSweepText(&out, sr, targets); err != nil {
		t.Fatalf("WriteSweepText: %v", err)
	}
	text := out.String()

	// Count header lines that contain "sub_coin_cards".
	lines := strings.Split(text, "\n")
	headerLine := lines[0]
	count := strings.Count(headerLine, "sub_coin_cards")
	if count != 1 {
		t.Errorf("sub_coin_cards appears %d times in header, want 1; header: %q", count, headerLine)
	}
}

// TestWriteSweepText_integralValues verifies that whole-number values print without decimals.
func TestWriteSweepText_integralValues(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{100}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	sr := buildSweepResult(grid, "sub_coin_cards",
		map[domain.CoinRate]float64{100: 5},
		map[domain.CoinRate]bool{100: false},
		targets,
	)

	var out strings.Builder
	if err := sim.WriteSweepText(&out, sr, targets); err != nil {
		t.Fatalf("WriteSweepText: %v", err)
	}
	// "5.00" must not appear; "5 FAIL" must appear.
	if strings.Contains(out.String(), "5.00") {
		t.Errorf("output contains '5.00'; want integer format; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "5 FAIL") {
		t.Errorf("output does not contain '5 FAIL'; got:\n%s", out.String())
	}
}

// TestWriteSweepJSON_noTargetsNeverListsPassingRates verifies that when no targets
// are configured, sweep.json carries no_targets:true and passing_rates:[].
func TestWriteSweepJSON_noTargetsNeverListsPassingRates(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100}
	sr := buildSweepResult(grid, "x", map[domain.CoinRate]float64{10: 1, 100: 0}, map[domain.CoinRate]bool{10: false, 100: false}, nil)

	var out strings.Builder
	if err := sim.WriteSweepJSON(&out, sr); err != nil {
		t.Fatalf("WriteSweepJSON: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, `"no_targets": true`) {
		t.Errorf("sweep.json missing no_targets:true; got:\n%s", text)
	}
	if !strings.Contains(text, `"passing_rates": []`) {
		t.Errorf("sweep.json should have empty passing_rates when no targets; got:\n%s", text)
	}
}

// TestWriteSweepJSON_deterministicOutput verifies two identical SweepResults produce identical JSON.
func TestWriteSweepJSON_deterministicOutput(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 250}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	sr := buildSweepResult(grid, "sub_coin_cards",
		map[domain.CoinRate]float64{10: 5, 250: 0},
		map[domain.CoinRate]bool{10: false, 250: true},
		targets,
	)
	sr.PassingRates = []domain.CoinRate{250}
	sr.RangeMin, sr.RangeMax = 250, 250
	sr.Contiguous = true

	var out1, out2 strings.Builder
	if err := sim.WriteSweepJSON(&out1, sr); err != nil {
		t.Fatalf("first WriteSweepJSON: %v", err)
	}
	if err := sim.WriteSweepJSON(&out2, sr); err != nil {
		t.Fatalf("second WriteSweepJSON: %v", err)
	}
	if out1.String() != out2.String() {
		t.Error("two calls with identical input produced different JSON")
	}
}

// TestWriteSweepText_unicodeHeaderAlignment reproduces defect 6.
// Headers containing multi-byte runes (≤, ≥, –) must produce table lines
// that all have the same rune-count width as the separator row.
func TestWriteSweepText_unicodeHeaderAlignment(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	// Header for the target column is "sub_coin_cards (≤0)" — contains multi-byte ≤.
	sr := buildSweepResult(grid, "sub_coin_cards",
		map[domain.CoinRate]float64{10: 5, 100: 0},
		map[domain.CoinRate]bool{10: false, 100: true},
		targets,
	)
	sr.PassingRates = []domain.CoinRate{100}
	sr.RangeMin, sr.RangeMax = 100, 100
	sr.Contiguous = true

	var out strings.Builder
	if err := sim.WriteSweepText(&out, sr, targets); err != nil {
		t.Fatalf("WriteSweepText: %v", err)
	}

	// Extract the table lines (lines containing | or +--).
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	var tableLines []string
	for _, line := range lines {
		if strings.ContainsAny(line, "|+") {
			tableLines = append(tableLines, line)
		}
	}
	// Separator line contains only ASCII dashes; its rune width == visual width.
	// Ensure there is a separator (second line).
	if len(tableLines) < 3 {
		t.Fatalf("expected header + separator + >=1 data rows, got %d table lines", len(tableLines))
	}
	separatorLine := tableLines[1]
	wantWidth := utf8.RuneCountInString(separatorLine)
	for i, line := range tableLines {
		got := utf8.RuneCountInString(line)
		if got != wantWidth {
			t.Errorf("table line %d rune width = %d, want %d (separator width)\nline:      %q\nseparator: %q",
				i, got, wantWidth, line, separatorLine)
		}
	}
}

// TestWriteSweepJSON_contiguous_nonePass reproduces defect 7:
// when no rates pass, "contiguous" must be absent from sweep.json.
func TestWriteSweepJSON_contiguous_nonePass(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	sr := buildSweepResult(grid, "sub_coin_cards",
		map[domain.CoinRate]float64{10: 5, 100: 1},
		map[domain.CoinRate]bool{10: false, 100: false},
		targets,
	)
	// PassingRates is empty; sr.Contiguous is false (zero value).

	var out strings.Builder
	if err := sim.WriteSweepJSON(&out, sr); err != nil {
		t.Fatalf("WriteSweepJSON: %v", err)
	}
	if strings.Contains(out.String(), `"contiguous"`) {
		t.Errorf("contiguous key must be absent when no rates pass; got:\n%s", out.String())
	}
}

// TestWriteSweepJSON_contiguous_true verifies that a contiguous passing range
// emits "contiguous": true.
func TestWriteSweepJSON_contiguous_true(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}
	sr := buildSweepResult(grid, "sub_coin_cards",
		map[domain.CoinRate]float64{10: 0, 100: 0},
		map[domain.CoinRate]bool{10: true, 100: true},
		targets,
	)
	sr.PassingRates = []domain.CoinRate{10, 100}
	sr.RangeMin, sr.RangeMax = 10, 100
	sr.Contiguous = true

	var out strings.Builder
	if err := sim.WriteSweepJSON(&out, sr); err != nil {
		t.Fatalf("WriteSweepJSON: %v", err)
	}

	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.String()), &report); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	raw, ok := report["contiguous"]
	if !ok {
		t.Fatal("contiguous key must be present when rates pass")
	}
	if string(raw) != "true" {
		t.Errorf("contiguous = %s, want true", raw)
	}
}

// TestWriteSweepJSON_contiguous_false verifies that a non-contiguous passing range
// emits "contiguous": false (not omitted).
func TestWriteSweepJSON_contiguous_false(t *testing.T) {
	t.Parallel()

	// Three-rate grid; first and last pass, middle fails → gap.
	grid := []domain.CoinRate{10, 50, 100}
	targets := []config.CalibrationTarget{atMost("x", 1)}
	sr := buildSweepResult(grid, "x",
		map[domain.CoinRate]float64{10: 0, 50: 5, 100: 0},
		map[domain.CoinRate]bool{10: true, 50: false, 100: true},
		targets,
	)
	sr.PassingRates = []domain.CoinRate{10, 100}
	sr.RangeMin, sr.RangeMax = 10, 100
	sr.Contiguous = false

	var out strings.Builder
	if err := sim.WriteSweepJSON(&out, sr); err != nil {
		t.Fatalf("WriteSweepJSON: %v", err)
	}

	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.String()), &report); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	raw, ok := report["contiguous"]
	if !ok {
		t.Fatal("contiguous key must be present (even when false) when passing rates exist but are non-contiguous")
	}
	if string(raw) != "false" {
		t.Errorf("contiguous = %s, want false", raw)
	}
}
