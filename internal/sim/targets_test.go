package sim_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/sim"
)

func ptr(v float64) *float64 { return &v }

// TestEvaluateTargets covers all comparison operators, pass/fail states, unknown
// metric, and missing bound errors in one table-driven test.
func TestEvaluateTargets(t *testing.T) {
	t.Parallel()

	summary := sim.Summary{
		"final_balance":           500,
		"sub_coin_cards":          3,
		"daily_reward_commons":    2,
		"unsold_duplicate_copies": 0,
		"total_refused_trades":    10,
	}

	cases := []struct {
		name     string
		targets  []config.CalibrationTarget
		wantPass []bool // per-target; nil means error expected
		wantErr  string
	}{
		{
			name:     "empty targets returns empty outcomes",
			targets:  nil,
			wantPass: []bool{},
		},
		{
			name: "at_least pass",
			targets: []config.CalibrationTarget{
				{Metric: "final_balance", Comparison: config.ComparisonAtLeast, Min: ptr(100)},
			},
			wantPass: []bool{true},
		},
		{
			name: "at_least fail",
			targets: []config.CalibrationTarget{
				{Metric: "final_balance", Comparison: config.ComparisonAtLeast, Min: ptr(1000)},
			},
			wantPass: []bool{false},
		},
		{
			name: "at_most pass",
			targets: []config.CalibrationTarget{
				{Metric: "sub_coin_cards", Comparison: config.ComparisonAtMost, Max: ptr(5)},
			},
			wantPass: []bool{true},
		},
		{
			name: "at_most fail",
			targets: []config.CalibrationTarget{
				{Metric: "sub_coin_cards", Comparison: config.ComparisonAtMost, Max: ptr(2)},
			},
			wantPass: []bool{false},
		},
		{
			name: "between pass",
			targets: []config.CalibrationTarget{
				{Metric: "total_refused_trades", Comparison: config.ComparisonBetween, Min: ptr(5), Max: ptr(20)},
			},
			wantPass: []bool{true},
		},
		{
			name: "between fail below",
			targets: []config.CalibrationTarget{
				{Metric: "total_refused_trades", Comparison: config.ComparisonBetween, Min: ptr(50), Max: ptr(100)},
			},
			wantPass: []bool{false},
		},
		{
			name: "between fail above",
			targets: []config.CalibrationTarget{
				{Metric: "total_refused_trades", Comparison: config.ComparisonBetween, Min: ptr(0), Max: ptr(5)},
			},
			wantPass: []bool{false},
		},
		{
			name: "multiple targets",
			targets: []config.CalibrationTarget{
				{Metric: "sub_coin_cards", Comparison: config.ComparisonAtMost, Max: ptr(0)},   // fail
				{Metric: "final_balance", Comparison: config.ComparisonAtLeast, Min: ptr(100)}, // pass
			},
			wantPass: []bool{false, true},
		},
		{
			name: "unknown metric names metric and lists available",
			targets: []config.CalibrationTarget{
				{Metric: "bogus_metric", Comparison: config.ComparisonAtLeast, Min: ptr(1)},
			},
			wantErr: "bogus_metric",
		},
		{
			name: "unknown metric error lists available metrics",
			targets: []config.CalibrationTarget{
				{Metric: "no_such", Comparison: config.ComparisonAtLeast, Min: ptr(1)},
			},
			wantErr: "final_balance", // one of the available metrics must appear
		},
		{
			name: "at_least missing min",
			targets: []config.CalibrationTarget{
				{Metric: "final_balance", Comparison: config.ComparisonAtLeast},
			},
			wantErr: "min bound",
		},
		{
			name: "at_most missing max",
			targets: []config.CalibrationTarget{
				{Metric: "final_balance", Comparison: config.ComparisonAtMost},
			},
			wantErr: "max bound",
		},
		{
			name: "between missing min",
			targets: []config.CalibrationTarget{
				{Metric: "final_balance", Comparison: config.ComparisonBetween, Max: ptr(1000)},
			},
			wantErr: "min bound",
		},
		{
			name: "between missing max",
			targets: []config.CalibrationTarget{
				{Metric: "final_balance", Comparison: config.ComparisonBetween, Min: ptr(0)},
			},
			wantErr: "max bound",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			outcomes, err := sim.EvaluateTargets(summary, tc.targets)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(outcomes) != len(tc.wantPass) {
				t.Fatalf("outcomes len: got %d, want %d", len(outcomes), len(tc.wantPass))
			}
			for i, want := range tc.wantPass {
				if outcomes[i].Pass != want {
					t.Errorf("outcomes[%d].Pass = %v, want %v", i, outcomes[i].Pass, want)
				}
			}
		})
	}
}

// TestEvaluateTargets_emptyReturnsNonNilSlice ensures an empty target list
// returns a non-nil empty slice, not nil.
func TestEvaluateTargets_emptyReturnsNonNilSlice(t *testing.T) {
	t.Parallel()
	outcomes, err := sim.EvaluateTargets(sim.Summary{"x": 1}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcomes == nil {
		t.Error("want non-nil empty slice, got nil")
	}
	if len(outcomes) != 0 {
		t.Errorf("want len 0, got %d", len(outcomes))
	}
}
