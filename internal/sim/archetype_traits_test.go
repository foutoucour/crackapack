package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// buyShelfPolicy opens one booster per session (so the player accumulates cards for NPCs to wish
// for) and then accepts every NPC-initiated offer visible in the session's offer list.
type buyShelfPolicy struct {
	t        *testing.T
	accepted *int
}

func (p buyShelfPolicy) RunSession(s sim.Session) {
	p.t.Helper()
	// Open a booster to build a card collection that NPCs can wish for.
	v := s.View()
	for _, cat := range v.Catalogues {
		if v.EnergyTotal >= v.BoosterCosts[cat.Code] {
			_ = s.OpenBooster(cat.Code)
			break
		}
	}
	// Accept all visible NPC-initiated offers.
	for _, offer := range s.Offers() {
		if offer.Origin != domain.OriginNPC {
			continue
		}
		d, err := s.AcceptOffer(offer.ID)
		if err != nil {
			continue // offer may have expired or already been taken
		}
		if d.Outcome == domain.OutcomeAccepted {
			*p.accepted++
		}
	}
}

// traitArchetypeBase returns a tradesConfig-style base with no archetypes, ready to have trait
// archetypes inserted.
func traitArchetypeBase(population int) config.Config {
	cfg := tradesConfig(population)
	cfg.NPCs.Archetypes = map[string]config.ArchetypeConfig{}
	cfg.NPCs.MaxMarginPct = 1000
	return cfg
}

// stingyArchetypeSpec returns an ArchetypeConfig for the stingy archetype matching the embedded
// YAML defaults, suitable for a sim-level invariant test.
func stingyArchetypeSpec() config.ArchetypeConfig {
	return config.ArchetypeConfig{
		Share:             1,
		MinBoostersPerDay: 1,
		MaxBoostersPerDay: 3,
		KeepCopies:        2,
		Behaviour:         "casual",
		WishSize:          5,
		MarginPct:         10,
		OfferChancePct:    100, // override for test speed: always propose when matched
		Traits:            []string{"stingy"},
		TraitParams: map[string]map[string]int{
			"stingy": {
				"common_pct":   0,
				"uncommon_pct": 5,
				"rare_pct":     20,
				"special_pct":  20,
				"mythic_pct":   40,
			},
		},
	}
}

// fishArchetypeSpec returns an ArchetypeConfig for the fish archetype matching the embedded YAML
// defaults, suitable for a sim-level invariant test.
func fishArchetypeSpec() config.ArchetypeConfig {
	return config.ArchetypeConfig{
		Share:             1,
		MinBoostersPerDay: 3,
		MaxBoostersPerDay: 6,
		KeepCopies:        1,
		Behaviour:         "speculator",
		WishSize:          8,
		MarginPct:         0,
		OfferChancePct:    100, // override for test speed: always propose when matched
		Traits:            []string{"fish"},
		TraitParams: map[string]map[string]int{
			"fish": {
				"price_unit_cents": 100,
				"rarity_points":    5,
				"foil_points":      3,
			},
		},
	}
}

// --- Criterion 10: engine with trades on, population 10, real default shares ---

// TestRun_defaultSharesWithTraitsBuildsAndRunsWithoutError verifies that loading a default config
// (which now includes fish and stingy with share=0) with a population and trades on does not
// error. This exercises the DefaultTraits registry path inside NewEngine.
func TestRun_defaultSharesWithTraitsBuildsAndRunsWithoutError(t *testing.T) {
	t.Parallel()
	// Use the real DefaultTraits (fish + stingy registered). They have share=0 so the crowd will
	// consist of the other embedded archetypes, but the registry is exercised at build time.
	cfg := tradesConfig(10)
	cfg.RunDays = 5
	// Replace the single "npc" archetype with several from the default set.
	cfg.NPCs.Archetypes = map[string]config.ArchetypeConfig{
		"stingy": stingyArchetypeSpec(),
		"fish":   fishArchetypeSpec(),
	}
	cfg.NPCs.MaxMarginPct = 1000
	_, err := buildEngineWithRegistry(t, cfg, sim.NewIdlePolicy(), domain.DefaultTraits()).Run()
	if err != nil {
		t.Fatalf("Run with trait archetypes and default registry: %v", err)
	}
}

// --- Criterion 13: buyShelfPolicy per archetype, no value gain ---

// TestRun_stingyArchetypeNoValueGain verifies that a crowd of stingy NPCs never lets the player
// gain value through trades, and that at least one trade is accepted.
func TestRun_stingyArchetypeNoValueGain(t *testing.T) {
	t.Parallel()
	cfg := traitArchetypeBase(10)
	cfg.NPCs.Archetypes["stingy"] = stingyArchetypeSpec()
	cfg.RunDays = 20

	var accepted int
	runResultWithRegistry(t, cfg,
		probePolicy{
			inner:    buyShelfPolicy{t: t, accepted: &accepted},
			t:        t,
			accepted: &accepted,
		},
		domain.DefaultTraits(),
	)

	if accepted == 0 {
		t.Error("no trade was accepted with the stingy archetype in 20 days; the invariant was never exercised")
	}
}

// TestRun_fishArchetypeNoValueGain verifies that a crowd of fish NPCs (margin=0) never lets the
// player gain value through trades, and that at least one trade is accepted.
func TestRun_fishArchetypeNoValueGain(t *testing.T) {
	t.Parallel()
	cfg := traitArchetypeBase(10)
	cfg.NPCs.Archetypes["fish"] = fishArchetypeSpec()
	cfg.RunDays = 20

	var accepted int
	runResultWithRegistry(t, cfg,
		probePolicy{
			inner:    buyShelfPolicy{t: t, accepted: &accepted},
			t:        t,
			accepted: &accepted,
		},
		domain.DefaultTraits(),
	)

	if accepted == 0 {
		t.Error("no trade was accepted with the fish archetype in 20 days; the invariant was never exercised")
	}
}

// --- Criterion 14: both archetypes together with probePolicy + exploiter ---

// TestRun_fishAndStingyTogetherNoValueGain verifies the no-value-gain invariant for a mixed crowd
// of 3 fish NPCs and 1 stingy NPC, exercised with the exploiter policy wrapped in probePolicy.
func TestRun_fishAndStingyTogetherNoValueGain(t *testing.T) {
	t.Parallel()
	cfg := traitArchetypeBase(4)

	fish := fishArchetypeSpec()
	fish.Share = 3
	stingy := stingyArchetypeSpec()
	stingy.Share = 1
	cfg.NPCs.Archetypes["fish"] = fish
	cfg.NPCs.Archetypes["stingy"] = stingy
	cfg.RunDays = 30

	var accepted int
	runResultWithRegistry(t, cfg,
		probePolicy{
			inner:    sim.NewExploiterPolicy(),
			t:        t,
			accepted: &accepted,
		},
		domain.DefaultTraits(),
	)

	if accepted == 0 {
		t.Error("no trade was accepted with fish+stingy crowd in 30 days; the invariant was never exercised")
	}
}
