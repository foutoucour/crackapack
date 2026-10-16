package domaintest

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// NewTimeSpiralCatalogues returns the TSP (Time Spiral) set catalogue and the
// separate TSB (Time Spiral Timeshifted) catalogue.
//
// The TSP booster sheets reference TSB card IDs (tsp-tsb-000 … tsp-tsb-009).
// Those IDs are intentionally absent from tsp.Cards — exactly as in the real
// MTGJSON files where TSB is a separate set. The classicGenerator assigns them
// reveal-rank 3 (unknown), placing them between special and rare, as documented
// in booster.go.
//
// TSP card inventory (all in tsp.Cards):
//   - 15 commons  (tsp-c-000 … tsp-c-014), colours cycling W U B R G (3 per colour)
//   - 4  uncommons (tsp-u-000 … tsp-u-003), colourless
//   - 1  rare      (tsp-r-000), colourless
//
// TSB card inventory (all in tsb.Cards, NOT in tsp.Cards):
//   - 10 timeshifted cards (tsp-tsb-000 … tsp-tsb-009), RaritySpecial
//
// TSP sheets:
//   - "common"      15 TSP commons, BalanceColors=true, Foil=false, equal weight 10
//   - "uncommon"     4 TSP uncommons, Foil=false, equal weight 10
//   - "rare"         1 TSP rare, weight 10
//   - "timeshifted" 10 TSB card IDs (absent from tsp.Cards), Foil=false, equal weight 10
//   - "foil"        30 card IDs (15 TSP common + 4 TSP uncommon + 1 TSP rare + 10 TSB),
//     Foil=true. Weights encode 50% common / 25% uncommon / 12.5% rare /
//     12.5% timeshifted by rarity group:
//     Total = 240
//     w_common     =  8  (15 × 8  = 120 = 50%)
//     w_uncommon   = 15  ( 4 × 15 =  60 = 25%)
//     w_rare       = 30  ( 1 × 30 =  30 = 12.5%)
//     w_timeshifted=  3  (10 ×  3 =  30 = 12.5%)
//
// TSP BoosterOptions (weight sum 1000, both produce 15 cards):
//
//	Option 1 (775): 10 common + 3 uncommon + 1 rare + 1 timeshifted        (no foil)
//	Option 2 (225): 9 common + 3 uncommon + 1 rare + 1 timeshifted + 1 foil (22.5% foil)
func NewTimeSpiralCatalogues() (tsp, tsb domain.SetCatalogue) {
	tspCards := make(map[domain.CardID]domain.Card)
	tspColors := [5]string{"W", "U", "B", "R", "G"}

	// 15 TSP commons — 3 per colour (W W W U U U B B B R R R G G G ordering by i%5).
	commonSheet := make([]domain.SheetCard, 0, 15)
	// foilSheet collects all 30 entries: 15 common + 4 uncommon + 1 rare + 10 TSB.
	foilSheet := make([]domain.SheetCard, 0, 30)

	for i := range 15 {
		id := domain.CardID(fmt.Sprintf("tsp-c-%03d", i))
		tspCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Time Spiral Common %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityCommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			Colors:   []string{tspColors[i%5]},
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 8})
	}

	// 4 TSP uncommons.
	uncommonSheet := make([]domain.SheetCard, 0, 4)
	for i := range 4 {
		id := domain.CardID(fmt.Sprintf("tsp-u-%03d", i))
		tspCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Time Spiral Uncommon %d", i),
			Number:   fmt.Sprintf("%d", 16+i),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		uncommonSheet = append(uncommonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 15})
	}

	// 1 TSP rare.
	rareID := domain.CardID("tsp-r-000")
	tspCards[rareID] = domain.Card{
		ID:       rareID,
		Name:     "Time Spiral Rare",
		Number:   "20",
		Rarity:   domain.RarityRare,
		Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
	}
	rareSheet := []domain.SheetCard{{ID: rareID, Weight: 10}}
	foilSheet = append(foilSheet, domain.SheetCard{ID: rareID, Weight: 30})

	// 10 TSB timeshifted cards — added to TSB's catalogue and to TSP's sheets, but
	// deliberately NOT added to tsp.Cards (mirroring the real MTGJSON split).
	timeshiftedSheet := make([]domain.SheetCard, 0, 10)
	tsbCards := make(map[domain.CardID]domain.Card)
	tsbSpecialSheet := make([]domain.SheetCard, 0, 10)

	for i := range 10 {
		id := domain.CardID(fmt.Sprintf("tsp-tsb-%03d", i))
		tsbCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Time Spiral Timeshifted %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RaritySpecial,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		timeshiftedSheet = append(timeshiftedSheet, domain.SheetCard{ID: id, Weight: 10})
		tsbSpecialSheet = append(tsbSpecialSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 3})
	}

	sortSheetByID(commonSheet)
	sortSheetByID(uncommonSheet)
	sortSheetByID(timeshiftedSheet)
	sortSheetByID(tsbSpecialSheet)
	sortSheetByID(foilSheet)

	tsp = domain.SetCatalogue{
		Code:        "TSP",
		Name:        "Time Spiral (synthetic)",
		ReleaseDate: "2006-10-06", // real release date
		Cards:       tspCards,
		Sheets: map[string]domain.Sheet{
			"common": {
				Name:          "common",
				Cards:         commonSheet,
				TotalWeight:   sumWeights(commonSheet),
				Foil:          false,
				BalanceColors: true,
			},
			"uncommon": {
				Name:        "uncommon",
				Cards:       uncommonSheet,
				TotalWeight: sumWeights(uncommonSheet),
			},
			"rare": {
				Name:        "rare",
				Cards:       rareSheet,
				TotalWeight: sumWeights(rareSheet),
			},
			"timeshifted": {
				Name:        "timeshifted",
				Cards:       timeshiftedSheet,
				TotalWeight: sumWeights(timeshiftedSheet),
				Foil:        false,
			},
			"foil": {
				Name:        "foil",
				Cards:       foilSheet,
				TotalWeight: sumWeights(foilSheet),
				Foil:        true,
			},
		},
		BoosterOptions: []domain.BoosterOption{
			{
				Contents: map[string]int{"common": 10, "uncommon": 3, "rare": 1, "timeshifted": 1},
				Weight:   775,
			},
			{
				Contents: map[string]int{"common": 9, "uncommon": 3, "rare": 1, "timeshifted": 1, "foil": 1},
				Weight:   225,
			},
		},
	}

	tsb = domain.SetCatalogue{
		Code:  "TSB",
		Name:  "Time Spiral Timeshifted (synthetic)",
		Cards: tsbCards,
		Sheets: map[string]domain.Sheet{
			"timeshifted": {
				Name:        "timeshifted",
				Cards:       tsbSpecialSheet,
				TotalWeight: sumWeights(tsbSpecialSheet),
				Foil:        false,
			},
		},
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"timeshifted": 1}, Weight: 1},
		},
	}

	return tsp, tsb
}
