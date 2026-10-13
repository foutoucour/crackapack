package domain

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
)

// ArchetypeSpec is the supply behaviour of one NPC archetype. Archetypes are data, not code: a
// new personality is a new entry in the configuration, see ADR-0009.
type ArchetypeSpec struct {
	// Share is the archetype's weight in the population. Only the ratio between shares matters.
	Share int
	// MinBoostersPerDay and MaxBoostersPerDay bound the activity rhythm, inclusive. Each NPC
	// draws its own rhythm in that range once, at generation.
	MinBoostersPerDay int
	MaxBoostersPerDay int
	// KeepCopies is how many copies of a printing the NPC keeps. It sells the rest to the
	// Counter. It must be in [1, PlaysetSize].
	KeepCopies int
	// FocusSet makes each NPC of the archetype open only one set, drawn at generation.
	FocusSet bool
}

// PopulationParams describes the NPC crowd to generate.
type PopulationParams struct {
	// Size is the number of NPCs. Zero means no NPCs.
	Size int
	// Archetypes maps an archetype name to its spec.
	Archetypes map[string]ArchetypeSpec
}

// NPC is one simulated collector. It opens boosters with the player's engine, keeps what its
// archetype keeps and sells the surplus to the Counter. It holds no coins.
type NPC struct {
	ID             int
	Archetype      string
	BoostersPerDay int
	KeepCopies     int
	// FocusSet is the code of the only set the NPC opens, or empty when it opens any set.
	FocusSet string

	collection *Collection
	// credit accumulates BoostersPerDay each tick; a booster is due every TicksPerDay of credit.
	credit int
}

// Collection returns the cards the NPC holds.
func (n *NPC) Collection() *Collection { return n.collection }

// GeneratePopulation builds p.Size NPCs, numbered from 0, grouped by archetype name. Each
// archetype gets its share of the population, with leftover NPCs going to the largest
// remainders, so the mix is exact and does not depend on the seed. Rhythms and focus sets are
// drawn from rng, so the population is reproducible from the seed.
//
// sets are the openable catalogues, in a stable order. A population of size zero is valid whatever
// the archetypes and sets.
func GeneratePopulation(p PopulationParams, sets []SetCatalogue, rng *rand.Rand) ([]*NPC, error) {
	if p.Size < 0 {
		return nil, fmt.Errorf("population: size must be >= 0, got %d", p.Size)
	}
	if p.Size == 0 {
		return nil, nil
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("population: at least one set is required to open boosters from")
	}
	names := slices.Sorted(maps.Keys(p.Archetypes))
	totalShare, err := validateArchetypes(names, p.Archetypes)
	if err != nil {
		return nil, err
	}

	counts := archetypeCounts(p.Size, totalShare, names, p.Archetypes)
	npcs := make([]*NPC, 0, p.Size)
	for _, name := range names {
		spec := p.Archetypes[name]
		for range counts[name] {
			npc := &NPC{
				ID:             len(npcs),
				Archetype:      name,
				BoostersPerDay: spec.MinBoostersPerDay + rng.IntN(spec.MaxBoostersPerDay-spec.MinBoostersPerDay+1),
				KeepCopies:     spec.KeepCopies,
				collection:     NewCollection(),
			}
			if spec.FocusSet {
				npc.FocusSet = sets[rng.IntN(len(sets))].Code
			}
			npcs = append(npcs, npc)
		}
	}
	return npcs, nil
}

// validateArchetypes checks every spec and returns the sum of the shares.
func validateArchetypes(names []string, archetypes map[string]ArchetypeSpec) (int, error) {
	var totalShare int
	for _, name := range names {
		spec := archetypes[name]
		switch {
		case spec.Share < 0:
			return 0, fmt.Errorf("population: archetype %q: share must be >= 0, got %d", name, spec.Share)
		case spec.MinBoostersPerDay < 0:
			return 0, fmt.Errorf("population: archetype %q: min boosters per day must be >= 0, got %d", name, spec.MinBoostersPerDay)
		case spec.MaxBoostersPerDay < spec.MinBoostersPerDay:
			return 0, fmt.Errorf("population: archetype %q: max boosters per day (%d) must be >= min (%d)", name, spec.MaxBoostersPerDay, spec.MinBoostersPerDay)
		case spec.KeepCopies < 1 || spec.KeepCopies > PlaysetSize:
			return 0, fmt.Errorf("population: archetype %q: keep copies must be in [1, %d], got %d", name, PlaysetSize, spec.KeepCopies)
		}
		totalShare += spec.Share
	}
	if totalShare == 0 {
		return 0, fmt.Errorf("population: at least one archetype with a share above zero is required")
	}
	return totalShare, nil
}

// archetypeCounts splits size NPCs between archetypes in proportion to their shares, using the
// largest remainder method. Ties between remainders go to the name that sorts first.
func archetypeCounts(size, totalShare int, names []string, archetypes map[string]ArchetypeSpec) map[string]int {
	counts := make(map[string]int, len(names))
	remainders := make(map[string]int, len(names))
	assigned := 0
	for _, name := range names {
		exact := size * archetypes[name].Share
		counts[name] = exact / totalShare
		remainders[name] = exact % totalShare
		assigned += counts[name]
	}

	byRemainder := slices.Clone(names)
	slices.SortStableFunc(byRemainder, func(a, b string) int { return cmp.Compare(remainders[b], remainders[a]) })
	for _, name := range byRemainder[:size-assigned] {
		counts[name]++
	}
	return counts
}
