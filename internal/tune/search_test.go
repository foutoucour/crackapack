package tune_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/sim"
	"github.com/foutoucour/crackapack/internal/tune"
)

// fakeRunner reports metrics that are a pure function of the config and the seed, so a search
// over it runs in microseconds and its outcome can be worked out by hand.
type fakeRunner struct {
	calls atomic.Int64
	// failWhen makes the run fail for the configs it accepts.
	failWhen func(config.Config) bool
}

func (f *fakeRunner) Run(cfg config.Config, run tune.Run) (sim.Summary, error) {
	f.calls.Add(1)
	if f.failWhen != nil && f.failWhen(cfg) {
		return nil, errors.New("the run fell over")
	}
	// The noise stands for the seed's luck: -1, 0 or +1 around the buy percentage.
	noise := float64(cfg.Seed%3) - 1
	summary := sim.Summary{
		"buy":         float64(cfg.CounterBuyPct) + noise,
		"sessions":    float64(cfg.SessionsPerDay),
		"days":        float64(cfg.RunDays),
		"unrelated":   float64(cfg.WelcomeBonus),
		"player_name": float64(len(run.Player)),
	}
	// A set alone has a pace of its own: the buy percentage over ten, plus the set's offset.
	if len(run.Sets) == 1 {
		if offset, ok := paceOffset(run.Sets[0]); ok {
			summary["pace."+strings.ToLower(run.Sets[0])] = float64(cfg.CounterBuyPct)/10 + offset
		}
	}
	return summary, nil
}

// paceOffset spreads three sets around the middle, about as a bell of three would.
func paceOffset(set string) (float64, bool) {
	switch set {
	case "RAV":
		return -3, true
	case "TSP":
		return 0, true
	case "GPT":
		return 3, true
	}
	return 0, false
}

const searchSpaceYAML = `
params:
  - {path: counter_buy_pct, type: int, min: 100, max: 200}
  - {path: welcome_bonus, type: int, min: 0, max: 1000}
`

const searchProblemYAML = `
scenarios:
  - {name: only, player: collector, sessions_per_day: 6, run_days: 20, sets: [RAV]}
goals:
  - {scenario: only, metric: buy, comparison: between, min: 140, max: 150}
`

func searchFixture(t *testing.T, spaceYAML, problemYAML string) (config.Config, tune.Space, tune.Problem) {
	t.Helper()
	base, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	space, err := tune.LoadSpace(strings.NewReader(spaceYAML))
	if err != nil {
		t.Fatalf("LoadSpace: %v", err)
	}
	problem, err := tune.LoadProblem(strings.NewReader(problemYAML))
	if err != nil {
		t.Fatalf("LoadProblem: %v", err)
	}
	return base, space, problem
}

func options() tune.Options {
	return tune.Options{Seed: 11, Candidates: 120, KeepFraction: 0.25, Seeds: 3, ValidationSeeds: 4, Top: 3, Workers: 4}
}

func TestSearch_findsAConfigInsideTheBands(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	baseBuyPct := base.CounterBuyPct

	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	best := report.Best()
	if best.Loss == nil || *best.Loss != 0 {
		t.Fatalf("best loss = %v, want 0 (about 11%% of the draws land in the band, 30 survive)", best.Loss)
	}
	if !report.Feasible {
		t.Error("Feasible = false, want true: a candidate has loss 0")
	}
	if got := report.BestConfig.CounterBuyPct; got < 139 || got > 151 {
		t.Errorf("best config counter_buy_pct = %d, want about 140-150", got)
	}
	for _, w := range report.Windows {
		if w.Path == "counter_buy_pct" && (w.Min < 139 || w.Max > 151) {
			t.Errorf("feasible window of counter_buy_pct = %v-%v, want inside the band give or take the noise", w.Min, w.Max)
		}
	}
	if base.CounterBuyPct != baseBuyPct {
		t.Errorf("the base config changed: counter_buy_pct = %d, was %d", base.CounterBuyPct, baseBuyPct)
	}
}

func TestSearch_theMostInfluentialParameterComesFirst(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)

	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if got := report.Influence[0].Path; got != "counter_buy_pct" {
		t.Errorf("most influential = %s, want counter_buy_pct", got)
	}
	// The loss is V-shaped in the buy percentage, so its rank correlation is weak but the
	// unrelated parameter's is weaker still.
	if a, b := abs(report.Influence[0].Rho), abs(report.Influence[1].Rho); a <= b {
		t.Errorf("|rho| of the influential parameter %.2f is not above the unrelated one's %.2f", a, b)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestSearch_isReproducibleWhateverTheNumberOfWorkers(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)

	encode := func(workers int) string {
		opts := options()
		opts.Workers = workers
		report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, opts)
		if err != nil {
			t.Fatalf("Search with %d workers: %v", workers, err)
		}
		var buf bytes.Buffer
		if err := tune.WriteCandidatesJSON(&buf, report); err != nil {
			t.Fatalf("WriteCandidatesJSON: %v", err)
		}
		return buf.String()
	}

	if one, four := encode(1), encode(4); one != four {
		t.Error("the report differs between 1 and 4 workers")
	}
}

