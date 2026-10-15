package domain

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
)

// traitBehaviour wraps a base TradeBehaviour with a list of traits. It is built by composeTraits
// and implements TradeBehaviour. Propose is delegated unchanged to the base. Margin and PickWishes
// are overridden to apply the trait effects, as described in ADR-0021.
type traitBehaviour struct {
	base         TradeBehaviour
	traits       []Trait
	maxMarginPct Percent

	// Cached type-asserted subsets, built once by composeTraits.
	scorers  []WishScorer
	adders   []MarginAdder
	vetoes   []CounterVeto
	reactors []DeclineReactor
}

// composeTraits wraps base with the traits listed in p. It rejects: an unknown trait name (listing
// known names from p.Registry or DefaultTraits), a duplicate trait name, a constructor error, a
// negative param, and a negative MaxMarginPct.
func composeTraits(base TradeBehaviour, p BehaviourParams) (TradeBehaviour, error) {
	if p.MaxMarginPct < 0 {
		return nil, fmt.Errorf("traits: max_margin_pct must be >= 0, got %d", p.MaxMarginPct)
	}
	registry := p.Registry
	if registry == nil {
		registry = DefaultTraits()
	}

	seen := make(map[string]bool, len(p.Traits))
	built := make([]Trait, 0, len(p.Traits))
	for _, spec := range p.Traits {
		if seen[spec.Name] {
			return nil, fmt.Errorf("trait %q is listed more than once", spec.Name)
		}
		seen[spec.Name] = true

		build, ok := registry[spec.Name]
		if !ok {
			return nil, fmt.Errorf("trait %q is not registered, known traits: %v", spec.Name, sortedRegistryKeys(registry))
		}
		for pname, v := range spec.Params {
			if v < 0 {
				return nil, fmt.Errorf("trait %q: parameter %q must be >= 0, got %d", spec.Name, pname, v)
			}
		}
		t, err := build(spec.Params, p.Env)
		if err != nil {
			return nil, fmt.Errorf("trait %q: %w", spec.Name, err)
		}
		built = append(built, t)
	}

	tb := &traitBehaviour{
		base:         base,
		traits:       built,
		maxMarginPct: p.MaxMarginPct,
	}
	for _, t := range built {
		if s, ok := t.(WishScorer); ok {
			tb.scorers = append(tb.scorers, s)
		}
		if a, ok := t.(MarginAdder); ok {
			tb.adders = append(tb.adders, a)
		}
		if v, ok := t.(CounterVeto); ok {
			tb.vetoes = append(tb.vetoes, v)
		}
		if r, ok := t.(DeclineReactor); ok {
			tb.reactors = append(tb.reactors, r)
		}
	}
	return tb, nil
}

// Propose delegates to the base behaviour unchanged.
func (tb *traitBehaviour) Propose(ctx ProposalContext) (want, give []TradeLine, ok bool) {
	return tb.base.Propose(ctx)
}

// Margin returns the base margin plus the sum of the trait adders, capped at MaxMarginPct. The cap
// never lowers the base margin, and a cap at or below it leaves the base margin alone. A negative
// adder result is clamped at 0. See ADR-0021.
func (tb *traitBehaviour) Margin(offer Offer) Percent {
	return tb.MarginFor(MarginContext{Offer: offer})
}

// MarginFor is Margin with the context the Exchange knows: the NPC and the tick. The Exchange
// prefers it, so the adders can read the NPC's state.
func (tb *traitBehaviour) MarginFor(ctx MarginContext) Percent {
	m := tb.base.Margin(ctx.Offer)
	if len(tb.adders) == 0 {
		return m
	}
	var sum Percent
	for _, a := range tb.adders {
		add := a.MarginAdd(ctx)
		if add < 0 {
			add = 0
		}
		sum += add
	}
	if sum == 0 {
		return m
	}
	// A cap of 0 leaves no room, so the traits add nothing. Reading 0 as "no cap" would let a
	// zero value, the default of an unset field, silently switch the guard against stacked
	// adders off.
	room := tb.maxMarginPct - m
	if room <= 0 {
		return m
	}
	return m + min(sum, room)
}

// PickWishes returns up to n wishes. If no trait implements WishScorer, it delegates straight to
// the base (preserving the rng consumption). Otherwise it asks the base for all candidates, scores
// them by summing each scorer's result, stable-sorts by score descending (the base order breaks
// ties, per ADR-0021) and returns the first n.
func (tb *traitBehaviour) PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish {
	if len(tb.scorers) == 0 {
		return tb.base.PickWishes(ctx, n, rng)
	}
	// Ask the base for every candidate it would return over all of scope; the base rng use is
	// the same as though it were called with len(ctx.Scope).
	candidates := tb.base.PickWishes(ctx, len(ctx.Scope), rng)
	if len(candidates) == 0 || n <= 0 {
		return nil
	}

	// Build a lookup for scoring; we never mutate ctx.Scope.
	cardByID := make(map[CardID]Card, len(ctx.Scope))
	for _, c := range ctx.Scope {
		cardByID[c.ID] = c
	}

	type scoredWish struct {
		wish  Wish
		score int
	}
	items := make([]scoredWish, len(candidates))
	for i, w := range candidates {
		var total int
		if card, ok := cardByID[w.ID]; ok {
			for _, s := range tb.scorers {
				total += s.WishScore(card)
			}
		}
		items[i] = scoredWish{wish: w, score: total}
	}

	// Stable sort descending by score so the base order breaks ties.
	slices.SortStableFunc(items, func(a, b scoredWish) int {
		return cmp.Compare(b.score, a.score)
	})

	out := make([]Wish, 0, min(n, len(items)))
	for i := range min(n, len(items)) {
		out = append(out, items[i].wish)
	}
	return out
}

// VetoesCounter returns true if any trait vetoes the counter offer. The Exchange calls this via a
// local interface assertion in exchange_trade.go so no method is added to TradeBehaviour.
func (tb *traitBehaviour) VetoesCounter(offer Offer) bool {
	for _, v := range tb.vetoes {
		if v.VetoCounter(offer) {
			return true
		}
	}
	return false
}

// ReactToDecline fans the decline context out to every DeclineReactor.
func (tb *traitBehaviour) ReactToDecline(ctx DeclineContext) {
	for _, r := range tb.reactors {
		r.ReactToDecline(ctx)
	}
}

// sortedRegistryKeys returns the sorted keys of the registry for error messages.
func sortedRegistryKeys(r TraitRegistry) []string {
	return slices.Sorted(maps.Keys(r))
}
