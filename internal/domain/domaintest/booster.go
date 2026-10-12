package domaintest

import (
	"fmt"
	"sort"

	"github.com/foutoucour/crackapack/internal/domain"
)

// NewRavnicaCatalogue returns a deterministic synthetic SetCatalogue that reproduces
// the Ravnica-block booster structure as documented in docs/core-features.md.
//
// Cards:
//   - 11 commons  (rav-c-000 … rav-c-010), colours cycling W U B R G W U B R G W
//   - 4  uncommons (rav-u-000 … rav-u-003), colourless
//   - 1  rare      (rav-r-000), colourless
//   - 3  basic lands in the foil sheet only (rav-b-000 … rav-b-002), colourless
//
// Sheets:
//   - "common"   (11 cards, BalanceColors=false, Foil=false, equal weight=10)
//   - "uncommon" (4 cards, Foil=false, equal weight=10)
//   - "rare"     (1 card, Foil=false, weight=10)
//   - "foil"     (16 cards = 11 common + 4 uncommon + 1 rare, Foil=true, equal weight=10)
//
// BoosterOptions (total weight 1000) reproduce the documented rates:
//
//	Option 1 (795): no foil — 11 common + 3 uncommon + 1 rare
//	Option 2 (139): foil replaces a common — 10 common + 3 uncommon + 1 rare + 1 foil
//	Option 3 ( 43): foil replaces an uncommon — 11 common + 2 uncommon + 1 rare + 1 foil
//	Option 4 ( 14): foil replaces the rare — 11 common + 3 uncommon + 1 foil
//	Option 5 (  9): two foils replace two commons — 9 common + 3 uncommon + 1 rare + 2 foil
//	                (assumption: both foils come from the common pool; the doc says "a few packs
//	                hold two foils" with no explicit per-foil rarity breakdown for this case)
//
// Rates implied by these weights (per 1000 packs):
//   - Packs with ≥1 foil:                     205 = 20.5%
//   - Two-foil packs:                            9 =  0.9%
//   - Foil cards replacing a common slot:      157 = 15.7%  (139×1 + 9×2)
//   - Foil cards replacing an uncommon slot:    43 =  4.3%
//   - Foil cards replacing the rare slot:       14 =  1.4%
func NewRavnicaCatalogue() domain.SetCatalogue {
	cards := make(map[domain.CardID]domain.Card)

	ravColors := [5]string{"W", "U", "B", "R", "G"}

	// 11 commons — colours cycle W U B R G W U B R G W.
	commonSheet := make([]domain.SheetCard, 0, 11)
	foilSheet := make([]domain.SheetCard, 0, 16)
	for i := range 11 {
		id := domain.CardID(fmt.Sprintf("rav-c-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Ravnica Common %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityCommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			Colors:   []string{ravColors[i%5]},
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 10})
	}

	// 4 uncommons.
	uncommonSheet := make([]domain.SheetCard, 0, 4)
	for i := range 4 {
		id := domain.CardID(fmt.Sprintf("rav-u-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Ravnica Uncommon %d", i),
			Number:   fmt.Sprintf("%d", 12+i),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		uncommonSheet = append(uncommonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 10})
	}

	// 1 rare.
	rareID := domain.CardID("rav-r-000")
	cards[rareID] = domain.Card{
		ID:       rareID,
		Name:     "Ravnica Rare",
		Number:   "16",
		Rarity:   domain.RarityRare,
		Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
	}
	rareSheet := []domain.SheetCard{{ID: rareID, Weight: 10}}
	foilSheet = append(foilSheet, domain.SheetCard{ID: rareID, Weight: 10})

	sortSheetByID(commonSheet)
	sortSheetByID(uncommonSheet)
	sortSheetByID(foilSheet)

	sheets := map[string]domain.Sheet{
		"common": {
			Name:          "common",
			Cards:         commonSheet,
			TotalWeight:   sumWeights(commonSheet),
			Foil:          false,
			BalanceColors: false, // Ravnica block is not colour-balanced in MTGJSON.
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
		"foil": {
			Name:        "foil",
			Cards:       foilSheet,
			TotalWeight: sumWeights(foilSheet),
			Foil:        true,
		},
	}

	return domain.SetCatalogue{
		Code:   "RAV",
		Name:   "Ravnica: City of Guilds (synthetic)",
		Cards:  cards,
		Sheets: sheets,
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"common": 11, "uncommon": 3, "rare": 1}, Weight: 795},
			{Contents: map[string]int{"common": 10, "uncommon": 3, "rare": 1, "foil": 1}, Weight: 139},
			{Contents: map[string]int{"common": 11, "uncommon": 2, "rare": 1, "foil": 1}, Weight: 43},
			{Contents: map[string]int{"common": 11, "uncommon": 3, "foil": 1}, Weight: 14},
			{Contents: map[string]int{"common": 9, "uncommon": 3, "rare": 1, "foil": 2}, Weight: 9},
		},
	}
}

