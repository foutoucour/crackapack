package config_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

func defaultArchetypeNames() []string {
	return []string{"archivist", "casual", "ego", "fish", "gambler", "playset_hunter", "speculator", "stingy", "targeted_collector", "whale"}
}

func archetypeNames(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.NPCs.Archetypes))
	for name := range cfg.NPCs.Archetypes {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func TestLoad_npcsDefaultToNoCrowdWithTheEmbeddedArchetypes(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.NPCs.Population != 0 {
		t.Errorf("NPCs.Population = %d, want 0: no crowd unless asked for", cfg.NPCs.Population)
	}
	if got := archetypeNames(cfg); !slices.Equal(got, defaultArchetypeNames()) {
		t.Errorf("archetypes = %v, want %v", got, defaultArchetypeNames())
	}
	if cfg.NPCs.Archetypes["targeted_collector"].FocusSet != config.FocusSetRandom {
		t.Error("targeted_collector has no focus set by default")
	}
}

func TestLoad_npcsPopulationKeepsTheDefaultArchetypes(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("npcs:\n  population: 500\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.NPCs.Population != 500 {
		t.Errorf("NPCs.Population = %d, want 500", cfg.NPCs.Population)
	}
	if got := archetypeNames(cfg); !slices.Equal(got, defaultArchetypeNames()) {
		t.Errorf("archetypes = %v, want the defaults %v", got, defaultArchetypeNames())
	}
}

func TestLoad_npcsArchetypeOverrideReplacesOnlyThatArchetype(t *testing.T) {
	t.Parallel()
	const yaml = `
npcs:
  archetypes:
    casual: {share: 1, min_boosters_per_day: 2, max_boosters_per_day: 3, keep_copies: 2}
`
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := config.ArchetypeConfig{Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 3, KeepCopies: 2}
	if got := cfg.NPCs.Archetypes["casual"]; !reflect.DeepEqual(got, want) {
		t.Errorf("casual = %+v, want %+v", got, want)
	}
	if got := archetypeNames(cfg); !slices.Equal(got, defaultArchetypeNames()) {
		t.Errorf("archetypes = %v, want the other defaults kept: %v", got, defaultArchetypeNames())
	}
}

func TestLoad_npcsRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantKey string
	}{
		{
			name:    "negative population",
			yaml:    "npcs: {population: -1}",
			wantKey: "npcs.population",
		},
		{
			name:    "archetype given without its other fields",
			yaml:    "npcs: {archetypes: {casual: {share: 10}}}",
			wantKey: "npcs.archetypes.casual.keep_copies",
		},
		{
			name:    "negative share",
			yaml:    "npcs: {archetypes: {casual: {share: -1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}}}",
			wantKey: "npcs.archetypes.casual.share",
		},
		{
			name:    "negative minimum rhythm",
			yaml:    "npcs: {archetypes: {casual: {share: 1, min_boosters_per_day: -1, max_boosters_per_day: 1, keep_copies: 1}}}",
			wantKey: "npcs.archetypes.casual.min_boosters_per_day",
		},
		{
			name:    "maximum rhythm below minimum",
			yaml:    "npcs: {archetypes: {casual: {share: 1, min_boosters_per_day: 5, max_boosters_per_day: 4, keep_copies: 1}}}",
			wantKey: "npcs.archetypes.casual.max_boosters_per_day",
		},
		{
			name:    "keep above a playset",
			yaml:    "npcs: {archetypes: {casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 5}}}",
			wantKey: "npcs.archetypes.casual.keep_copies",
		},
		{
			name: "no share above zero with a crowd to build",
			yaml: `
npcs:
  population: 10
  archetypes:
    casual: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    playset_hunter: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    speculator: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    targeted_collector: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    whale: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
`,
			wantKey: "npcs.archetypes",
		},
		{
			name:    "unknown key",
			yaml:    "npcs: {populaton: 5}",
			wantKey: "populaton",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tt.yaml))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantKey) {
				t.Errorf("error %q does not name %q", err, tt.wantKey)
			}
		})
	}
}

func TestLoad_npcsAcceptsNoShareWhenThereIsNoCrowd(t *testing.T) {
	t.Parallel()
	const yaml = `
npcs:
  archetypes:
    casual: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    playset_hunter: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    speculator: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    targeted_collector: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    whale: {share: 0, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load: %v", err)
	}
}
