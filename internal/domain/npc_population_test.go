package domain_test

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func testSets() []domain.SetCatalogue {
	return []domain.SetCatalogue{{Code: "AAA"}, {Code: "BBB"}, {Code: "CCC"}}
}

func testRNG(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed)) }

func countByArchetype(npcs []*domain.NPC) map[string]int {
	counts := make(map[string]int)
	for _, n := range npcs {
		counts[n.Archetype]++
	}
	return counts
}

func TestGeneratePopulationFollowsArchetypeShares(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		size       int
		shares     map[string]int
		wantCounts map[string]int
	}{
		{
			name:       "shares divide the population exactly",
			size:       10,
			shares:     map[string]int{"casual": 60, "whale": 30, "hunter": 10},
			wantCounts: map[string]int{"casual": 6, "whale": 3, "hunter": 1},
		},
		{
			name:       "leftover NPCs go to the largest remainders",
			size:       10,
			shares:     map[string]int{"casual": 55, "whale": 35, "hunter": 10},
			wantCounts: map[string]int{"casual": 6, "whale": 3, "hunter": 1},
		},
		{
			name:       "ties between remainders are broken by archetype name",
			size:       7,
			shares:     map[string]int{"b": 1, "a": 1, "c": 1},
			wantCounts: map[string]int{"a": 3, "b": 2, "c": 2},
		},
		{
			name:       "an archetype with a zero share has no NPC",
			size:       4,
			shares:     map[string]int{"casual": 1, "whale": 0},
			wantCounts: map[string]int{"casual": 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			archetypes := make(map[string]domain.ArchetypeSpec, len(tt.shares))
			for name, share := range tt.shares {
				archetypes[name] = domain.ArchetypeSpec{Share: share, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1}
			}

			npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: tt.size, Archetypes: archetypes}, testSets(), testRNG(1))
			if err != nil {
				t.Fatalf("GeneratePopulation: %v", err)
			}

			if got := countByArchetype(npcs); !reflect.DeepEqual(got, tt.wantCounts) {
				t.Errorf("counts = %v, want %v", got, tt.wantCounts)
			}
		})
	}
}

func TestGeneratePopulationDrawsTraitsFromTheArchetypeSpec(t *testing.T) {
	t.Parallel()

	params := domain.PopulationParams{
		Size: 200,
		Archetypes: map[string]domain.ArchetypeSpec{
			"hunter": {Share: 1, MinBoostersPerDay: 3, MaxBoostersPerDay: 6, KeepCopies: 4},
			"fan":    {Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 2, KeepCopies: 1, FocusSet: domain.FocusSetRandom},
		},
	}

	npcs, err := domain.GeneratePopulation(params, testSets(), testRNG(7))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}

	seenRhythm := make(map[int]bool)
	sets := map[string]bool{"AAA": true, "BBB": true, "CCC": true}
	for i, n := range npcs {
		if n.ID != i {
			t.Errorf("npcs[%d].ID = %d, want %d", i, n.ID, i)
		}
		spec := params.Archetypes[n.Archetype]
		if n.BoostersPerDay < spec.MinBoostersPerDay || n.BoostersPerDay > spec.MaxBoostersPerDay {
			t.Errorf("NPC %d (%s): BoostersPerDay = %d, want within [%d, %d]", n.ID, n.Archetype, n.BoostersPerDay, spec.MinBoostersPerDay, spec.MaxBoostersPerDay)
		}
		if n.KeepCopies != spec.KeepCopies {
			t.Errorf("NPC %d (%s): KeepCopies = %d, want %d", n.ID, n.Archetype, n.KeepCopies, spec.KeepCopies)
		}
		if spec.FocusSet == domain.FocusSetRandom && !sets[n.FocusSet] {
			t.Errorf("NPC %d (%s): FocusSet = %q, want one of the sets", n.ID, n.Archetype, n.FocusSet)
		}
		if spec.FocusSet == domain.FocusSetNone && n.FocusSet != "" {
			t.Errorf("NPC %d (%s): FocusSet = %q, want none", n.ID, n.Archetype, n.FocusSet)
		}
		if n.Archetype == "hunter" {
			seenRhythm[n.BoostersPerDay] = true
		}
		if n.Collection() == nil {
			t.Errorf("NPC %d has no collection", n.ID)
		}
	}
	if len(seenRhythm) != 4 {
		t.Errorf("hunter rhythms seen = %v, want every value from 3 to 6 over 100 NPCs", seenRhythm)
	}
}

