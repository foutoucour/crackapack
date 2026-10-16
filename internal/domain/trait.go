package domain

import (
	"maps"
	"slices"
)

// Trait is a personality mix-in that shapes how an NPC trades on top of its base behaviour. One
// trait may implement one or more of WishScorer, MarginAdder, CounterVeto, DeclineReactor.
type Trait interface{ Name() string }

// WishScorer ranks a card for the NPC's wish list. Each scorer's score is summed; the NPC ranks
// candidates by their total score, highest first, with the base behaviour's order breaking ties.
// See ADR-0021.
type WishScorer interface{ WishScore(card Card) int }

// MarginAdder adds to the NPC's margin based on the offer. The result is clamped at 0 before
// summing so a single trait never lowers the margin. See ADR-0021.
type MarginAdder interface{ MarginAdd(offer Offer) Percent }

// CounterVeto lets a trait refuse a player's counter-offer to an NPC proposal. One veto from any
// trait makes the Exchange refuse the counter. See ADR-0021.
type CounterVeto interface{ VetoCounter(counter Offer) bool }

// DeclineReactor is notified when the player declines one of the NPC's offers. The Exchange does
// not call it yet.
type DeclineReactor interface{ ReactToDecline(declined Offer) }

// TraitConstructor builds a Trait from the parameter map in its config block. It must reject a
// negative parameter value.
type TraitConstructor func(params map[string]int) (Trait, error)

// TraitRegistry maps a trait name to its constructor.
type TraitRegistry map[string]TraitConstructor

// DefaultTraits returns the built-in trait registry. It is a function, not a package-level
// variable, to avoid the global-variable lint rule. It is empty until personality stories add
// stingy, ego, gambler, fish and archivist.
func DefaultTraits() TraitRegistry { return TraitRegistry{} }

// TraitNames returns the sorted names of the traits in DefaultTraits, for config error messages.
func TraitNames() []string { return slices.Sorted(maps.Keys(DefaultTraits())) }

// KnownTrait reports whether name is a registered trait in DefaultTraits.
func KnownTrait(name string) bool {
	_, ok := DefaultTraits()[name]
	return ok
}
