package sim

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// RateResult holds the simulation result and calibration outcomes for one rate.
type RateResult struct {
	Rate     domain.CoinRate
	Result   Result
	Outcomes []TargetOutcome // empty when no targets are configured
	// Pass is true when all Outcomes pass. Meaningful only when len(Outcomes) > 0;
	// when there are no targets the field is false and PassingRates in SweepResult
	// is always empty.
	Pass bool
}

// SweepResult is the aggregate output of a sweep run.
type SweepResult struct {
	Rates []RateResult

	// PassingRates holds the rates where all targets pass, in grid order.
	// Empty when NoTargets is true or when no rate passes.
	PassingRates []domain.CoinRate

	// RangeMin and RangeMax are the first and last passing rates (0 when PassingRates is empty).
	RangeMin domain.CoinRate
	RangeMax domain.CoinRate

	// Contiguous is true when PassingRates forms a block with no gap in the original grid.
	// Meaningful only when len(PassingRates) >= 2.
	Contiguous bool

	// NoTargets is true when Sweep was called with an empty targets slice.
	// In this case PassingRates is always empty and the report states "no targets configured".
	NoTargets bool
}

// Sweep runs the provided run function for each rate in rates, in grid order,
// evaluates targets against each run's summary, and returns the aggregated SweepResult.
//
// If targets is non-empty, Sweep validates metric names against the first run's summary
// before running the remaining rates, so a typo in a metric name fails fast.
//
// The run closure receives the rate and must build a fresh engine, fresh rng, and
// fresh policy so that each rate's run is independent and deterministic.
//
// Errors from run include the rate in the message ("rate 50: …").
func Sweep(
	rates []domain.CoinRate,
	run func(domain.CoinRate) (Result, error),
	targets []config.CalibrationTarget,
) (SweepResult, error) {
	if len(rates) == 0 {
		return SweepResult{}, fmt.Errorf("sweep: rates must not be empty")
	}

	noTargets := len(targets) == 0
	rateResults := make([]RateResult, 0, len(rates))

	for i, rate := range rates {
		result, err := run(rate)
		if err != nil {
			return SweepResult{}, fmt.Errorf("rate %d: %w", rate, err)
		}

		// After the first run, validate metric names before continuing.
		// This aborts early on a bad metric name without running other rates.
		if !noTargets && i == 0 {
			if _, err := EvaluateTargets(result.Summary, targets); err != nil {
				return SweepResult{}, err
			}
		}

		rr := RateResult{
			Rate:   rate,
			Result: result,
		}

		if !noTargets {
			outcomes, err := EvaluateTargets(result.Summary, targets)
			if err != nil {
				return SweepResult{}, err
			}
			rr.Outcomes = outcomes
			rr.Pass = allOutcomesPass(outcomes)
		}

		rateResults = append(rateResults, rr)
	}

	sr := SweepResult{
		Rates:     rateResults,
		NoTargets: noTargets,
	}

	if !noTargets {
		sr.PassingRates = gatherPassingRates(rateResults)
		if len(sr.PassingRates) > 0 {
			sr.RangeMin = sr.PassingRates[0]
			sr.RangeMax = sr.PassingRates[len(sr.PassingRates)-1]
			sr.Contiguous = isContiguousInGrid(rates, sr.PassingRates)
		}
	}

	return sr, nil
}

// allOutcomesPass returns true when every outcome passes.
func allOutcomesPass(outcomes []TargetOutcome) bool {
	for _, o := range outcomes {
		if !o.Pass {
			return false
		}
	}
	return true
}

// gatherPassingRates returns the rates from rateResults where Pass is true, in order.
func gatherPassingRates(rateResults []RateResult) []domain.CoinRate {
	var passing []domain.CoinRate
	for _, rr := range rateResults {
		if rr.Pass {
			passing = append(passing, rr.Rate)
		}
	}
	return passing
}

// isContiguousInGrid returns true when the passing rates form a contiguous block
// (no gap) within the original grid. A single passing rate is always contiguous.
func isContiguousInGrid(grid []domain.CoinRate, passing []domain.CoinRate) bool {
	if len(passing) <= 1 {
		return true
	}
	passingSet := make(map[domain.CoinRate]bool, len(passing))
	for _, r := range passing {
		passingSet[r] = true
	}
	// Find the grid indices of the first and last passing rate.
	first, last := -1, -1
	for i, r := range grid {
		if passingSet[r] {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	// If any rate between first and last is not passing, there is a gap.
	for i := first + 1; i < last; i++ {
		if !passingSet[grid[i]] {
			return false
		}
	}
	return true
}
