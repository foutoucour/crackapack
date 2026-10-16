package domain

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
)

// Wish is a card an NPC wants, and how many copies in total it wants to hold.
type Wish struct {
	ID   CardID
	Want int
}

// WishContext is what a behaviour sees when it picks wishes.
type WishContext struct {
	NPC *NPC
	// Scope is every card the NPC may wish for, sorted by ID. Scopes are shared across NPCs; a
	// behaviour must never mutate Scope.
	Scope []Card
	// Wished holds the cards already on the NPC's wish list.
	Wished map[CardID]struct{}
}

// ProposalContext is what a behaviour sees when it builds an offer to the player.
type ProposalContext struct {
	NPC *NPC
	// Wanted is what the NPC asks of the player: duplicates the player holds that are on the NPC's
	// wish list, capped by what the NPC still wants.
	Wanted []TradeLine
	// Shelf is what the NPC can give.
	Shelf []TradeLine
	// PlayerCopies returns how many copies of a printing the player holds.
	PlayerCopies func(CardID) int
	// MaxCards bounds each side of the offer.
	MaxCards int
}

// TradeBehaviour is how an archetype trades. An archetype names a registered behaviour in the
// configuration, with optional traits mixed on top of it, see ADR-0021. A new personality is a
// trait type, one registry entry and a config entry.
type TradeBehaviour interface {
	// PickWishes returns up to n wishes for the NPC, none of them already in ctx.Wished.
	PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish
	// Margin returns the percentage the NPC demands on top of the reference value it gives. It is
	// never negative.
	Margin(offer Offer) Percent
	// Propose builds an offer to the player: the cards the NPC asks for and the cards it gives.
	// The cards it gives are worth no more than the cards it asks for, once the margin is paid.
	// ok is false when no such offer exists.
	Propose(ctx ProposalContext) (want, give []TradeLine, ok bool)
}

// BehaviourParams are the numbers a behaviour reads. They come from the archetype's configuration.
type BehaviourParams struct {
	MarginPct Percent
	Valuer    Valuer
	// Traits mix on top of the base behaviour, see ADR-0021. NewBehaviour owns what they do.
	Traits []TraitSpec
	// Registry is the trait registry used to build traits. Nil means DefaultTraits.
	Registry TraitRegistry
	// Env is passed to every trait constructor.
	Env TraitEnv
	// MaxMarginPct caps the total margin (base + trait adders), without ever lowering the base
	// margin. Zero leaves no room, so traits add nothing.
	MaxMarginPct Percent
}

type behaviourConstructor func(BehaviourParams) TradeBehaviour

// behaviourConstructors returns the registry of behaviours by name. It is a function, not a
// package-level variable, to avoid the global-variable lint rule.
func behaviourConstructors() map[string]behaviourConstructor {
	return map[string]behaviourConstructor{
		"casual":             func(p BehaviourParams) TradeBehaviour { return casualBehaviour{base{p}} },
		"playset_hunter":     func(p BehaviourParams) TradeBehaviour { return playsetHunterBehaviour{base{p}} },
		"targeted_collector": func(p BehaviourParams) TradeBehaviour { return targetedCollectorBehaviour{base{p}} },
		"speculator":         func(p BehaviourParams) TradeBehaviour { return speculatorBehaviour{base{p}} },
		"whale":              func(p BehaviourParams) TradeBehaviour { return whaleBehaviour{base{p}} },
	}
}

// BehaviourNames returns the registered behaviour names, sorted.
func BehaviourNames() []string {
	return slices.Sorted(maps.Keys(behaviourConstructors()))
}

// KnownBehaviour reports whether name is a registered behaviour.
func KnownBehaviour(name string) bool {
	_, ok := behaviourConstructors()[name]
	return ok
}