func TestGeneratePopulationIsReproducibleFromTheSeed(t *testing.T) {
	t.Parallel()

	params := domain.PopulationParams{
		Size: 50,
		Archetypes: map[string]domain.ArchetypeSpec{
			"fan": {Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 9, KeepCopies: 1, FocusSet: domain.FocusSetRandom},
		},
	}
	traits := func(seed uint64) [][3]any {
		npcs, err := domain.GeneratePopulation(params, testSets(), testRNG(seed))
		if err != nil {
			t.Fatalf("GeneratePopulation: %v", err)
		}
		out := make([][3]any, len(npcs))
		for i, n := range npcs {
			out[i] = [3]any{n.Archetype, n.BoostersPerDay, n.FocusSet}
		}
		return out
	}

	if !reflect.DeepEqual(traits(3), traits(3)) {
		t.Error("same seed produced different populations")
	}
	if reflect.DeepEqual(traits(3), traits(4)) {
		t.Error("different seeds produced the same population")
	}
}

func TestGeneratePopulationRejectsInvalidParams(t *testing.T) {
	t.Parallel()

	valid := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 2, KeepCopies: 1}
	with := func(mutate func(*domain.ArchetypeSpec)) map[string]domain.ArchetypeSpec {
		spec := valid
		mutate(&spec)
		return map[string]domain.ArchetypeSpec{"casual": spec}
	}
	tests := []struct {
		name       string
		size       int
		archetypes map[string]domain.ArchetypeSpec
		sets       []domain.SetCatalogue
	}{
		{name: "negative size", size: -1, archetypes: with(func(*domain.ArchetypeSpec) {}), sets: testSets()},
		{name: "no archetype", size: 5, archetypes: nil, sets: testSets()},
		{name: "every share zero", size: 5, archetypes: with(func(s *domain.ArchetypeSpec) { s.Share = 0 }), sets: testSets()},
		{name: "negative share", size: 5, archetypes: with(func(s *domain.ArchetypeSpec) { s.Share = -1 }), sets: testSets()},
		{name: "negative minimum rhythm", size: 5, archetypes: with(func(s *domain.ArchetypeSpec) { s.MinBoostersPerDay = -1 }), sets: testSets()},
		{name: "minimum rhythm above maximum", size: 5, archetypes: with(func(s *domain.ArchetypeSpec) { s.MinBoostersPerDay = 3 }), sets: testSets()},
		{name: "keep below one", size: 5, archetypes: with(func(s *domain.ArchetypeSpec) { s.KeepCopies = 0 }), sets: testSets()},
		{name: "keep above a playset", size: 5, archetypes: with(func(s *domain.ArchetypeSpec) { s.KeepCopies = domain.PlaysetSize + 1 }), sets: testSets()},
		{name: "no set to open", size: 5, archetypes: with(func(*domain.ArchetypeSpec) {}), sets: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.GeneratePopulation(domain.PopulationParams{Size: tt.size, Archetypes: tt.archetypes}, tt.sets, testRNG(1)); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestGeneratePopulationOfSizeZeroIsEmpty(t *testing.T) {
	t.Parallel()

	npcs, err := domain.GeneratePopulation(domain.PopulationParams{}, nil, testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	if len(npcs) != 0 {
		t.Errorf("got %d NPCs, want none", len(npcs))
	}
}
