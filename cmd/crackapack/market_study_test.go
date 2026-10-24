package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/study"
	"github.com/foutoucour/crackapack/internal/tune"
)

const demoVariants = `
variants:
  - name: baseline
    note: the demo world
  - name: bigger-crowd
    note: twice the NPCs
    overrides:
      - {path: npcs.population, value: 40}
`

// demoStudyArgs wires the demo dataset into a study whose outputs land in the returned store.
func demoStudyArgs(t *testing.T, workers int) (studyArgs, map[string]*bytes.Buffer, *strings.Builder) {
	t.Helper()
	ds := demo.Build()
	base, err := config.Load(strings.NewReader(ds.TuneConfigYAML))
	if err != nil {
		t.Fatalf("load demo config: %v", err)
	}
	store := make(map[string]*bytes.Buffer)
	stdout := &strings.Builder{}
	return studyArgs{
		baseConfig:     base,
		problemReader:  strings.NewReader(ds.TuneGoalsYAML),
		scenario:       "pace-3-visits",
		variantsReader: strings.NewReader(demoVariants),
		catFS:          ds.Files,
		pricesFS:       ds.Files,
		pricesName:     "prices.json",
		policies:       buildPolicies(),
		runDays:        20,
		seeds:          []uint64{7},
		workers:        workers,
		targets: study.Targets{
			CounterSpeed: study.Band{Min: 1.2, Max: 1.5}, MarketSpeedMin: 2, MarketOverCounterMin: 1.5, FloorDays: 1,
		},
		newOutput: func(name string) (io.WriteCloser, error) {
			buf := &bytes.Buffer{}
			store[name] = buf
			return nopWriteCloser{buf}, nil
		},
		stdout: stdout,
	}, store, stdout
}

func TestRunStudy_runsEveryVariantWithEveryPlayerOnEverySet(t *testing.T) {
	t.Parallel()
	a, store, stdout := demoStudyArgs(t, 2)

	if err := runStudy(context.Background(), a); err != nil {
		t.Fatalf("runStudy: %v", err)
	}

	var got struct {
		Outcomes []study.Outcome `json:"outcomes"`
	}
	if store["study.json"] == nil {
		t.Fatal("study.json not written")
	}
	if err := json.Unmarshal(store["study.json"].Bytes(), &got); err != nil {
		t.Fatalf("study.json: %v", err)
	}
	if len(got.Outcomes) != 2 {
		t.Fatalf("study.json has %d variants, want 2", len(got.Outcomes))
	}
	for _, o := range got.Outcomes {
		for _, player := range []string{study.BoostersOnly, study.Counter, study.Market} {
			if len(o.Runs[player]) != 2 {
				t.Errorf("variant %s, player %s: %d runs, want one per demo set", o.Name, player, len(o.Runs[player]))
			}
		}
	}
	if store["report.html"] == nil || !strings.Contains(store["report.html"].String(), "bigger-crowd") {
		t.Error("report.html is missing or does not name the variants")
	}
	if !strings.Contains(stdout.String(), "baseline") {
		t.Errorf("stdout lacks the summary of the variants:\n%s", stdout.String())
	}
}

func TestRunStudy_poolsTheSetsOfEverySeed(t *testing.T) {
	t.Parallel()
	a, store, _ := demoStudyArgs(t, 2)
	a.seeds = []uint64{7, 8}

	if err := runStudy(context.Background(), a); err != nil {
		t.Fatalf("runStudy: %v", err)
	}

	var got struct {
		Outcomes []study.Outcome `json:"outcomes"`
	}
	if err := json.Unmarshal(store["study.json"].Bytes(), &got); err != nil {
		t.Fatalf("study.json: %v", err)
	}
	runs := got.Outcomes[0].Runs[study.Counter]
	if len(runs) != 4 {
		t.Fatalf("%d runs for the Counter, want one per set and seed, 4", len(runs))
	}
	seen := map[uint64]int{}
	for _, r := range runs {
		seen[r.Seed]++
	}
	if seen[7] != 2 || seen[8] != 2 {
		t.Errorf("runs per seed = %v, want 2 of each", seen)
	}
	if !strings.Contains(store["report.html"].String(), "seeds 7, 8") {
		t.Error("report.html does not say which seeds it pools")
	}
}

