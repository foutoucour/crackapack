package config

import (
	"fmt"
	"strings"
)

// validateTarget checks that a CalibrationTarget has a non-empty metric,
// a recognised comparison, and the bounds required by that comparison.
//
// For metrics whose name starts with "days_to_", an at_most or between
// target's max must not exceed runDays. A value above runDays encodes
// "not reached within the run" (internal/sim/metrics.go uses RunDays+1 as
// the sentinel), so no real observation can satisfy such a bound.
func validateTarget(idx int, t CalibrationTarget, runDays int) error {
	if t.Metric == "" {
		return fmt.Errorf("config: targets[%d].metric must not be empty", idx)
	}
	switch t.Comparison {
	case ComparisonAtLeast:
		if t.Min == nil {
			return fmt.Errorf("config: targets[%d].min is required for comparison %q", idx, t.Comparison)
		}
	case ComparisonAtMost:
		if t.Max == nil {
			return fmt.Errorf("config: targets[%d].max is required for comparison %q", idx, t.Comparison)
		}
		if strings.HasPrefix(t.Metric, "days_to_") && *t.Max > float64(runDays) {
			return fmt.Errorf(
				"config: targets[%d].max (%g) is above run_days (%d); a value above run_days means the milestone was not reached",
				idx, *t.Max, runDays,
			)
		}
	case ComparisonBetween:
		if t.Min == nil {
			return fmt.Errorf("config: targets[%d].min is required for comparison %q", idx, t.Comparison)
		}
		if t.Max == nil {
			return fmt.Errorf("config: targets[%d].max is required for comparison %q", idx, t.Comparison)
		}
		if *t.Min > *t.Max {
			return fmt.Errorf("config: targets[%d]: min (%g) must be <= max (%g)", idx, *t.Min, *t.Max)
		}
		if strings.HasPrefix(t.Metric, "days_to_") && *t.Max > float64(runDays) {
			return fmt.Errorf(
				"config: targets[%d].max (%g) is above run_days (%d); a value above run_days means the milestone was not reached",
				idx, *t.Max, runDays,
			)
		}
	default:
		return fmt.Errorf("config: targets[%d].comparison: unknown value %q; must be one of at_least, at_most, between", idx, t.Comparison)
	}
	return nil
}
