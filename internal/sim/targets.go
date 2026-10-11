package sim

import (
	"fmt"
	"sort"
	"strings"

	"github.com/foutoucour/crackapack/internal/config"
)

// TargetOutcome holds the evaluation result for one [config.CalibrationTarget].
type TargetOutcome struct {
	Metric     string
	Comparison config.Comparison
	Min        *float64 // nil when Comparison is at_most
	Max        *float64 // nil when Comparison is at_least
	Value      float64
	Pass       bool
}

// EvaluateTargets evaluates each target in targets against summary by exact metric name.
// An unknown metric name is an error that names the metric and lists the available names sorted.
// A target whose required bound is missing (e.g. at_least without Min) is also an error.
// Returns an empty (non-nil) slice when targets is empty.
func EvaluateTargets(summary Summary, targets []config.CalibrationTarget) ([]TargetOutcome, error) {
	outcomes := make([]TargetOutcome, 0, len(targets))
	for _, t := range targets {
		value, ok := summary[t.Metric]
		if !ok {
			available := sortedKeys(summary)
			return nil, fmt.Errorf("metric %q not found; available: %s",
				t.Metric, strings.Join(available, ", "))
		}
		pass, err := evaluateTarget(t, value)
		if err != nil {
			return nil, fmt.Errorf("metric %q: %w", t.Metric, err)
		}
		outcomes = append(outcomes, TargetOutcome{
			Metric:     t.Metric,
			Comparison: t.Comparison,
			Min:        t.Min,
			Max:        t.Max,
			Value:      value,
			Pass:       pass,
		})
	}
	return outcomes, nil
}

// evaluateTarget checks a single target's pass/fail condition against value.
func evaluateTarget(t config.CalibrationTarget, value float64) (bool, error) {
	switch t.Comparison {
	case config.ComparisonAtLeast:
		if t.Min == nil {
			return false, fmt.Errorf("min bound required for at_least")
		}
		return value >= *t.Min, nil
	case config.ComparisonAtMost:
		if t.Max == nil {
			return false, fmt.Errorf("max bound required for at_most")
		}
		return value <= *t.Max, nil
	case config.ComparisonBetween:
		if t.Min == nil {
			return false, fmt.Errorf("min bound required for between")
		}
		if t.Max == nil {
			return false, fmt.Errorf("max bound required for between")
		}
		return value >= *t.Min && value <= *t.Max, nil
	default:
		return false, fmt.Errorf("unknown comparison %q", t.Comparison)
	}
}

// sortedKeys returns the keys of a Summary sorted ascending.
func sortedKeys(s Summary) []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