func TestRenderStudy_rebuildsTheReportOfAFinishedStudyWithoutSimulating(t *testing.T) {
	t.Parallel()
	a, store, _ := demoStudyArgs(t, 2)
	if err := runStudy(context.Background(), a); err != nil {
		t.Fatalf("runStudy: %v", err)
	}
	rendered := make(map[string]*bytes.Buffer)
	stdout := &strings.Builder{}

	err := renderStudy(bytes.NewReader(store["study.json"].Bytes()), a.targets,
		func(name string) (io.WriteCloser, error) {
			buf := &bytes.Buffer{}
			rendered[name] = buf
			return nopWriteCloser{buf}, nil
		}, stdout)

	if err != nil {
		t.Fatalf("renderStudy: %v", err)
	}
	if rendered["report.html"].String() != store["report.html"].String() {
		t.Error("the report rebuilt from study.json differs from the one the study wrote")
	}
	if rendered["study.json"].String() != store["study.json"].String() {
		t.Error("the study.json rebuilt differs from the one the study wrote")
	}
	if !strings.Contains(stdout.String(), "baseline") {
		t.Errorf("stdout lacks the variants:\n%s", stdout.String())
	}
}

func TestCmdMarketStudy_fromRewritesTheReportInPlaceWithoutOtherFlags(t *testing.T) {
	t.Parallel()
	a, store, _ := demoStudyArgs(t, 2)
	if err := runStudy(context.Background(), a); err != nil {
		t.Fatalf("runStudy: %v", err)
	}
	dir := t.TempDir()
	studyPath := filepath.Join(dir, "study.json")
	if err := os.WriteFile(studyPath, store["study.json"].Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"market-study", "--from", studyPath, "--out", dir}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	html, err := os.ReadFile(filepath.Join(dir, "report.html"))
	if err != nil {
		t.Fatalf("report.html not written: %v", err)
	}
	if !strings.Contains(string(html), "bigger-crowd") {
		t.Error("report.html does not name the variants of the study")
	}
	// The command judges against the product's targets, not the test's, so the verdicts may differ
	// from the study's. The runs must come back as they were.
	outcomesOf := func(data []byte) []study.Outcome {
		var f studyFile
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("study.json: %v", err)
		}
		return f.Outcomes
	}
	rewritten, err := os.ReadFile(studyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(outcomesOf(rewritten), outcomesOf(store["study.json"].Bytes())) {
		t.Error("the runs of study.json changed when its report was rewritten")
	}
}

func TestRenderStudy_rejectsAFileThatIsNotAStudy(t *testing.T) {
	t.Parallel()

	err := renderStudy(strings.NewReader(`{"outcomes": []}`), studyTargets(), nil, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "no variant") {
		t.Errorf("renderStudy = %v, want an error saying the file holds no variant", err)
	}
}

func TestRunStudy_marketPlayerReportsItsDeals(t *testing.T) {
	t.Parallel()
	a, store, _ := demoStudyArgs(t, 2)

	if err := runStudy(context.Background(), a); err != nil {
		t.Fatalf("runStudy: %v", err)
	}

	var got struct {
		Outcomes []study.Outcome `json:"outcomes"`
	}
	if err := json.Unmarshal(store["study.json"].Bytes(), &got); err != nil {
		t.Fatalf("study.json: %v", err)
	}
	var listings float64
	for _, r := range got.Outcomes[0].Runs[study.Market] {
		listings += r.Listings
	}
	if listings == 0 {
		t.Error("the market player never listed a card in the demo, want it to trade")
	}
}

