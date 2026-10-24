package demo

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// DEX is the demo's expensive set. It sits in the catalogue directory but not in SetCodes, so the
// demo shows the expensive bundle being left out of the game and out of the energy cost median.
const (
	dexRareCount   = 5
	dexBoosterRare = 3    // rares in one booster
	dexCardCents   = 4000 // every DEX card is worth $40, so a booster is worth $120
)

// newDEXCatalogue builds the demo expensive set: rares only, in a booster of three. At dexCardCents
// a card, its booster is worth twice the real limit, domain.ExpensiveBoosterValue.
func newDEXCatalogue() domain.SetCatalogue {
	cards := make(map[domain.CardID]domain.Card, dexRareCount)
	rareSheet := make([]domain.SheetCard, 0, dexRareCount)
	for i := range dexRareCount {
		id := domain.CardID(fmt.Sprintf("dex-r-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Expensive Rare %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityRare,
			Finishes: []domain.Finish{domain.FinishNonFoil},
		}
		rareSheet = append(rareSheet, domain.SheetCard{ID: id, Weight: 1})
	}
	sortSheetByID(rareSheet)

	return domain.SetCatalogue{
		Code:        "DEX",
		Name:        "Demo Expensive Set",
		IconCode:    "DEX",
		ReleaseDate: "1993-08-05", // the date of the first real set, the dearest of them all
		Cards:       cards,
		Sheets: map[string]domain.Sheet{
			"rare": {Name: "rare", Cards: rareSheet, TotalWeight: sumWeights(rareSheet)},
		},
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"rare": dexBoosterRare}, Weight: 1},
		},
	}
}

// newDEXPrices prices every card of cat at dexCardCents.
func newDEXPrices(cat domain.SetCatalogue) domain.PriceSnapshot {
	snap := make(domain.PriceSnapshot, len(cat.Cards))
	for id := range cat.Cards {
		snap[id] = dexCardCents
	}
	return snap
}
