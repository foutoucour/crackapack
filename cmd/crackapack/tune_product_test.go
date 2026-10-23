package main

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/tune"
)

// productTuneDir is the directory of the product's search files, relative to this package.
const productTuneDir = "../../tune"

// productDraws is how many candidates the validity test draws: enough to meet the edges of every
// range, few enough to take a moment.
const productDraws = 2000

func readProductFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(productTuneDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// The files in tune/ are the input of the product's search. A draw the config refuses scores the
// worst loss and wastes a simulation slot, so the space must only draw valid configs, among them a
// cost table that rises with the tier, see ADR-0048.
func TestProductTuneFiles_everyDrawOfTheSpaceIsAValidConfig(t *testing.T) {
	t.Parallel()
	base, err := config.Load(strings.NewReader(readProductFile(t, "base.yaml")))
	if err != nil {
		t.Fatalf("base.yaml: %v", err)
	}
	space, err := tune.LoadSpace(strings.NewReader(readProductFile(t, "space.yaml")))
	if err != nil {
		t.Fatalf("space.yaml: %v", err)
	}
	rng := rand.New(rand.NewPCG(1, 1))

	for range productDraws {
		overrides := space.Sample(rng)
		if _, err := config.ApplyOverrides(base, overrides); err != nil {
			t.Fatalf("a draw of the product space is refused: %v\noverrides: %+v", err, overrides)
		}
	}
}

func TestProductTuneFiles_goalsLoadAndCoverTheScope(t *testing.T) {
	t.Parallel()
	problem, err := tune.LoadProblem(strings.NewReader(readProductFile(t, "goals.yaml")))
	if err != nil {
		t.Fatalf("goals.yaml: %v", err)
	}

	const scopeSize = 107 // ADR-0039
	for _, s := range problem.Scenarios {
		if len(s.Sets) != scopeSize {
			t.Errorf("scenario %s has %d sets, want the %d of the scope", s.Name, len(s.Sets), scopeSize)
		}
	}
}

// A days_to_ metric that is not reached reads as one day past the run, so a ceiling above the run
// length lets a set that never finishes pass, see ADR-0050.
func TestProductTuneFiles_aPaceCeilingNeverExceedsItsRunLength(t *testing.T) {
	t.Parallel()
	problem, err := tune.LoadProblem(strings.NewReader(readProductFile(t, "goals.yaml")))
	if err != nil {
		t.Fatalf("goals.yaml: %v", err)
	}
	runDays := make(map[string]int, len(problem.Scenarios))
	for _, s := range problem.Scenarios {
		runDays[s.Name] = s.RunDays
	}

	for _, g := range problem.Goals {
		if !strings.HasPrefix(g.Metric, "days_to_") || g.Max == nil {
			continue
		}
		if run := runDays[g.Scenario]; run == 0 || *g.Max > float64(run) {
			t.Errorf("goal %s on %s: ceiling %v is not within the scenario's run_days %d", g.Metric, g.Scenario, *g.Max, run)
		}
	}
}
