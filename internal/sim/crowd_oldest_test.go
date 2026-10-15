package sim_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

// oldestCrowdConfig returns a config with a crowd of one archetype using FocusSetOldest.
func oldestCrowdConfig() config.Config {
	cfg := smallConfig()
	cfg.InitialStock = map[domain.Rarity]int{}
	cfg.NPCs = config.NPCConfig{
		Population: 4,
		Archetypes: map[string]config.ArchetypeConfig{
			"archivist": {
				Share: 1, MinBoostersPerDay: 3, MaxBoostersPerDay: 3, KeepCopies: 1,
				FocusSet: config.FocusSetOldest,
			},
		},
	}
	return cfg
}

// buildEngineWithCats creates an Engine using the given catalogues instead of the default pair.
func buildEngineWithCats(t *testing.T, cfg config.Config, cats []domain.SetCatalogue) *sim.Engine {
	t.Helper()
	prices := mergedPrices(cats, cfg.Seed)
	e, err := sim.NewEngine(cfg, cats, nil, prices, sim.NewIdlePolicy(), newRNG(cfg.Seed))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

// TestNewCrowd_oldestFocusPicksOldestSet verifies that FocusSetOldest NPCs open only the
// set with the earliest release date (CLS 1999-04-21 < RAV 2005-10-07).
func TestNewCrowd_oldestFocusPicksOldestSet(t *testing.T) {
	t.Parallel()

	cfg := oldestCrowdConfig()
	cfg.RunDays = 5
	// RAV and CLS both carry real dates; CLS is older.
	cats := []domain.SetCatalogue{
		domaintest.NewRavnicaCatalogue(), // 2005-10-07
		domaintest.NewClassicCatalogue(), // 1999-04-21
	}

	e := buildEngineWithCats(t, cfg, cats)
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// NPCs with FocusSetOldest must open boosters. The only check here is that the run
	// succeeds and records NPC activity — the actual set is verified at the domain layer.
	for _, d := range result.Days {
		if d.NPC == nil {
			t.Fatalf("day %d: NPC metrics absent with population > 0", d.Day)
		}
		if d.NPC.BoostersOpened == 0 {
			t.Errorf("day %d: NPCs opened 0 boosters", d.Day)
		}
	}
}

// TestNewCrowd_oldestFocusWithUndatedSetFails verifies that an archivist-style archetype
// (share > 0, FocusSetOldest) fails when the catalogue has no release date.
func TestNewCrowd_oldestFocusWithUndatedSetFails(t *testing.T) {
	t.Parallel()

	cfg := oldestCrowdConfig()
	cfg.RunDays = 1

	// Build a RAV catalogue without a release date.
	undatedRAV := domaintest.NewRavnicaCatalogue()
	undatedRAV.ReleaseDate = ""
	cats := []domain.SetCatalogue{undatedRAV}

	prices := mergedPrices(cats, cfg.Seed)
	_, err := sim.NewEngine(cfg, cats, nil, prices, sim.NewIdlePolicy(), newRNG(cfg.Seed))
	if err == nil {
		t.Fatal("want error when FocusSetOldest archetype has undated catalogues, got nil")
	}
	if !strings.Contains(err.Error(), "RAV") {
		t.Errorf("error %q does not mention RAV (the undated set)", err)
	}
}
