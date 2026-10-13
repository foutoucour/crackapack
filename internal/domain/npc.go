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
	// Behaviour names the registered TradeBehaviour of the archetype, see BehaviourNames. Empty
	// means the archetype does not trade.
	Behaviour string
	// WishSize is how many cards an NPC wishes for at once.
	WishSize int
	// MarginPct is the margin an NPC demands on top of the reference value it gives. It must be
	// >= 0, so an NPC never gives more value than it gets.
	MarginPct Percent
	// OfferChancePct is the chance, per tick, that an NPC with a match proposes a trade to the
	// player. It must be in [0, 100].
	OfferChancePct int
}

// PopulationParams describes the NPC crowd to generate.
type PopulationParams struct {
	// Size is the number of NPCs. Zero means no NPCs.
	Size int
	// Archetypes maps an archetype name to its spec.
	Archetypes map[string]ArchetypeSpec
	// Valuer values cards for the archetypes that have a behaviour. It may be the zero Valuer
	// when none does.
	Valuer Valuer
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
	// Behaviour is how the NPC trades, or nil when its archetype does not trade.
	Behaviour TradeBehaviour
	// WishSize is how many cards the NPC wishes for at once.
	WishSize int
	// OfferChancePct is the chance, per tick, that the NPC proposes a trade when it has a match.
	OfferChancePct int

	collection *Collection
	// wishes is the wish list, and shelf holds the surplus copies the NPC can trade away, per
	// card, with the tick each copy arrived on, oldest first. See npc_shelf.go.
	wishes []Wish
	shelf  map[CardKey][]int
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

	behaviours := make(map[string]TradeBehaviour, len(names))
	for _, name := range names {
		spec := p.Archetypes[name]
		if spec.Behaviour == "" {
			continue
		}
		behaviour, err := NewBehaviour(spec.Behaviour, BehaviourParams{MarginPct: spec.MarginPct, Valuer: p.Valuer})
		if err != nil {
			return nil, fmt.Errorf("population: archetype %q: %w", name, err)
		}
		behaviours[name] = behaviour
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
				Behaviour:      behaviours[name],
				WishSize:       spec.WishSize,
				OfferChancePct: spec.OfferChancePct,
				collection:     NewCollection(),
				shelf:          make(map[CardKey][]int),
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
		case spec.Behaviour != "" && !KnownBehaviour(spec.Behaviour):
			return 0, fmt.Errorf("population: archetype %q: unknown behaviour %q, known behaviours: %v", name, spec.Behaviour, BehaviourNames())
		case spec.WishSize < 0:
			return 0, fmt.Errorf("population: archetype %q: wish size must be >= 0, got %d", name, spec.WishSize)
		case spec.MarginPct < 0:
			return 0, fmt.Errorf("population: archetype %q: margin must be >= 0, got %d", name, spec.MarginPct)
		case spec.OfferChancePct < 0 || spec.OfferChancePct > 100:
			return 0, fmt.Errorf("population: archetype %q: offer chance must be in [0, 100], got %d", name, spec.OfferChancePct)
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
