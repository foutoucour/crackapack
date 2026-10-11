package sim_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// makeResult returns a minimal Result whose Summary contains the provided metrics.
func makeResult(metrics map[string]float64) sim.Result {
	return sim.Result{Summary: sim.Summary(metrics)}
}

// atMost returns a CalibrationTarget using the at_most comparison.
func atMost(metric string, max float64) config.CalibrationTarget {
	return config.CalibrationTarget{Metric: metric, Comparison: config.ComparisonAtMost, Max: ptr(max)}
}

// makeRun builds a run function that returns successive results from a pre-built map keyed by rate.
func makeRun(results map[domain.CoinRate]sim.Result) func(domain.CoinRate) (sim.Result, error) {
	return func(rate domain.CoinRate) (sim.Result, error) {
		r, ok := results[rate]
		if !ok {
			return sim.Result{}, fmt.Errorf("no result for rate %d", rate)
		}
		return r, nil
	}
}

// TestSweep_noTargets verifies that Sweep with an empty target slice sets NoTargets,
// runs all rates, and never populates PassingRates.
func TestSweep_noTargets(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100}
	results := map[domain.CoinRate]sim.Result{
		10:  makeResult(map[string]float64{"sub_coin_cards": 5}),
		100: makeResult(map[string]float64{"sub_coin_cards": 0}),
	}

	sr, err := sim.Sweep(grid, makeRun(results), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sr.NoTargets {
		t.Error("NoTargets should be true")
	}
	if len(sr.PassingRates) != 0 {
		t.Errorf("PassingRates should be empty, got %v", sr.PassingRates)
	}
	if len(sr.Rates) != 2 {
		t.Errorf("Rates len: got %d, want 2", len(sr.Rates))
	}
	// Pass field must not be true when NoTargets (no vacuous pass).
	for _, rr := range sr.Rates {
		if rr.Pass {
			t.Errorf("rate %d: Pass should be false when no targets", rr.Rate)
		}
	}
}

// TestSweep_allPass verifies that when all rates pass every target,
// PassingRates contains all rates and Contiguous is true.
func TestSweep_allPass(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{50, 100, 250}
	results := map[domain.CoinRate]sim.Result{
		50:  makeResult(map[string]float64{"sub_coin_cards": 0}),
		100: makeResult(map[string]float64{"sub_coin_cards": 0}),
		250: makeResult(map[string]float64{"sub_coin_cards": 0}),
	}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}

	sr, err := sim.Sweep(grid, makeRun(results), targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sr.PassingRates) != 3 {
		t.Errorf("PassingRates: got %v, want [50 100 250]", sr.PassingRates)
	}
	if sr.RangeMin != 50 || sr.RangeMax != 250 {
		t.Errorf("range: got %d–%d, want 50–250", sr.RangeMin, sr.RangeMax)
	}
	if !sr.Contiguous {
		t.Error("Contiguous should be true")
	}
}

// TestSweep_somePass verifies partial passing rates and correct range.
func TestSweep_somePass(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 100, 250, 500}
	results := map[domain.CoinRate]sim.Result{
		10:  makeResult(map[string]float64{"sub_coin_cards": 5}),
		100: makeResult(map[string]float64{"sub_coin_cards": 1}),
		250: makeResult(map[string]float64{"sub_coin_cards": 0}),
		500: makeResult(map[string]float64{"sub_coin_cards": 0}),
	}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}

	sr, err := sim.Sweep(grid, makeRun(results), targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sr.PassingRates) != 2 {
		t.Errorf("PassingRates: got %v, want [250 500]", sr.PassingRates)
	}
	if sr.RangeMin != 250 || sr.RangeMax != 500 {
		t.Errorf("range: got %d–%d, want 250–500", sr.RangeMin, sr.RangeMax)
	}
	if !sr.Contiguous {
		t.Error("Contiguous should be true (250 and 500 are adjacent in the grid)")
	}
}

// TestSweep_nonePass verifies that when no rate passes, PassingRates is empty
// and range values are zero.
func TestSweep_nonePass(t *testing.T) {
	t.Parallel()

	grid := []domain.CoinRate{10, 50}
	results := map[domain.CoinRate]sim.Result{
		10: makeResult(map[string]float64{"sub_coin_cards": 5}),
		50: makeResult(map[string]float64{"sub_coin_cards": 2}),
	}
	targets := []config.CalibrationTarget{atMost("sub_coin_cards", 0)}

	sr, err := sim.Sweep(grid, makeRun(results), targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sr.PassingRates) != 0 {
		t.Errorf("PassingRates: got %v, want empty", sr.PassingRates)
	}
	if sr.RangeMin != 0 || sr.RangeMax != 0 {
		t.Errorf("range: got %d–%d, want 0–0", sr.RangeMin, sr.RangeMax)
	}
}

// TestSweep_withGap verifies that non-contiguous passing rates set Contiguous=false.
func TestSweep_withGap(t *testing.T) {
	t.Parallel()

	// Grid: 10, 50, 100, 250. Rates 10 and 250 pass; 50 and 100 fail.
	grid := []domain.CoinRate{10, 50, 100, 250}
	results := map[domain.CoinRate]sim.Result{
		10:  makeResult(map[string]float64{"x": 0}),
		50:  makeResult(map[string]float64{"x": 5}),
		100: makeResult(map[string]float64{"x": 3}),
		250: makeResult(map[string]float64{"x": 0}),
	}
	targets := []config.CalibrationTarget{atMost("x", 1)}

	sr, err := sim.Sweep(grid, makeRun(results), targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sr.PassingRates) != 2 {
		t.Errorf("PassingRates: got %v, want [10 250]", sr.PassingRates)
	}
	if sr.Contiguous {
		t.Error("Contiguous should be false (gap between 10 and 250)")
	}
}

// TestSweep_unknownMetricAbortsAfterFirstRun verifies that an unknown metric
// in a target causes an error after the first run and before the second.
func TestSweep_unknownMetricAbortsAfterFirstRun(t *testing.T) {
	t.Parallel()

	callCount := 0
	run := func(rate domain.CoinRate) (sim.Result, error) {
		callCount++
		return makeResult(map[string]float64{"real_metric": float64(rate)}), nil
	}

	grid := []domain.CoinRate{10, 100, 250}
	targets := []config.CalibrationTarget{
		atMost("no_such_metric", 0),
	}

	_, err := sim.Sweep(grid, run, targets)
	if err == nil {
		t.Fatal("want error for unknown metric, got nil")
	}
	if !strings.Contains(err.Error(), "no_such_metric") {
		t.Errorf("error %q does not mention the unknown metric", err.Error())
	}
	// Only the first run should have been called.
	if callCount != 1 {
		t.Errorf("run called %d times, want 1 (abort after first)", callCount)
	}
}

// TestSweep_runErrorIncludesRate verifies that a run error includes the rate value.
func TestSweep_runErrorIncludesRate(t *testing.T) {
	t.Parallel()

	run := func(rate domain.CoinRate) (sim.Result, error) {
		return sim.Result{}, fmt.Errorf("engine blew up")
	}
	grid := []domain.CoinRate{100}

	_, err := sim.Sweep(grid, run, nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("error %q does not contain the rate", err.Error())
	}
}

// TestSweep_emptyRates verifies that an empty grid returns an error.
func TestSweep_emptyRates(t *testing.T) {
	t.Parallel()

	_, err := sim.Sweep(nil, func(_ domain.CoinRate) (sim.Result, error) {
		return sim.Result{}, nil
	}, nil)
	if err == nil {
		t.Fatal("want error for empty rates, got nil")
	}
}