// NewClassicCatalogue returns a deterministic synthetic SetCatalogue that reproduces the
// non-Ravnica classic booster structure (Coldsnap / Time Spiral / Future Sight style) as
// documented in docs/core-features.md.
//
// Cards:
//   - 15 commons (cls-c-000 … cls-c-014), colours W×3 U×3 B×3 R×3 G×3 (3 per colour)
//   - 4  uncommons (cls-u-000 … cls-u-003), colourless
//   - 1  rare      (cls-r-000), colourless
//
// Sheets:
//   - "common"   (15 cards, BalanceColors=true,  Foil=false, equal weight=10)
//   - "uncommon" (4 cards, BalanceColors=false, Foil=false, equal weight=10)
//   - "rare"     (1 card, Foil=false, weight=10)
//   - "foil"     (20 cards = all above, Foil=true, weights 48/55/60 → 60/25/15% by rarity)
//
// Foil-sheet rarity weights:
//
//	Each common:   48  (15×48  = 720, 720/1200 = 60%)
//	Each uncommon: 75  (4×75   = 300, 300/1200 = 25%)
//	1 rare:        180 (1×180  = 180, 180/1200 = 15%)
//	Total:                       1200
//
// BoosterOptions (total weight 1000):
//
//	Option 1 (775): no foil — 11 common + 3 uncommon + 1 rare
//	Option 2 (225): one foil replaces a common — 10 common + 3 uncommon + 1 rare + 1 foil
//
// Rates: 22.5% of packs contain a foil. Foil rarity distribution: 60% common, 25% uncommon,
// 15% rare (from the foil-sheet weights above).
func NewClassicCatalogue() domain.SetCatalogue {
	cards := make(map[domain.CardID]domain.Card)

	clsColors := [5]string{"W", "U", "B", "R", "G"}

	// 15 commons — 3 per colour.
	commonSheet := make([]domain.SheetCard, 0, 15)
	foilSheet := make([]domain.SheetCard, 0, 20)
	for i := range 15 {
		id := domain.CardID(fmt.Sprintf("cls-c-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Classic Common %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityCommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			Colors:   []string{clsColors[i%5]},
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 48})
	}

	// 4 uncommons.
	uncommonSheet := make([]domain.SheetCard, 0, 4)
	for i := range 4 {
		id := domain.CardID(fmt.Sprintf("cls-u-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Classic Uncommon %d", i),
			Number:   fmt.Sprintf("%d", 16+i),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		uncommonSheet = append(uncommonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 75})
	}

	// 1 rare.
	rareID := domain.CardID("cls-r-000")
	cards[rareID] = domain.Card{
		ID:       rareID,
		Name:     "Classic Rare",
		Number:   "20",
		Rarity:   domain.RarityRare,
		Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
	}
	rareSheet := []domain.SheetCard{{ID: rareID, Weight: 10}}
	foilSheet = append(foilSheet, domain.SheetCard{ID: rareID, Weight: 180})

	sortSheetByID(commonSheet)
	sortSheetByID(uncommonSheet)
	sortSheetByID(foilSheet)

	// Verify foil-sheet total weight is 1200: 15×48 + 4×75 + 180 = 720+300+180 = 1200.
	sheets := map[string]domain.Sheet{
		"common": {
			Name:          "common",
			Cards:         commonSheet,
			TotalWeight:   sumWeights(commonSheet),
			Foil:          false,
			BalanceColors: true, // Coldsnap/Time Spiral/Future Sight style.
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
		"foil": {
			Name:        "foil",
			Cards:       foilSheet,
			TotalWeight: sumWeights(foilSheet),
			Foil:        true,
		},
	}

	return domain.SetCatalogue{
		Code:   "CLS",
		Name:   "Classic Set (synthetic)",
		Cards:  cards,
		Sheets: sheets,
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"common": 11, "uncommon": 3, "rare": 1}, Weight: 775},
			{Contents: map[string]int{"common": 10, "uncommon": 3, "rare": 1, "foil": 1}, Weight: 225},
		},
	}
}

// sortSheetByID sorts a SheetCard slice by ascending ID, matching the contract of Sheet.Cards.
func sortSheetByID(s []domain.SheetCard) {
	sort.Slice(s, func(i, j int) bool { return s[i].ID < s[j].ID })
}
