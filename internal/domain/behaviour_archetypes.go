package domain

import (
	"cmp"
	"math/rand/v2"
	"slices"
)

// casualBehaviour wishes for any card it lacks, at random.
type casualBehaviour struct{ base }

func (b casualBehaviour) PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish {
	return wishesOf(pickRandom(lacking(ctx), n, rng), 1)
}

// playsetHunterBehaviour wishes for commons and uncommons it holds fewer than a playset of.
type playsetHunterBehaviour struct{ base }

func (b playsetHunterBehaviour) PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish {
	var pool []Card
	for _, card := range ctx.Scope {
		if card.Rarity != RarityCommon && card.Rarity != RarityUncommon {
			continue
		}
		if _, wished := ctx.Wished[card.ID]; wished {
			continue
		}
		if ctx.NPC.collection.Copies(card.ID) >= PlaysetSize {
			continue
		}
		pool = append(pool, card)
	}
	return wishesOf(pickRandom(pool, n, rng), PlaysetSize)
}

// targetedCollectorBehaviour completes its set from the rarest cards down.
type targetedCollectorBehaviour struct{ base }

func (b targetedCollectorBehaviour) PickWishes(ctx WishContext, n int, _ *rand.Rand) []Wish {
	pool := lacking(ctx)
	slices.SortStableFunc(pool, func(a, b Card) int { return cmp.Compare(collectorRank(b.Rarity), collectorRank(a.Rarity)) })
	return wishesOf(firstN(pool, n), 1)
}

// collectorRank orders rarities for a collector: a rare or special card comes before an uncommon,
// which comes before a common.
func collectorRank(r Rarity) int {
	switch r {
	case RarityRare, RaritySpecial:
		return 2
	case RarityUncommon:
		return 1
	default:
		return 0
	}
}

// speculatorBehaviour wishes for the most valuable cards it lacks, and asks for the most valuable
// duplicates first.
type speculatorBehaviour struct{ base }

func (b speculatorBehaviour) PickWishes(ctx WishContext, n int, _ *rand.Rand) []Wish {
	pool := lacking(ctx)
	slices.SortStableFunc(pool, func(x, y Card) int {
		return cmp.Compare(b.params.Valuer.Price(y.ID), b.params.Valuer.Price(x.ID))
	})
	return wishesOf(firstN(pool, n), 1)
}

func (b speculatorBehaviour) Propose(ctx ProposalContext) (want, give []TradeLine, ok bool) {
	wanted := slices.Clone(ctx.Wanted)
	slices.SortStableFunc(wanted, func(x, y TradeLine) int {
		return cmp.Compare(b.params.Valuer.Price(y.Key.ID), b.params.Valuer.Price(x.Key.ID))
	})
	ctx.Wanted = wanted
	return proposeSwap(ctx, b.params)
}

// whaleBehaviour goes for rares and specials, at random.
type whaleBehaviour struct{ base }

func (b whaleBehaviour) PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish {
	var pool []Card
	for _, card := range lacking(ctx) {
		if card.Rarity == RarityRare || card.Rarity == RaritySpecial {
			pool = append(pool, card)
		}
	}
	return wishesOf(pickRandom(pool, n, rng), 1)
}
