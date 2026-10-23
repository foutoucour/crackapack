package tune_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/tune"
)

func ptr(v float64) *float64 { return &v }

func TestDistance(t *testing.T) {
	t.Parallel()

	between := config.CalibrationTarget{Comparison: config.ComparisonBetween, Min: ptr(10), Max: ptr(18)}
	atMost := config.CalibrationTarget{Comparison: config.ComparisonAtMost, Max: ptr(3000)}
	atMostZero := config.CalibrationTarget{Comparison: config.ComparisonAtMost, Max: ptr(0)}
	atLeast := config.CalibrationTarget{Comparison: config.ComparisonAtLeast, Min: ptr(0.5)}
	point := config.CalibrationTarget{Comparison: config.ComparisonBetween, Min: ptr(4), Max: ptr(4)}

	tests := []struct {
		name  string
		band  config.CalibrationTarget
		value float64
		want  float64
	}{
		{"inside a between band", between, 14, 0},
		{"on the lower edge", between, 10, 0},
		{"below a between band", between, 6, 0.5},
		{"above a between band", between, 22, 0.5},
		{"above an at_most band is scaled by its bound", atMost, 4500, 0.5},
		{"inside an at_most band", atMost, 100, 0},
		{"at_most zero counts units", atMostZero, 14, 14},
		{"below an at_least band uses a floor of one", atLeast, 0, 0.5},
		{"a zero-width band is scaled by its bound", point, 6, 0.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tune.Distance(tc.value, tc.band); got != tc.want {
				t.Errorf("Distance(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

const problemYAML = `
scenarios:
  - {name: pace3, player: collector, sessions_per_day: 3, run_days: 40, sets: [RAV, TSP], supplementary: [TSB], per_set: true}
  - {name: pile, player: master-collector, sets: [RAV, TSP]}
goals:
  - {scenario: pace3, metric: "days_to_100pct.{set}", comparison: between, min: 10, max: 18}
  - {scenario: pile, metric: final_balance, comparison: at_most, max: 3000}
  - {scenario: pile, metric: sub_coin_cards, comparison: at_most, max: 0}
`

func TestProblemRuns_expandsAPerSetScenarioIntoOneRunPerSet(t *testing.T) {
	t.Parallel()

	problem, err := tune.LoadProblem(strings.NewReader(problemYAML))
	if err != nil {
		t.Fatalf("LoadProblem: %v", err)
	}
	runs := problem.Runs()

	if len(runs) != 3 {
		t.Fatalf("got %d runs, want 3 (two sets alone, then the pile scenario)", len(runs))
	}
	rav := runs[0]
	if rav.Name != "pace3/RAV" || len(rav.Sets) != 1 || rav.Sets[0] != "RAV" {
		t.Errorf("first run = %+v, want pace3/RAV on RAV alone", rav)
	}
	if rav.SessionsPerDay != 3 || rav.RunDays != 40 || rav.Supplementary[0] != "TSB" {
		t.Errorf("first run lost its scenario settings: %+v", rav)
	}
	if got := rav.Bands[0].Metric; got != "days_to_100pct.rav" {
		t.Errorf("metric = %q, want days_to_100pct.rav", got)
	}
	if got := runs[1].Bands[0].Metric; got != "days_to_100pct.tsp" {
		t.Errorf("second run metric = %q, want days_to_100pct.tsp", got)
	}
	pile := runs[2]
	if pile.Name != "pile" || len(pile.Sets) != 2 || len(pile.Bands) != 2 {
		t.Errorf("pile run = %+v, want both sets and both goals", pile)
	}
}

func TestLoadProblem_rejects(t *testing.T) {
	t.Parallel()

	const scenario = "scenarios:\n  - {name: a, player: collector, sets: [RAV]}\n"
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"no scenarios", "goals:\n  - {scenario: a, metric: m, comparison: at_most, max: 1}\n", "scenarios must not be empty"},
		{"no goals", scenario, "goals must not be empty"},
		{"unknown scenario", scenario + "goals:\n  - {scenario: b, metric: m, comparison: at_most, max: 1}\n", "unknown scenario"},
		{"unknown key", scenario + "goals:\n  - {scenario: a, metric: m, comparison: at_most, max: 1, weight: 2}\n", "weight"},
		{"band without its bound", scenario + "goals:\n  - {scenario: a, metric: m, comparison: between, min: 1}\n", "needs min and max"},
		{"inverted band", scenario + "goals:\n  - {scenario: a, metric: m, comparison: between, min: 5, max: 1}\n", "above max"},
		{"placeholder without per_set", scenario + "goals:\n  - {scenario: a, metric: \"m.{set}\", comparison: at_most, max: 1}\n", "per_set"},
		{"scenario without sets", "scenarios:\n  - {name: a, player: collector}\ngoals:\n  - {scenario: a, metric: m, comparison: at_most, max: 1}\n", "sets must not be empty"},
		{"duplicate scenario", scenario + "  - {name: a, player: collector, sets: [RAV]}\ngoals:\n  - {scenario: a, metric: m, comparison: at_most, max: 1}\n", "twice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tune.LoadProblem(strings.NewReader(tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("LoadProblem error = %v, want one naming %q", err, tc.want)
			}
		})
	}
}
