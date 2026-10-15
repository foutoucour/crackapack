package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// --- Criterion 5: stingy archetype file ---

func TestLoad_stingyArchetypeHasExpectedValues(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, ok := cfg.NPCs.Archetypes["stingy"]
	if !ok {
		t.Fatal("archetype 'stingy' not found after Load; embedded YAML must be present")
	}
	if a.Share != 4 {
		t.Errorf("stingy.Share = %d, want 4 (set to 4 in integration story T-009, assumption to calibrate)", a.Share)
	}
	if len(a.Colors) != 1 || a.Colors[0] != "B" {
		t.Errorf("stingy.Colors = %v, want [B] (black)", a.Colors)
	}
	if len(a.Traits) != 1 || a.Traits[0] != "stingy" {
		t.Errorf("stingy.Traits = %v, want [stingy]", a.Traits)
	}
	if a.OfferChancePct > 1 {
		t.Errorf("stingy.OfferChancePct = %d, want ≤ 1 (passive NPC)", a.OfferChancePct)
	}
	// Margin additions by rarity must be strictly increasing for the interesting rarities.
	params, ok := a.TraitParams["stingy"]
	if !ok {
		t.Fatal("stingy.TraitParams[\"stingy\"] not set; margin table must be present")
	}
	uncommon := params["uncommon_pct"]
	rare := params["rare_pct"]
	mythic := params["mythic_pct"]
	if uncommon >= rare {
		t.Errorf("stingy trait: uncommon_pct=%d must be < rare_pct=%d", uncommon, rare)
	}
	if rare >= mythic {
		t.Errorf("stingy trait: rare_pct=%d must be < mythic_pct=%d", rare, mythic)
	}
}

// --- Criterion 12: fish archetype file ---

func TestLoad_fishArchetypeHasExpectedValues(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, ok := cfg.NPCs.Archetypes["fish"]
	if !ok {
		t.Fatal("archetype 'fish' not found after Load; embedded YAML must be present")
	}
	if a.Share != 4 {
		t.Errorf("fish.Share = %d, want 4 (set to 4 in integration story T-009, assumption to calibrate)", a.Share)
	}
	if len(a.Colors) != 1 || a.Colors[0] != "G" {
		t.Errorf("fish.Colors = %v, want [G] (green)", a.Colors)
	}
	if a.MarginPct != 0 {
		t.Errorf("fish.MarginPct = %d, want 0 (friendly NPC)", a.MarginPct)
	}
	if a.OfferChancePct < 10 {
		t.Errorf("fish.OfferChancePct = %d, want ≥ 10 (proposes often)", a.OfferChancePct)
	}
	if a.WishSize <= 5 {
		t.Errorf("fish.WishSize = %d, want > 5 (broad collector)", a.WishSize)
	}
	if a.KeepCopies != 1 {
		t.Errorf("fish.KeepCopies = %d, want 1 (trades duplicates freely)", a.KeepCopies)
	}
	if len(a.Traits) != 1 || a.Traits[0] != "fish" {
		t.Errorf("fish.Traits = %v, want [fish]", a.Traits)
	}
}

// --- Criterion 15: unknown trait error lists both stingy and fish as known ---

func TestLoad_unknownTraitErrorMentionsFishAndStingy(t *testing.T) {
	t.Parallel()
	const yaml = `
npcs:
  archetypes:
    casual:
      share: 1
      min_boosters_per_day: 1
      max_boosters_per_day: 1
      keep_copies: 1
      traits: [nope]
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("want error for unknown trait, got nil")
	}
	// The error must mention "nope" as the unknown trait, and both "fish" and "stingy" as known.
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the unknown trait 'nope'", err)
	}
	if !strings.Contains(err.Error(), "fish") {
		t.Errorf("error %q does not mention 'fish' as a known trait", err)
	}
	if !strings.Contains(err.Error(), "stingy") {
		t.Errorf("error %q does not mention 'stingy' as a known trait", err)
	}
}
