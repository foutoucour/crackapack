package domain

import "sort"

// DrawableCards returns the cards of pool that the booster sheets of cat can draw and cat does not
// hold itself, sorted by ID. The timeshifted cards of Time Spiral are such a pool: a separate set
// that the boosters of Time Spiral draw from.
//
// A card of pool that no sheet references is left out, since no booster can give it to anyone.
func DrawableCards(cat, pool SetCatalogue) []Card {
	seen := make(map[CardID]bool)
	var out []Card
	for _, sheet := range cat.Sheets {
		for _, sc := range sheet.Cards {
			if _, inSet := cat.Cards[sc.ID]; inSet || seen[sc.ID] {
				continue
			}
			seen[sc.ID] = true
			if card, ok := pool.Cards[sc.ID]; ok {
				out = append(out, card)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// WithPools returns cat with the drawable cards of each pool added to its cards: the catalogue that
// a binder is counted over, see docs/core-features.md#collection-and-binders. cat's own card map is
// not written to.
func WithPools(cat SetCatalogue, pools []SetCatalogue) SetCatalogue {
	merged := cat
	merged.Cards = make(map[CardID]Card, len(cat.Cards))
	for id, card := range cat.Cards {
		merged.Cards[id] = card
	}
	for _, pool := range pools {
		for _, card := range DrawableCards(cat, pool) {
			merged.Cards[card.ID] = card
		}
	}
	return merged
}
