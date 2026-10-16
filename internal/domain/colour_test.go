package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestGeneratePopulationRejectsUnknownColor(t *testing.T) {
	t.Parallel()
	spec := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: []string{"X"}}
	_, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"a": spec}},
		testSets(), testRNG(1))
	if err == nil {
		t.Fatal("want error for unknown colour X, got nil")
	}
	if !strings.Contains(err.Error(), `"X"`) {
		t.Errorf("error %q does not name the unknown colour X", err)
	}
}

func TestGeneratePopulationRejectsDuplicateColor(t *testing.T) {
	t.Parallel()
	spec := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: []string{"B", "B"}}
	_, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"a": spec}},
		testSets(), testRNG(1))
	if err == nil {
		t.Fatal("want error for duplicate colour B, got nil")
	}
	if !strings.Contains(err.Error(), `"B"`) {
		t.Errorf("error %q does not name the duplicate colour B", err)
	}
}

func TestGeneratePopulationAcceptsColorCombinations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		colors []string
	}{
		{"empty", nil},
		{"single W", []string{"W"}},
		{"single G", []string{"G"}},
		{"two UB", []string{"U", "B"}},
		{"all five", []string{"W", "U", "B", "R", "G"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: tt.colors}
			_, err := domain.GeneratePopulation(
				domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"a": spec}},
				testSets(), testRNG(1))
			if err != nil {
				t.Fatalf("GeneratePopulation: %v", err)
			}
		})
	}
}

func TestGeneratePopulationCopiesColorsToEachNPC(t *testing.T) {
	t.Parallel()
	colors := []string{"U", "B"}
	spec := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: colors}
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 3, Archetypes: map[string]domain.ArchetypeSpec{"a": spec}},
		testSets(), testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	for _, npc := range npcs {
		if !slices.Equal(npc.Colors, colors) {
			t.Errorf("NPC %d: Colors = %v, want %v", npc.ID, npc.Colors, colors)
		}
		// Mutation of the spec's slice must not affect NPCs: each NPC gets its own copy.
		if len(npc.Colors) > 0 && &npc.Colors[0] == &colors[0] {
			t.Errorf("NPC %d: Colors shares the spec's backing array; want an independent copy", npc.ID)
		}
	}
}