func TestRunStudy_givesTheSameOutcomesWhateverTheNumberOfWorkers(t *testing.T) {
	t.Parallel()
	outcomes := func(workers int) string {
		a, store, _ := demoStudyArgs(t, workers)
		if err := runStudy(context.Background(), a); err != nil {
			t.Fatalf("runStudy with %d workers: %v", workers, err)
		}
		return store["study.json"].String()
	}

	if one, three := outcomes(1), outcomes(3); one != three {
		t.Error("study.json differs between 1 and 3 workers")
	}
}

func TestRunStudy_stopsWhenItsContextEnds(t *testing.T) {
	t.Parallel()
	a, store, _ := demoStudyArgs(t, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runStudy(ctx, a)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("runStudy = %v, want context.Canceled", err)
	}
	if len(store) != 0 {
		t.Errorf("wrote %d outputs from a run that was stopped, want none", len(store))
	}
}

func TestNewStudyRunner_putsEveryPlayerInACrowdThatKeepsItsSurplus(t *testing.T) {
	t.Parallel()
	a, _, _ := demoStudyArgs(t, 1)
	problem, err := tune.LoadProblem(a.problemReader)
	if err != nil {
		t.Fatalf("LoadProblem: %v", err)
	}
	runs, err := scenarioRuns(problem, a.scenario)
	if err != nil {
		t.Fatalf("scenarioRuns: %v", err)
	}
	args := tuneArgs{catFS: a.catFS, pricesFS: a.pricesFS, pricesName: a.pricesName, policies: a.policies}
	cfg := a.baseConfig
	cfg.RunDays = 3
	run := runs[0]
	run.Player = study.Counter

	plain, err := newSimRunner(args, runs)
	if err != nil {
		t.Fatalf("newSimRunner: %v", err)
	}
	shelved, err := newStudyRunner(args, runs)
	if err != nil {
		t.Fatalf("newStudyRunner: %v", err)
	}
	sold := func(r simRunner) float64 {
		summary, err := r.Run(cfg, run)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return summary["npc_cards_sold"]
	}

	if plainSold, shelvedSold := sold(plain), sold(shelved); shelvedSold >= plainSold {
		t.Errorf("the crowd sold %v cards to the Counter with a shelf, and %v without: want fewer while it holds them", shelvedSold, plainSold)
	}
}

func TestStopOnFile_endsTheContextWhenTheFileAppearsOrTheDeadlinePasses(t *testing.T) {
	t.Parallel()
	const poll, wait = 5 * time.Millisecond, 5 * time.Second
	ended := func(ctx context.Context) bool {
		select {
		case <-ctx.Done():
			return true
		case <-time.After(wait):
			return false
		}
	}

	t.Run("file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "STOP")
		ctx, cancel := stopOnFile(context.Background(), path, poll, 0)
		defer cancel()
		if ctx.Err() != nil {
			t.Fatal("the context ended before the file appeared")
		}

		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}

		if !ended(ctx) {
			t.Error("the context did not end after the STOP file appeared")
		}
	})
	t.Run("deadline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := stopOnFile(context.Background(), filepath.Join(t.TempDir(), "STOP"), poll, 20*time.Millisecond)
		defer cancel()

		if !ended(ctx) {
			t.Error("the context did not end at the deadline")
		}
	})
}

func TestRunStudy_rejectsAnUnknownScenarioAndAnUnknownOverride(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*studyArgs)
		want string
	}{
		{"scenario", func(a *studyArgs) { a.scenario = "pace-9-visits" }, "pace-9-visits"},
		{"override", func(a *studyArgs) {
			a.variantsReader = strings.NewReader("variants:\n  - name: bad\n    overrides:\n      - {path: no.such.key, value: 1}\n")
		}, "no.such.key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, _, _ := demoStudyArgs(t, 1)
			tt.edit(&a)

			err := runStudy(context.Background(), a)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("runStudy = %v, want an error naming %q", err, tt.want)
			}
		})
	}
}
