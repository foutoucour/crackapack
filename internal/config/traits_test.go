package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// testTraitNames returns a sorted list of trait names for trait config tests, independent of the
// live DefaultTraits registry (which is empty until personality stories add real traits).
func testTraitNames() []string {
	return []string{"ego", "gambler", "margin", "score", "stingy", "veto"}
}

func TestCheckTraits_acceptsValidConfig(t *testing.T) {
	t.Parallel()
	a := config.ArchetypeConfig{
		Traits:      []string{"ego", "stingy"},
		TraitParams: map[string]map[string]int{"ego": {"boldness": 5}, "stingy": {"floor": 10}},
	}
	if err := config.CheckTraitsForTest("npcs.archetypes.x", a, testTraitNames()); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestCheckTraits_rejectsDuplicateName(t *testing.T) {
	t.Parallel()
	a := config.ArchetypeConfig{
		Traits: []string{"ego", "ego"},
	}
	err := config.CheckTraitsForTest("npcs.archetypes.x", a, testTraitNames())
	if err == nil {
		t.Fatal("want error for duplicate trait name")
	}
	if !strings.Contains(err.Error(), "npcs.archetypes.x") || !strings.Contains(err.Error(), "ego") {
		t.Errorf("error %q does not name the key and the duplicate trait", err)
	}
}

func TestCheckTraits_rejectsTraitParamNotInTraits(t *testing.T) {
	t.Parallel()
	a := config.ArchetypeConfig{
		Traits:      []string{"ego"},
		TraitParams: map[string]map[string]int{"stingy": {"floor": 10}}, // "stingy" not in traits
	}
	err := config.CheckTraitsForTest("npcs.archetypes.x", a, testTraitNames())
	if err == nil {
		t.Fatal("want error for trait_params entry not in traits")
	}
	if !strings.Contains(err.Error(), "stingy") {
		t.Errorf("error %q does not name the offending param key", err)
	}
}

func TestCheckTraits_rejectsNegativeParam(t *testing.T) {
	t.Parallel()
	a := config.ArchetypeConfig{
		Traits:      []string{"ego"},
		TraitParams: map[string]map[string]int{"ego": {"boldness": -1}},
	}
	err := config.CheckTraitsForTest("npcs.archetypes.x", a, testTraitNames())
	if err == nil {
		t.Fatal("want error for negative param value")
	}
	if !strings.Contains(err.Error(), "boldness") {
		t.Errorf("error %q does not name the offending param", err)
	}
}

func TestCheckTraits_rejectsUnknownTraitName(t *testing.T) {
	t.Parallel()
	a := config.ArchetypeConfig{
		Traits: []string{"unknown_trait"},
	}
	err := config.CheckTraitsForTest("npcs.archetypes.x", a, testTraitNames())
	if err == nil {
		t.Fatal("want error for unknown trait name")
	}
	if !strings.Contains(err.Error(), "unknown_trait") {
		t.Errorf("error %q does not name the unknown trait", err)
	}
	if !strings.Contains(err.Error(), "known traits") {
		t.Errorf("error %q does not list known traits", err)
	}
}

func TestLoad_maxMarginPctDefault(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NPCs.MaxMarginPct != 1000 {
		t.Errorf("MaxMarginPct = %d, want 1000 (default)", cfg.NPCs.MaxMarginPct)
	}
}

func TestLoad_maxMarginPctAcceptedAtBound(t *testing.T) {
	t.Parallel()
	_, err := config.Load(strings.NewReader("npcs:\n  max_margin_pct: 10000\n"))
	if err != nil {
		t.Errorf("max_margin_pct at bound should be accepted: %v", err)
	}
}

func TestLoad_maxMarginPctRejectedAboveBound(t *testing.T) {
	t.Parallel()
	_, err := config.Load(strings.NewReader("npcs:\n  max_margin_pct: 10001\n"))
	if err == nil {
		t.Fatal("max_margin_pct above bound must be rejected")
	}
	if !strings.Contains(err.Error(), "npcs.max_margin_pct") {
		t.Errorf("error %q does not name the key", err)
	}
}

func TestLoad_maxMarginPctRejectedWhenNegative(t *testing.T) {
	t.Parallel()
	_, err := config.Load(strings.NewReader("npcs:\n  max_margin_pct: -1\n"))
	if err == nil {
		t.Fatal("negative max_margin_pct must be rejected")
	}
	if !strings.Contains(err.Error(), "npcs.max_margin_pct") {
		t.Errorf("error %q does not name the key", err)
	}
}

func TestLoad_traitParamAboveBoundIsRejected(t *testing.T) {
	t.Parallel()
	const yaml = `
npcs:
  archetypes:
    casual:
      share: 1
      min_boosters_per_day: 1
      max_boosters_per_day: 1
      keep_copies: 1
      trait_params:
        casual:
          boldness: 10001
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("trait param above bound must be rejected")
	}
	if !strings.Contains(err.Error(), "npcs.archetypes.casual.trait_params.casual.boldness") {
		t.Errorf("error %q does not name the key", err)
	}
}

func TestLoad_unknownTraitFromRegistryIsRejected(t *testing.T) {
	t.Parallel()
	// A name that no story will ever register, so the test does not depend on which traits exist.
	const yaml = `
npcs:
  archetypes:
    casual:
      share: 1
      min_boosters_per_day: 1
      max_boosters_per_day: 1
      keep_copies: 1
      traits: [not_a_real_trait]
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("unknown trait name must be rejected")
	}
	if !strings.Contains(err.Error(), "not_a_real_trait") {
		t.Errorf("error %q does not name the unknown trait", err)
	}
	if !strings.Contains(err.Error(), "known traits") {
		t.Errorf("error %q does not mention known traits", err)
	}
}
