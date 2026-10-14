package demo

import (
	"math/rand/v2"
	"sort"

	"github.com/foutoucour/crackapack/internal/domain"
)

// Price distribution constants (in USDCents). These shape the synthetic price
// snapshot so that binder milestones take several days and the coin rate visibly
// changes the outcome.
const (
	commonLoCents   = 3    // cheapest common: 3 cents
	commonHiCents   = 30   // most expensive common: 30 cents
	uncommonLoCents = 10   // cheapest uncommon: 10 cents
	uncommonHiCents = 100  // most expensive uncommon: $1
	rareLoCents     = 50   // cheapest regular rare: 50 cents
	rareHiCents     = 2000 // most expensive regular rare: $20
	chaseRareCents  = 3000 // chase rare price: $30
	specialLoCents  = 20   // cheapest timeshifted: 20 cents
	specialHiCents  = 200  // most expensive timeshifted: $2
	mythicLoCents   = 200  // cheapest mythic: $2
	mythicHiCents   = 5000 // most expensive mythic: $50
	chaseRareCount  = 3    // first N rares per catalogue (by sorted ID) are chase rares
)

// newDemoPrices builds a PriceSnapshot for all demo catalogues.
//
// Price distribution uses an integer cubic curve — no floating-point arithmetic,
// ensuring bit-identical results on every platform. For each rarity:
//
//   - Common:  3–30 cents
//   - Uncommon: 10–100 cents
//   - Rare:     50–2000 cents (regular); $30 for the first chaseRareCount rares per set
//   - Special:  20–200 cents (timeshifted)
//   - Mythic:   200–5000 cents
//
// Within each catalogue, one non-basic card (the last by sorted ID) is left
// unpriced to exercise the fallback-price path.
func newDemoPrices(cats []domain.SetCatalogue, rng *rand.Rand) domain.PriceSnapshot {
	snap := make(domain.PriceSnapshot)
	for _, cat := range cats {
		ids := sortedNonBasicIDs(cat)
		if len(ids) == 0 {
			continue
		}
		unpricedID := ids[len(ids)-1] // intentionally left without a price
		chaseSet := buildChaseSet(ids, cat)

		for _, id := range ids {
			if id == unpricedID {
				continue
			}
			card := cat.Cards[id]
			var price domain.USDCents
			switch card.Rarity {
			case domain.RarityCommon:
				price = cubicCents(rng, commonLoCents, commonHiCents)
			case domain.RarityUncommon:
				price = cubicCents(rng, uncommonLoCents, uncommonHiCents)
			case domain.RarityRare:
				if chaseSet[id] {
					price = chaseRareCents
				} else {
					price = cubicCents(rng, rareLoCents, rareHiCents)
				}
			case domain.RaritySpecial:
				price = cubicCents(rng, specialLoCents, specialHiCents)
			case domain.RarityMythic:
				price = cubicCents(rng, mythicLoCents, mythicHiCents)
			}
			snap[id] = price
		}
	}
	return snap
}

// cubicCents returns a price in [lo, hi] using an integer cubic curve.
//
// Draw u uniformly in [0, 1000) from rng; result = lo + (hi-lo)*u³/10⁹.
// The cubic mapping concentrates values near lo: 50% of draws (u < 500) land in
// the bottom 12.5% of the range. For commons (3–30 cents) this means most cards
// cost 3–6 cents, with a long tail reaching 30 cents — matching real price skew.
// No floating-point arithmetic is used, so results are identical on every platform.
func cubicCents(rng *rand.Rand, lo, hi int) domain.USDCents {
	u := int64(rng.IntN(1000))
	return domain.USDCents(int64(lo) + int64(hi-lo)*u*u*u/1_000_000_000)
}

// sortedNonBasicIDs returns all non-basic-land card IDs in cat, sorted ascending.
func sortedNonBasicIDs(cat domain.SetCatalogue) []domain.CardID {
	ids := make([]domain.CardID, 0, len(cat.Cards))
	for id, card := range cat.Cards {
		if !card.IsBasicLand {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// buildChaseSet returns the first chaseRareCount rare IDs (by sorted position in
// sortedIDs) as a set. These cards receive the fixed chaseRareCents price.
func buildChaseSet(sortedIDs []domain.CardID, cat domain.SetCatalogue) map[domain.CardID]bool {
	chaseSet := make(map[domain.CardID]bool)
	count := 0
	for _, id := range sortedIDs {
		if cat.Cards[id].Rarity == domain.RarityRare {
			chaseSet[id] = true
			count++
			if count >= chaseRareCount {
				break
			}
		}
	}
	return chaseSet
}
