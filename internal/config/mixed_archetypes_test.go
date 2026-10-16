package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// TestEgoStingyArchetype_loadsFromEmbeddedYAML verifies that the embedded ego_stingy.yaml
// loads with the expected fields: colors [U, B], traits [ego, stingy], share=3, behaviour=casual,
// and both trait parameter sets present.
func TestEgoStingyArchetype_loadsFromEmbeddedYAML(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	a, ok := cfg.NPCs.Archetypes["ego_stingy"]
	if !ok {
		t.Fatal("archetype 'ego_stingy' not found in embedded defaults")
	}

	if a.Share != 3 {
		t.Errorf("Share = %d, want 3 (assumption to calibrate, set in T-009)", a.Share)
	}
	if len(a.Colors) != 2 || a.Colors[0] != "U" || a.Colors[1] != "B" {
		t.Errorf("Colors = %v, want [U B]", a.Colors)
	}
	if len(a.Traits) != 2 || a.Traits[0] != "ego" || a.Traits[1] != "stingy" {
		t.Errorf("Traits = %v, want [ego stingy]", a.Traits)
	}
	if a.Behaviour != "casual" {
		t.Errorf("Behaviour = %q, want \"casual\"", a.Behaviour)
	}
	if a.MarginPct <= 0 {
		t.Errorf("MarginPct = %d, want > 0 (ego_stingy has a base margin)", a.MarginPct)
	}

	egoParams, ok := a.TraitParams["ego"]
	if !ok {
		t.Fatal("TraitParams[\"ego\"] not set; ego_stingy must supply ego params")
	}
	if _, ok := egoParams["max_retries"]; !ok {
		t.Error("ego params missing max_retries")
	}
	if _, ok := egoParams["endowment_pct_per_day"]; !ok {
		t.Error("ego params missing endowment_pct_per_day")
	}

	stingyParams, ok := a.TraitParams["stingy"]
	if !ok {
		t.Fatal("TraitParams[\"stingy\"] not set; ego_stingy must supply stingy params")
	}
	for _, key := range []string{"common_pct", "uncommon_pct", "rare_pct", "special_pct", "mythic_pct"} {
		if _, ok := stingyParams[key]; !ok {
			t.Errorf("stingy params missing %s", key)
		}
	}
}

// TestArchivistEgoStingyArchetype_loadsFromEmbeddedYAML verifies that the embedded
// archivist_ego_stingy.yaml loads with the expected fields: colors [W, U, B],
// traits [archivist, ego, stingy], focus_set=false (no focus_set so archivist matters),
// share=2, behaviour=casual.
func TestArchivistEgoStingyArchetype_loadsFromEmbeddedYAML(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	a, ok := cfg.NPCs.Archetypes["archivist_ego_stingy"]
	if !ok {
		t.Fatal("archetype 'archivist_ego_stingy' not found in embedded defaults")
	}

	if a.Share != 2 {
		t.Errorf("Share = %d, want 2 (assumption to calibrate, set in T-009)", a.Share)
	}
	if len(a.Colors) != 3 || a.Colors[0] != "W" || a.Colors[1] != "U" || a.Colors[2] != "B" {
		t.Errorf("Colors = %v, want [W U B]", a.Colors)
	}
	if len(a.Traits) != 3 {
		t.Fatalf("Traits = %v, want [archivist ego stingy]", a.Traits)
	}
	if a.Traits[0] != "archivist" || a.Traits[1] != "ego" || a.Traits[2] != "stingy" {
		t.Errorf("Traits = %v, want [archivist ego stingy]", a.Traits)
	}
	if a.Behaviour != "casual" {
		t.Errorf("Behaviour = %q, want \"casual\"", a.Behaviour)
	}
	// focus_set must NOT be oldest: with oldest, every card in scope shares one release date
	// so the archivist wish-scorer would break ties trivially instead of ranking older sets.
	if a.FocusSet == config.FocusSetOldest {
		t.Error("FocusSet must not be oldest; archivist_ego_stingy uses no focus_set so the archivist trait ranks wishes across all sets")
	}

	// Archivist takes no params; only ego and stingy params should be present.
	if _, ok := a.TraitParams["archivist"]; ok {
		t.Error("TraitParams[\"archivist\"] present; archivist takes no parameters")
	}
	if _, ok := a.TraitParams["ego"]; !ok {
		t.Error("TraitParams[\"ego\"] not set")
	}
	if _, ok := a.TraitParams["stingy"]; !ok {
		t.Error("TraitParams[\"stingy\"] not set")
	}
}

// TestArchivistEgoStingyArchetype_validatesWithTradesOn verifies that the new three-colour
// archetype passes the full validation path with trades enabled.
func TestArchivistEgoStingyArchetype_validatesWithTradesOn(t *testing.T) {
	t.Parallel()
	const yaml = `
npcs:
  population: 10
trades:
  enabled: true
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load with trades enabled: %v — archivist_ego_stingy must pass all validations", err)
	}
}

// TestEgoStingyArchetype_validatesWithTradesOn verifies that ego_stingy passes validation
// with trades enabled.
func TestEgoStingyArchetype_validatesWithTradesOn(t *testing.T) {
	t.Parallel()
	const yaml = `
npcs:
  population: 10
trades:
  enabled: true
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load with trades enabled: %v — ego_stingy must pass all validations", err)
	}
}

// TestMixedArchetypes_marginCapIsNotExceeded proves, for each shipped archetype with traits,
// that its base margin_pct plus the largest possible add of each of its traits stays below the
// default npcs.max_margin_pct (1000). The largest adds are:
//
//	stingy:  mythic_pct (the highest rarity add)
//	ego:     endowment_pct_per_day × npcs.surplus_hold_days (the oldest a card can sit on the shelf)
//	gambler: margin_extra_pct (the starting extra that tilt raises from)
//
// This proves that no archetype can blow through the margin cap by design.
func TestMixedArchetypes_marginCapIsNotExceeded(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	maxMarginPct := cfg.NPCs.MaxMarginPct

	// Largest possible add per trait, using the archetype's own trait_params.
	largestAdd := func(name string, a config.ArchetypeConfig) int {
		var total int
		for _, trait := range a.Traits {
			switch trait {
			case "stingy":
				total += a.TraitParams["stingy"]["mythic_pct"]
			case "ego":
				total += a.TraitParams["ego"]["endowment_pct_per_day"] * cfg.NPCs.SurplusHoldDays
			case "gambler":
				total += a.TraitParams["gambler"]["margin_extra_pct"]
			case "fish", "archivist":
				// fish and archivist add nothing to the margin.
			}
		}
		return total
	}

	for name, a := range cfg.NPCs.Archetypes {
		if len(a.Traits) == 0 {
			continue // no trait adds; base margin is already below the cap by construction
		}
		worstCase := int(a.MarginPct) + largestAdd(name, a)
		if worstCase >= int(maxMarginPct) {
			t.Errorf("archetype %q: worst-case margin %d >= max_margin_pct %d; must stay below the cap",
				name, worstCase, maxMarginPct)
		}
	}
}
