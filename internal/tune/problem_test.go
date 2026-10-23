package tune_test

import (
	"math"
	"slices"
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

// A set whose sheets borrow cards from a donor set plays alone with the donor as a card pool, and
// the donor played alone must not also be its own pool, or its cards sit in two catalogues.
func TestProblemRuns_aPerSetRunDropsItsOwnSetFromTheSupplementaryPools(t *testing.T) {
	t.Parallel()
	problem, err := tune.LoadProblem(strings.NewReader(`
scenarios:
  - {name: pace, player: collector, sets: [AER, KLD], supplementary: [MPS, KLD], per_set: true}
goals:
  - {scenario: pace, metric: "days_to_100pct.{set}", comparison: between, min: 10, max: 18}
`))
	if err != nil {
		t.Fatalf("LoadProblem: %v", err)
	}

	runs := problem.Runs()

	if got := runs[0].Supplementary; !slices.Equal(got, []string{"MPS", "KLD"}) {
		t.Errorf("AER alone: supplementary = %v, want [MPS KLD]", got)
	}
	if got := runs[1].Supplementary; !slices.Equal(got, []string{"MPS"}) {
		t.Errorf("KLD alone: supplementary = %v, want [MPS]", got)
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
		{"no goals", scenario, "goals and shapes must not both be empty"},
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

// bellOfThree is three values at the quantiles of a normal curve of the given center and spread:
// the points a perfect bell of three sets would take. The z of 1/6 and 5/6 is 0.9674.
func bellOfThree(center, spread float64) []float64 {
	const z = 0.9674215661
	return []float64{center - z*spread, center, center + z*spread}
}

func TestShapeDistance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"a perfect bell", bellOfThree(14, 4), 0},
		{"the order of the values does not matter", []float64{bellOfThree(14, 4)[2], bellOfThree(14, 4)[0], 14}, 0},
		{"a bell one spread too late is one spread off per value", []float64{18 - 3.8697, 18, 18 + 3.8697}, 3},
		{"one value, at the center", []float64{14}, 0},
		{"one value, two spreads away", []float64{22}, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tune.ShapeDistance(tc.values, 14, 4); math.Abs(got-tc.want) > 1e-3 {
				t.Errorf("ShapeDistance(%v) = %v, want %v", tc.values, got, tc.want)
			}
		})
	}
}

func TestShapeDistance_aPileAtTheCenterIsNotABell(t *testing.T) {
	t.Parallel()
	bell := tune.ShapeDistance(bellOfThree(14, 4), 14, 4)

	pile := tune.ShapeDistance([]float64{14, 14, 14}, 14, 4)

	if pile <= bell {
		t.Errorf("a pile of equal values scores %v, a bell scores %v: the pile must be farther", pile, bell)
	}
}

func TestShapeDistance_aLongTailCostsMoreThanASmallOne(t *testing.T) {
	t.Parallel()
	short := tune.ShapeDistance([]float64{10, 14, 18}, 14, 4)

	long := tune.ShapeDistance([]float64{10, 14, 41}, 14, 4)

	if long <= short {
		t.Errorf("a set that finishes at 41 days scores %v, one at 18 scores %v: the tail must cost more", long, short)
	}
}

const shapeProblemYAML = `
scenarios:
  - {name: pace3, player: collector, sessions_per_day: 3, run_days: 40, sets: [RAV, TSP, GPT], per_set: true}
  - {name: pile, player: master-collector, sets: [RAV, TSP]}
goals:
  - {scenario: pile, metric: final_balance, comparison: at_most, max: 3000}
shapes:
  - {scenario: pace3, metric: "days_to_100pct.{set}", center: 14, spread: 4}
`

func TestProblemRuns_aShapeNamesTheMetricOfEachSetsRun(t *testing.T) {
	t.Parallel()
	problem, err := tune.LoadProblem(strings.NewReader(shapeProblemYAML))
	if err != nil {
		t.Fatalf("LoadProblem: %v", err)
	}

	runs := problem.Runs()

	if len(runs) != 4 {
		t.Fatalf("got %d runs, want 4 (three sets alone, then the pile)", len(runs))
	}
	tsp := runs[1]
	if len(tsp.Shapes) != 1 || tsp.Shapes[0].Metric != "days_to_100pct.tsp" {
		t.Errorf("TSP run shapes = %+v, want its own metric days_to_100pct.tsp", tsp.Shapes)
	}
	if runs[0].Shapes[0].Key != tsp.Shapes[0].Key {
		t.Errorf("the runs of one scenario must share the shape's key: %q and %q", runs[0].Shapes[0].Key, tsp.Shapes[0].Key)
	}
	if len(runs[3].Shapes) != 0 {
		t.Errorf("the pile run carries shapes %+v, want none", runs[3].Shapes)
	}
}

func TestLoadProblem_rejectsABadShape(t *testing.T) {
	t.Parallel()
	const scenarios = `
scenarios:
  - {name: pace3, player: collector, sets: [RAV, TSP], per_set: true}
  - {name: pile, player: master-collector, sets: [RAV, TSP]}
goals:
  - {scenario: pile, metric: final_balance, comparison: at_most, max: 3000}
shapes:
`
	tests := []struct {
		name  string
		shape string
		want  string
	}{
		{"unknown scenario", `  - {scenario: nope, metric: "days_to_100pct.{set}", center: 14, spread: 4}`, "unknown scenario"},
		{"scenario not per set", `  - {scenario: pile, metric: "days_to_100pct.{set}", center: 14, spread: 4}`, "per_set"},
		{"metric without the set placeholder", `  - {scenario: pace3, metric: days_to_100pct.rav, center: 14, spread: 4}`, "{set}"},
		{"no spread", `  - {scenario: pace3, metric: "days_to_100pct.{set}", center: 14, spread: 0}`, "spread"},
		{"no center", `  - {scenario: pace3, metric: "days_to_100pct.{set}", center: 0, spread: 4}`, "center"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tune.LoadProblem(strings.NewReader(scenarios + tc.shape + "\n"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("LoadProblem error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestLoadProblem_aShapeCanBeTheOnlyGoal(t *testing.T) {
	t.Parallel()
	_, err := tune.LoadProblem(strings.NewReader(`
scenarios:
  - {name: pace3, player: collector, sets: [RAV, TSP], per_set: true}
shapes:
  - {scenario: pace3, metric: "days_to_100pct.{set}", center: 14, spread: 4}
`))
	if err != nil {
		t.Errorf("LoadProblem: %v, want a shape to count as a goal", err)
	}
}