func TestSearch_validatesOnSeedsTheSearchNeverUsed(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)

	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(report.SearchSeeds) != 3 || len(report.ValidationSeeds) != 4 {
		t.Fatalf("seeds = %d search, %d validation, want 3 and 4", len(report.SearchSeeds), len(report.ValidationSeeds))
	}
	for _, v := range report.ValidationSeeds {
		if slices.Contains(report.SearchSeeds, v) {
			t.Errorf("validation seed %d was also a search seed", v)
		}
	}
	validated := 0
	for _, c := range report.Candidates {
		if c.ValidationLoss != nil {
			validated++
		}
	}
	if validated != 3 {
		t.Errorf("%d candidates validated, want the top 3", validated)
	}
}

func TestSearch_screensOnOneSeedBeforeScoringOnAll(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	runner := &fakeRunner{}
	opts := options()

	report, err := tune.Search(context.Background(), base, space, problem, runner, opts)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	survivors := 0
	for _, c := range report.Candidates {
		if c.Loss != nil {
			survivors++
		}
	}
	if survivors != 30 {
		t.Fatalf("%d survivors, want 30 (a quarter of 120)", survivors)
	}
	// One run per scenario: the base config on every search seed, every candidate on one seed,
	// the survivors on every search seed, the top on every validation seed.
	want := int64(opts.Seeds + opts.Candidates + survivors*opts.Seeds + opts.Top*opts.ValidationSeeds)
	if got := runner.calls.Load(); got != want {
		t.Errorf("%d simulations, want %d", got, want)
	}
}

func TestSearch_appliesTheScenarioSettings(t *testing.T) {
	t.Parallel()
	base, space, _ := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	problem, err := tune.LoadProblem(strings.NewReader(`
scenarios:
  - {name: only, player: collector, sessions_per_day: 6, run_days: 20, sets: [RAV]}
goals:
  - {scenario: only, metric: sessions, comparison: between, min: 6, max: 6}
  - {scenario: only, metric: days, comparison: between, min: 20, max: 20}
`))
	if err != nil {
		t.Fatalf("LoadProblem: %v", err)
	}

	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := *report.Baseline.Loss; got != 0 {
		t.Errorf("baseline loss = %v, want 0: the scenario's 6 visits and 20 days did not reach the run", got)
	}
}

func TestSearch_scoresAnInvalidConfigWorstAndNeverKeepsIt(t *testing.T) {
	t.Parallel()
	// A sell percentage above the buy percentage is refused by the config validation.
	base, space, problem := searchFixture(t, `
params:
  - {path: counter_buy_pct, type: int, min: 20, max: 200}
  - {path: counter_sell_pct, type: int, min: 50, max: 100}
`, searchProblemYAML)

	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if report.Invalid == 0 {
		t.Fatal("no candidate was invalid, want the draws with buy below sell to be")
	}
	for _, c := range report.Candidates {
		if c.Invalid != "" && (c.Loss != nil || c.ScreenLoss != tune.InvalidLoss) {
			t.Errorf("invalid candidate %d scored: %+v", c.ID, c)
		}
	}
	if report.Best().Invalid != "" {
		t.Error("the best candidate is an invalid one")
	}
}

