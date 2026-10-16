package sim_test

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

// TestWhaleConfig_allFiveColorsEqualsNilScope verifies that a config-level crowd where
// the whale archetype has all five colours produces an engine that runs identically to one
// where the same archetype has no colours. Per ADR-0022, all-five == no restriction.
func TestWhaleConfig_allFiveColorsEqualsNilScope(t *testing.T) {
	t.Parallel()

	buildCfg := func(colors []string) config.Config {
		cfg := smallConfig()
		cfg.RunDays = 3
		cfg.InitialStock = map[domain.Rarity]int{}
		cfg.NPCs = config.NPCConfig{
			Population:      4,
			SurplusHoldDays: 2,
			Archetypes: map[string]config.ArchetypeConfig{
				"whale": {
					Share:             1,
					MinBoostersPerDay: 5,
					MaxBoostersPerDay: 5,
					KeepCopies:        1,
					Colors:            colors,
				},
			},
		}
		return cfg
	}

	cats := []domain.SetCatalogue{
		domaintest.NewRavnicaCatalogue(),
		domaintest.NewClassicCatalogue(),
	}
	prices := mergedPrices(cats, smallConfig().Seed)

	run := func(colors []string) sim.Result {
		t.Helper()
		cfg := buildCfg(colors)
		e, err := sim.NewEngine(cfg, cats, nil, prices, sim.NewIdlePolicy(), rand.New(rand.NewPCG(cfg.Seed, cfg.Seed)))
		if err != nil {
			t.Fatalf("NewEngine(colors=%v): %v", colors, err)
		}
		result, err := e.Run()
		if err != nil {
			t.Fatalf("Run(colors=%v): %v", colors, err)
		}
		return result
	}

	noColors := run(nil)
	allFive := run([]string{"W", "U", "B", "R", "G"})

	if !reflect.DeepEqual(noColors.Summary, allFive.Summary) {
		t.Errorf("no-colors and all-five summary differ:\n  nil    = %v\n  all5   = %v",
			noColors.Summary, allFive.Summary)
	}
}
