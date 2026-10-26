package domain

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// minNamesPerColour keeps a pool wide enough that a crowd of a dozen rarely has to repeat a name.
const minNamesPerColour = 12

func namedPopulation(t *testing.T, size int, archetypes map[string]ArchetypeSpec) []*NPC {
	t.Helper()
	npcs, err := GeneratePopulation(PopulationParams{Size: size, Archetypes: archetypes}, []SetCatalogue{{Code: "AAA"}}, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	return npcs
}

func archetype(colours ...string) ArchetypeSpec {
	return ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: colours}
}

func TestNPCNames_everyPoolHasEnoughNamesAndNoNameIsInTwoPools(t *testing.T) {
	t.Parallel()
	seen := make(map[string]string)
	for colour, pool := range npcNamePools() {
		if len(pool) < minNamesPerColour {
			t.Errorf("the %q pool has %d names, want at least %d", colour, len(pool), minNamesPerColour)
		}
		for _, name := range pool {
			if other, ok := seen[name]; ok {
				t.Errorf("%q is in the %q pool and in the %q pool", name, other, colour)
			}
			seen[name] = colour
		}
	}
	for _, colour := range append(ValidColors(), colourlessPool) {
		if _, ok := npcNamePools()[colour]; !ok {
			t.Errorf("no pool for %q", colour)
		}
	}
}

func TestNPCNames_aNPCOfOneColourIsNamedAfterThatColour(t *testing.T) {
	t.Parallel()
	for _, colour := range ValidColors() {
		npcs := namedPopulation(t, 3, map[string]ArchetypeSpec{"mono": archetype(colour)})

		for _, npc := range npcs {
			if !slices.Contains(npcNamePools()[colour], npc.Name) {
				t.Errorf("NPC %d of colour %s is named %q, which is not in the %s pool", npc.ID, colour, npc.Name, colour)
			}
		}
	}
}

func TestNPCNames_aNPCOfSeveralColoursSpreadsOverThem(t *testing.T) {
	t.Parallel()
	npcs := namedPopulation(t, 5, map[string]ArchetypeSpec{"whale": archetype(ValidColors()...)})

	for i, colour := range ValidColors() {
		if !slices.Contains(npcNamePools()[colour], npcs[i].Name) {
			t.Errorf("NPC %d is named %q, want a name of %s: the colours of a NPC take turns", i, npcs[i].Name, colour)
		}
	}
}

func TestNPCNames_aNPCOfNoColourIsNamedAfterAColourlessCharacter(t *testing.T) {
	t.Parallel()
	npcs := namedPopulation(t, 2, map[string]ArchetypeSpec{"casual": archetype()})

	for _, npc := range npcs {
		if !slices.Contains(npcNamePools()[colourlessPool], npc.Name) {
			t.Errorf("NPC %d has no colour and is named %q, want a name of the colourless pool", npc.ID, npc.Name)
		}
	}
}

func TestNPCNames_noTwoNPCsShareANameEvenWhenTheirColoursOverlap(t *testing.T) {
	t.Parallel()
	npcs := namedPopulation(t, 12, map[string]ArchetypeSpec{
		"stingy":      archetype("B"),
		"ego_stingy":  archetype("U", "B"),
		"archivist":   archetype("W"),
		"playset_one": archetype(),
	})

	seen := make(map[string]int)
	for _, npc := range npcs {
		if npc.Name == "" {
			t.Errorf("NPC %d has no name", npc.ID)
		}
		if first, ok := seen[npc.Name]; ok {
			t.Errorf("NPC %d and NPC %d are both named %q", first, npc.ID, npc.Name)
		}
		seen[npc.Name] = npc.ID
	}
}

func TestNPCNames_aCrowdBiggerThanItsPoolStillGivesEveryNPCItsOwnName(t *testing.T) {
	t.Parallel()
	size := 3 * len(npcNamePools()["W"])
	npcs := namedPopulation(t, size, map[string]ArchetypeSpec{"white": archetype("W")})

	seen := make(map[string]bool, size)
	for _, npc := range npcs {
		if seen[npc.Name] {
			t.Fatalf("NPC %d repeats the name %q", npc.ID, npc.Name)
		}
		seen[npc.Name] = true
	}
}

func TestNPCNames_theSameSeedGivesTheSameNames(t *testing.T) {
	t.Parallel()
	archetypes := map[string]ArchetypeSpec{"a": archetype("R"), "b": archetype("G", "U")}

	first, second := namedPopulation(t, 8, archetypes), namedPopulation(t, 8, archetypes)

	for i := range first {
		if first[i].Name != second[i].Name {
			t.Errorf("NPC %d is %q, then %q", i, first[i].Name, second[i].Name)
		}
	}
}
