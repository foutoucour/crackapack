package main

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/tune"
)

// tuneOptions is a search small enough for a test: a few candidates on a few seeds.
func tuneOptions(workers int) tune.Options {
	return tune.Options{Seed: 5, Candidates: 8, KeepFraction: 0.5, Seeds: 2, ValidationSeeds: 2, Top: 2, Workers: workers}
}

// demoTuneArgs wires the demo dataset into a search whose outputs land in the returned store.
func demoTuneArgs(t *testing.T, goals string, opts tune.Options) (tuneArgs, map[string]*bytes.Buffer, *strings.Builder) {
	t.Helper()
	ds := demo.Build()
	store := make(map[string]*bytes.Buffer)
	stdout := &strings.Builder{}
	if goals == "" {
		goals = ds.TuneGoalsYAML
	}
	return tuneArgs{
		configReader:  strings.NewReader(ds.TuneConfigYAML),
		spaceReader:   strings.NewReader(ds.TuneSpaceYAML),
		problemReader: strings.NewReader(goals),
		catFS:         ds.Files,
		pricesFS:      ds.Files,
		pricesName:    "prices.json",
		policies:      buildPolicies(),
		opts:          opts,
		newOutput: func(name string) (io.WriteCloser, error) {
			buf := &bytes.Buffer{}
			store[name] = buf
			return nopWriteCloser{buf}, nil
		},
		stdout: stdout,
	}, store, stdout
}

func TestRunTune_writesTheReportTheCandidatesAndABestConfigThatLoads(t *testing.T) {
	t.Parallel()
	a, store, stdout := demoTuneArgs(t, "", tuneOptions(2))

	if err := runTune(context.Background(), a); err != nil {
		t.Fatalf("runTune: %v", err)
	}

	for _, name := range []string{"candidates.json", "best.yaml", "report.txt"} {
		if store[name] == nil || store[name].Len() == 0 {
			t.Errorf("%s: missing or empty", name)
		}
	}
	if store["report.txt"].String() != stdout.String() {
		t.Error("report.txt differs from what was printed")
	}
	if _, err := config.Load(bytes.NewReader(store["best.yaml"].Bytes())); err != nil {
		t.Errorf("best.yaml does not load as a config: %v", err)
	}
	if !strings.Contains(stdout.String(), "Best candidates") {
		t.Errorf("stdout lacks the report:\n%s", stdout.String())
	}
}

func TestRunTune_givesTheSameReportWhateverTheNumberOfWorkers(t *testing.T) {
	t.Parallel()

	candidates := func(workers int) string {
		a, store, _ := demoTuneArgs(t, "", tuneOptions(workers))
		if err := runTune(context.Background(), a); err != nil {
			t.Fatalf("runTune with %d workers: %v", workers, err)
		}
		return store["candidates.json"].String()
	}
	if one, three := candidates(1), candidates(3); one != three {
		t.Error("candidates.json differs between 1 and 3 workers")
	}
}

func TestRunTune_rejectsWhatTheCatalogueDirectoryCannotServe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goals string
		want  string
	}{
		{"unknown player", "scenarios:\n  - {name: s, player: nobody, sets: [DEM]}\ngoals:\n  - {scenario: s, metric: final_balance, comparison: at_most, max: 1}\n", "unknown player"},
		{"unknown set", "scenarios:\n  - {name: s, player: collector, sets: [ZZZ]}\ngoals:\n  - {scenario: s, metric: final_balance, comparison: at_most, max: 1}\n", "ZZZ"},
		{"unknown pool", "scenarios:\n  - {name: s, player: collector, sets: [DEM], supplementary: [ZZZ]}\ngoals:\n  - {scenario: s, metric: final_balance, comparison: at_most, max: 1}\n", "ZZZ"},
		{"unknown metric", "scenarios:\n  - {name: s, player: collector, sets: [DEM]}\ngoals:\n  - {scenario: s, metric: no_such_metric, comparison: at_most, max: 1}\n", "no_such_metric"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, _, _ := demoTuneArgs(t, tc.goals, tuneOptions(1))
			err := runTune(context.Background(), a)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("runTune error = %v, want one naming %q", err, tc.want)
			}
		})
	}
}

func TestCmdTune_requiresItsFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := run([]string{"tune"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error for missing flags, got nil")
	}
	for _, flag := range []string{"--catalogue-dir", "--goals", "--out", "--prices", "--space"} {
		if !strings.Contains(err.Error(), flag) {
			t.Errorf("error %q does not name %s", err, flag)
		}
	}
}

func TestCmdTune_runsOnFilesAndWritesTheOutputs(t *testing.T) {
	t.Parallel()
	ds := demo.Build()
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}
	catDir := filepath.Join(dir, "catalogues")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"DEM.json", "DSP.json", "DST.json"} {
		data, err := fs.ReadFile(ds.Files, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(catDir, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	prices, err := fs.ReadFile(ds.Files, "prices.json")
	if err != nil {
		t.Fatalf("read prices: %v", err)
	}
	out := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer
	err = run([]string{
		"tune",
		"--config", write("base.yaml", ds.TuneConfigYAML),
		"--space", write("space.yaml", ds.TuneSpaceYAML),
		"--goals", write("goals.yaml", ds.TuneGoalsYAML),
		"--catalogue-dir", catDir,
		"--prices", write("prices.json", string(prices)),
		"--out", out,
		"--candidates", "6", "--keep", "0.5", "--seeds", "1", "--validation-seeds", "1", "--top", "1", "--workers", "2",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run tune: %v\nstderr: %s", err, stderr.String())
	}

	for _, name := range []string{"candidates.json", "best.yaml", "report.txt"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
	if !strings.Contains(stderr.String(), "tune: scoring") {
		t.Errorf("no progress on stderr:\n%s", stderr.String())
	}
}