// NewBehaviour builds the behaviour registered under name. It rejects an unknown name and a
// negative margin: a negative margin would let the player gain value by trading through a chain
// of NPCs and selling the result to the Counter.
//
// When p.Traits is non-empty, the base behaviour is wrapped with traits via composeTraits. The
// wrapped path may error; the unwrapped path (no traits) is byte-identical to the pre-trait code.
func NewBehaviour(name string, p BehaviourParams) (TradeBehaviour, error) {
	if p.MarginPct < 0 {
		return nil, fmt.Errorf("behaviour %q: margin must be >= 0, got %d", name, p.MarginPct)
	}
	build, ok := behaviourConstructors()[name]
	if !ok {
		return nil, fmt.Errorf("behaviour %q is not registered, known behaviours: %v", name, BehaviourNames())
	}
	base := build(p)
	if len(p.Traits) == 0 {
		return base, nil
	}
	return composeTraits(base, p)
}

// base holds what every behaviour shares: the margin is the configured percentage, and the
// proposal is a swap of wanted duplicates for shelf cards.
type base struct{ params BehaviourParams }

func (b base) Margin(Offer) Percent { return b.params.MarginPct }

func (b base) Propose(ctx ProposalContext) (want, give []TradeLine, ok bool) {
	return proposeSwap(ctx, b.params)
}

// proposeSwap asks for the wanted lines, up to ctx.MaxCards cards, and gives the most it can from
// the shelf without giving more than it asks for, once the margin is paid. Cards the player lacks
// come first, so the offer is worth taking.
func proposeSwap(ctx ProposalContext, p BehaviourParams) (want, give []TradeLine, ok bool) {
	want = capLines(ctx.Wanted, ctx.MaxCards)
	if len(want) == 0 {
		return nil, nil, false
	}
	budget := int64(linesValue(p.Valuer, want)) * 100

	shelf := slices.Clone(ctx.Shelf)
	slices.SortStableFunc(shelf, func(a, b TradeLine) int {
		return cmp.Compare(playerHolds(ctx, a), playerHolds(ctx, b))
	})

	var cards int
	var cents int64
	for _, line := range shelf {
		price := int64(p.Valuer.Price(line.Key.ID))
		qty := 0
		for qty < line.Qty && cards < ctx.MaxCards && (cents+price)*(100+int64(p.MarginPct)) <= budget {
			qty++
			cards++
			cents += price
		}
		if qty > 0 {
			give = append(give, TradeLine{Key: line.Key, Qty: qty})
		}
	}
	if len(give) == 0 {
		return nil, nil, false
	}
	return want, give, true
}

// playerHolds is 1 when the player already holds the printing of line, else 0.
func playerHolds(ctx ProposalContext, line TradeLine) int {
	if ctx.PlayerCopies(line.Key.ID) > 0 {
		return 1
	}
	return 0
}

// capLines keeps lines in order until maxCards cards are reached, shortening the last one.
func capLines(lines []TradeLine, maxCards int) []TradeLine {
	var out []TradeLine
	left := maxCards
	for _, l := range lines {
		if left <= 0 {
			break
		}
		qty := min(l.Qty, left)
		out = append(out, TradeLine{Key: l.Key, Qty: qty})
		left -= qty
	}
	return out
}

// lacking returns the cards of ctx.Scope the NPC holds no copy of and has not wished yet.
func lacking(ctx WishContext) []Card {
	out := make([]Card, 0, len(ctx.Scope))
	for _, card := range ctx.Scope {
		if _, wished := ctx.Wished[card.ID]; wished {
			continue
		}
		if ctx.NPC.collection.Copies(card.ID) > 0 {
			continue
		}
		out = append(out, card)
	}
	return out
}

// pickRandom returns n cards of pool drawn without replacement, or all of them when there are
// fewer. pool is not modified.
func pickRandom(pool []Card, n int, rng *rand.Rand) []Card {
	if n <= 0 {
		return nil
	}
	shuffled := slices.Clone(pool)
	n = min(n, len(shuffled))
	for i := range n {
		j := i + rng.IntN(len(shuffled)-i)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	return shuffled[:n]
}

// firstN returns the first n cards of pool.
func firstN(pool []Card, n int) []Card {
	if n <= 0 {
		return nil
	}
	return pool[:min(n, len(pool))]
}

// wishesOf turns cards into wishes that want copies of each.
func wishesOf(cards []Card, copies int) []Wish {
	wishes := make([]Wish, 0, len(cards))
	for _, card := range cards {
		wishes = append(wishes, Wish{ID: card.ID, Want: copies})
	}
	return wishes
}