func TestSearch_aRunThatFailsMakesTheCandidateInvalid(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	runner := &fakeRunner{failWhen: func(cfg config.Config) bool { return cfg.WelcomeBonus > 500 }}

	report, err := tune.Search(context.Background(), base, space, problem, runner, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if report.Invalid == 0 {
		t.Error("no candidate was invalid, want the draws with a welcome bonus above 500 to be")
	}
	if got := report.BestConfig.WelcomeBonus; got > 500 {
		t.Errorf("best config has a welcome bonus of %d, which fails to run", got)
	}
}

func TestSearch_failsOnABandNoRunReports(t *testing.T) {
	t.Parallel()
	base, space, _ := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	problem, err := tune.LoadProblem(strings.NewReader(`
scenarios:
  - {name: only, player: collector, sets: [RAV]}
goals:
  - {scenario: only, metric: no_such_metric, comparison: at_most, max: 1}
`))
	if err != nil {
		t.Fatalf("LoadProblem: %v", err)
	}

	_, err = tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err == nil || !strings.Contains(err.Error(), "no_such_metric") {
		t.Errorf("Search error = %v, want one naming no_such_metric", err)
	}
}

func TestSearch_failsWhenTheBaseConfigDoesNotRun(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	runner := &fakeRunner{failWhen: func(config.Config) bool { return true }}

	_, err := tune.Search(context.Background(), base, space, problem, runner, options())
	if err == nil || !strings.Contains(err.Error(), "base config") {
		t.Errorf("Search error = %v, want one about the base config", err)
	}
}

func TestSearch_stopsWhenTheContextIsCancelled(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tune.Search(ctx, base, space, problem, &fakeRunner{}, options())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Search error = %v, want context.Canceled", err)
	}
}

func TestSearch_rejectsBadOptions(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)

	for name, mutate := range map[string]func(*tune.Options){
		"no candidates":       func(o *tune.Options) { o.Candidates = 0 },
		"keep above one":      func(o *tune.Options) { o.KeepFraction = 1.5 },
		"no seeds":            func(o *tune.Options) { o.Seeds = 0 },
		"no validation seeds": func(o *tune.Options) { o.ValidationSeeds = 0 },
		"no top":              func(o *tune.Options) { o.Top = 0 },
		"no workers":          func(o *tune.Options) { o.Workers = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			opts := options()
			mutate(&opts)
			if _, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, opts); err == nil {
				t.Error("Search succeeded, want an error")
			}
		})
	}
}

func TestWriteBestYAML_loadsBackAsTheBestConfig(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	var buf bytes.Buffer
	if err := tune.WriteBestYAML(&buf, report); err != nil {
		t.Fatalf("WriteBestYAML: %v", err)
	}
	loaded, err := config.Load(&buf)
	if err != nil {
		t.Fatalf("config.Load of best.yaml: %v", err)
	}
	if loaded.CounterBuyPct != report.BestConfig.CounterBuyPct || loaded.WelcomeBonus != report.BestConfig.WelcomeBonus {
		t.Errorf("best.yaml loads as buy %d bonus %d, want %d and %d",
			loaded.CounterBuyPct, loaded.WelcomeBonus, report.BestConfig.CounterBuyPct, report.BestConfig.WelcomeBonus)
	}
}

func TestWriteText_namesTheBestCandidateTheWindowsAndTheInfluence(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, searchProblemYAML)
	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	var buf bytes.Buffer
	if err := tune.WriteText(&buf, report); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	text := buf.String()
	for _, want := range []string{"Best candidates", "Feasible window", "counter_buy_pct", "welcome_bonus", "What moves the loss", "meets every band"} {
		if !strings.Contains(text, want) {
			t.Errorf("report lacks %q:\n%s", want, text)
		}
	}
}

const shapeSearchProblemYAML = `
scenarios:
  - {name: alone, player: collector, sets: [RAV, TSP, GPT], per_set: true}
shapes:
  - {scenario: alone, metric: "pace.{set}", center: 14, spread: 3}
`

func TestSearch_aShapeGoalCentersTheSpread(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, shapeSearchProblemYAML)

	report, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	best := report.Best()
	if got := report.BestConfig.CounterBuyPct; got < 135 || got > 145 {
		t.Errorf("best counter_buy_pct = %d, want about 140, which puts the middle set at 14 days", got)
	}
	if _, ok := best.Goals["alone shape pace.{set}"]; !ok {
		t.Errorf("best goals = %v, want one entry for the shape", best.Goals)
	}
	if best.Loss == nil || *best.Loss > 0.5 {
		t.Errorf("best loss = %v, want under 0.5: offsets of -3, 0 and 3 are close to a bell of spread 3", best.Loss)
	}
}

func TestSearch_aShapeOnAMetricTheRunsLackAbortsTheSearch(t *testing.T) {
	t.Parallel()
	base, space, problem := searchFixture(t, searchSpaceYAML, `
scenarios:
  - {name: alone, player: collector, sets: [RAV, TSP, GPT], per_set: true}
shapes:
  - {scenario: alone, metric: "missing.{set}", center: 14, spread: 3}
`)

	_, err := tune.Search(context.Background(), base, space, problem, &fakeRunner{}, options())

	if err == nil || !strings.Contains(err.Error(), "reports no metric") {
		t.Errorf("Search error = %v, want it to say a run reports no metric", err)
	}
}
