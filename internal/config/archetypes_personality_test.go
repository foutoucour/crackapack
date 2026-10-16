package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// TestEgoArchetype_embeddedDefaults verifies the embedded ego.yaml fields that the story
// mandates: share=0 so existing runs stay byte-identical, colour Blue (U), and trait "ego".
// Behaviour "casual" is the base on which the ego trait mixes.
func TestEgoArchetype_embeddedDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ego, ok := cfg.NPCs.Archetypes["ego"]
	if !ok {
		t.Fatal("archetype 'ego' not found in default config")
	}

	if ego.Share != 0 {
		t.Errorf("ego.Share = %d, want 0: share must be 0 until integration story", ego.Share)
	}
	if len(ego.Colors) != 1 || ego.Colors[0] != "U" {
		t.Errorf("ego.Colors = %v, want [U]", ego.Colors)
	}
	if len(ego.Traits) != 1 || ego.Traits[0] != "ego" {
		t.Errorf("ego.Traits = %v, want [ego]", ego.Traits)
	}
	if ego.Behaviour != "casual" {
		t.Errorf("ego.Behaviour = %q, want \"casual\"", ego.Behaviour)
	}

	egoParams, ok := ego.TraitParams["ego"]
	if !ok {
		t.Fatal("ego.TraitParams[\"ego\"] not found; ego archetype must supply max_retries and endowment_pct_per_day")
	}
	if _, ok := egoParams["max_retries"]; !ok {
		t.Error("ego trait_params missing max_retries")
	}
	if _, ok := egoParams["endowment_pct_per_day"]; !ok {
		t.Error("ego trait_params missing endowment_pct_per_day")
	}
}

// TestGamblerArchetype_embeddedDefaults verifies the embedded gambler.yaml fields: share=0,
// colour Red (R), and trait "gambler".
func TestGamblerArchetype_embeddedDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	gambler, ok := cfg.NPCs.Archetypes["gambler"]
	if !ok {
		t.Fatal("archetype 'gambler' not found in default config")
	}

	if gambler.Share != 0 {
		t.Errorf("gambler.Share = %d, want 0: share must be 0 until integration story", gambler.Share)
	}
	if len(gambler.Colors) != 1 || gambler.Colors[0] != "R" {
		t.Errorf("gambler.Colors = %v, want [R]", gambler.Colors)
	}
	if len(gambler.Traits) != 1 || gambler.Traits[0] != "gambler" {
		t.Errorf("gambler.Traits = %v, want [gambler]", gambler.Traits)
	}
	if gambler.Behaviour != "casual" {
		t.Errorf("gambler.Behaviour = %q, want \"casual\"", gambler.Behaviour)
	}

	gParams, ok := gambler.TraitParams["gambler"]
	if !ok {
		t.Fatal("gambler.TraitParams[\"gambler\"] not found; gambler archetype must supply chance_step_pct etc.")
	}
	for _, key := range []string{"chance_step_pct", "max_offer_chance_pct", "margin_extra_pct", "margin_step_pct"} {
		if _, ok := gParams[key]; !ok {
			t.Errorf("gambler trait_params missing %s", key)
		}
	}
}
