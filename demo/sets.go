// Package demo provides a deterministic synthetic dataset for the crackapack demo
// command. It is generated programmatically from a fixed seed; no JSON data files
// are committed. The domain never imports this package.
package demo

import (
	"fmt"
	"sort"

	"github.com/foutoucour/crackapack/internal/domain"
)

// DEM card counts.
const (
	demCommonCount   = 80 // 16 per colour (W U B R G)
	demUncommonCount = 30
	demRareCount     = 15
	demBasicCount    = 5
)

// DEM booster option weights (Ravnica-style five-option foil pattern, total 1000).
// Source: domaintest.NewRavnicaCatalogue in internal/domain/domaintest/booster.go.
const (
	demOptW1 = 795 // 11 common + 3 uncommon + 1 rare (no foil)
	demOptW2 = 139 // 10 common + 3 uncommon + 1 rare + 1 foil
	demOptW3 = 43  // 11 common + 2 uncommon + 1 rare + 1 foil
	demOptW4 = 14  // 11 common + 3 uncommon + 1 foil (foil replaces rare)
	demOptW5 = 9   // 9 common + 3 uncommon + 1 rare + 2 foil
)

// DSP and DST card counts.
const (
	dspCommonCount      = 60 // 12 per colour (W U B R G)
	dspUncommonCount    = 20
	dspRareCount        = 10
	dspBasicCount       = 5
	dstTimeshiftedCount = 20 // supplementary timeshifted specials (RaritySpecial)
)

// DSP booster option weights (Time Spiral-style two-option pattern, total 1000).
// Source: domaintest.NewTimeSpiralCatalogues in internal/domain/domaintest/timespiral.go.
const (
	dspOptW1 = 775 // 10 common + 3 uncommon + 1 rare + 1 timeshifted (no foil)
	dspOptW2 = 225 // 9 common + 3 uncommon + 1 rare + 1 timeshifted + 1 foil
)

// newDEMCatalogue builds the demo classic set (Ravnica-style layout).
//
// 80 commons (16 per colour W/U/B/R/G) + 30 uncommons + 15 rares + 5 basics.
// Common sheet: 85 cards, BalanceColors=false (Ravnica-block style).
// Foil sheet: 125 non-basic cards (commons + uncommons + rares).
// Booster options: five-option Ravnica pattern, weights sum to 1000.
func newDEMCatalogue() domain.SetCatalogue {
	// Mono colours cycling W U B R G for colour-balanced draws.
	demColors := [5]string{"W", "U", "B", "R", "G"}

	cards := make(map[domain.CardID]domain.Card)

	// Commons: dem-c-000 to dem-c-079, colours cycling W,U,B,R,G.
	commonSheet := make([]domain.SheetCard, 0, demCommonCount+demBasicCount)
	foilSheet := make([]domain.SheetCard, 0, demCommonCount+demUncommonCount+demRareCount)
	for i := range demCommonCount {
		id := domain.CardID(fmt.Sprintf("dem-c-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Common %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityCommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			Colors:   []string{demColors[i%5]},
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
		// Foil rarity weights mirror NewClassicCatalogue: 60%/25%/15% by rarity group.
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 48})
	}

	// Basic lands: dem-b-000 to dem-b-004 (in common sheet only, not foil).
	for i := range demBasicCount {
		id := domain.CardID(fmt.Sprintf("dem-b-%03d", i))
		cards[id] = domain.Card{
			ID:          id,
			Name:        fmt.Sprintf("Demo Basic Land %d", i),
			Number:      fmt.Sprintf("%d", demCommonCount+i+1),
			Rarity:      domain.RarityCommon,
			Finishes:    []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			IsBasicLand: true,
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
	}

	// Uncommons: dem-u-000 to dem-u-029.
	uncommonSheet := make([]domain.SheetCard, 0, demUncommonCount)
	for i := range demUncommonCount {
		id := domain.CardID(fmt.Sprintf("dem-u-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Uncommon %d", i),
			Number:   fmt.Sprintf("%d", demCommonCount+demBasicCount+i+1),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		uncommonSheet = append(uncommonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 75})
	}

	// Rares: dem-r-000 to dem-r-014.
	rareSheet := make([]domain.SheetCard, 0, demRareCount)
	for i := range demRareCount {
		id := domain.CardID(fmt.Sprintf("dem-r-%03d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Rare %d", i),
			Number:   fmt.Sprintf("%d", demCommonCount+demBasicCount+demUncommonCount+i+1),
			Rarity:   domain.RarityRare,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		rareSheet = append(rareSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 180})
	}

	sortSheetByID(commonSheet)
	sortSheetByID(uncommonSheet)
	sortSheetByID(rareSheet)
	sortSheetByID(foilSheet)

	sheets := map[string]domain.Sheet{
		"common": {
			Name:          "common",
			Cards:         commonSheet,
			TotalWeight:   sumWeights(commonSheet),
			Foil:          false,
			BalanceColors: false, // Ravnica-style: not colour-balanced per MTGJSON
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
		Code:   "DEM",
		Name:   "Demo Set (Ravnica-style)",
		Cards:  cards,
		Sheets: sheets,
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"common": 11, "uncommon": 3, "rare": 1}, Weight: demOptW1},
			{Contents: map[string]int{"common": 10, "uncommon": 3, "rare": 1, "foil": 1}, Weight: demOptW2},
			{Contents: map[string]int{"common": 11, "uncommon": 2, "rare": 1, "foil": 1}, Weight: demOptW3},
			{Contents: map[string]int{"common": 11, "uncommon": 3, "foil": 1}, Weight: demOptW4},
			{Contents: map[string]int{"common": 9, "uncommon": 3, "rare": 1, "foil": 2}, Weight: demOptW5},
		},
	}
}

// newDSPCatalogues builds the demo Time Spiral-style set (DSP) and its
// supplementary timeshifted catalogue (DST).
//
// DSP: 60 commons (12 per colour) + 20 uncommons + 10 rares + 5 basics.
// Common sheet: 65 cards, BalanceColors=true (Time Spiral-style).
// DSP's "timeshifted" sheet references DST card IDs absent from DSP.Cards,
// mirroring the TSP/TSB split in internal/domain/domaintest/timespiral.go.
// DST: 20 timeshifted specials (RaritySpecial), loaded as a card pool.
func newDSPCatalogues() (dsp, dst domain.SetCatalogue) {
	// Mono colours cycling W U B R G for colour-balanced draws.
	demColors := [5]string{"W", "U", "B", "R", "G"}

	dspCards := make(map[domain.CardID]domain.Card)

	// DSP commons: dsp-c-000 to dsp-c-059, colours cycling W,U,B,R,G (12 per colour).
	commonSheet := make([]domain.SheetCard, 0, dspCommonCount+dspBasicCount)
	foilSheet := make([]domain.SheetCard, 0, dspCommonCount+dspUncommonCount+dspRareCount+dstTimeshiftedCount)
	for i := range dspCommonCount {
		id := domain.CardID(fmt.Sprintf("dsp-c-%03d", i))
		dspCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Spiral Common %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityCommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			Colors:   []string{demColors[i%5]},
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
		// Foil weights mirror NewTimeSpiralCatalogues: 50%/25%/12.5%/12.5% by rarity group.
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 8})
	}

	// DSP basics: dsp-b-000 to dsp-b-004 (in common sheet only).
	for i := range dspBasicCount {
		id := domain.CardID(fmt.Sprintf("dsp-b-%03d", i))
		dspCards[id] = domain.Card{
			ID:          id,
			Name:        fmt.Sprintf("Demo Spiral Basic %d", i),
			Number:      fmt.Sprintf("%d", dspCommonCount+i+1),
			Rarity:      domain.RarityCommon,
			Finishes:    []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			IsBasicLand: true,
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
	}

	// DSP uncommons: dsp-u-000 to dsp-u-019.
	uncommonSheet := make([]domain.SheetCard, 0, dspUncommonCount)
	for i := range dspUncommonCount {
		id := domain.CardID(fmt.Sprintf("dsp-u-%03d", i))
		dspCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Spiral Uncommon %d", i),
			Number:   fmt.Sprintf("%d", dspCommonCount+dspBasicCount+i+1),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		uncommonSheet = append(uncommonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 15})
	}

	// DSP rares: dsp-r-000 to dsp-r-009.
	rareSheet := make([]domain.SheetCard, 0, dspRareCount)
	for i := range dspRareCount {
		id := domain.CardID(fmt.Sprintf("dsp-r-%03d", i))
		dspCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Spiral Rare %d", i),
			Number:   fmt.Sprintf("%d", dspCommonCount+dspBasicCount+dspUncommonCount+i+1),
			Rarity:   domain.RarityRare,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		rareSheet = append(rareSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 30})
	}

	// DST timeshifted: dst-s-000 to dst-s-019.
	// Card IDs are added to DST but NOT to DSP.Cards, mirroring the TSP/TSB split.
	timeshiftedSheet := make([]domain.SheetCard, 0, dstTimeshiftedCount)
	dstCards := make(map[domain.CardID]domain.Card)
	dstSpecialSheet := make([]domain.SheetCard, 0, dstTimeshiftedCount)

	for i := range dstTimeshiftedCount {
		id := domain.CardID(fmt.Sprintf("dst-s-%03d", i))
		dstCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Demo Timeshifted %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RaritySpecial,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		timeshiftedSheet = append(timeshiftedSheet, domain.SheetCard{ID: id, Weight: 10})
		dstSpecialSheet = append(dstSpecialSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 3})
	}

	sortSheetByID(commonSheet)
	sortSheetByID(uncommonSheet)
	sortSheetByID(rareSheet)
	sortSheetByID(timeshiftedSheet)
	sortSheetByID(dstSpecialSheet)
	sortSheetByID(foilSheet)

	dsp = domain.SetCatalogue{
		Code:  "DSP",
		Name:  "Demo Spiral Set (Time Spiral-style)",
		Cards: dspCards,
		Sheets: map[string]domain.Sheet{
			"common": {
				Name:          "common",
				Cards:         commonSheet,
				TotalWeight:   sumWeights(commonSheet),
				Foil:          false,
				BalanceColors: true, // Time Spiral-style: colour-balanced per MTGJSON
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
				Weight:   dspOptW1,
			},
			{
				Contents: map[string]int{"common": 9, "uncommon": 3, "rare": 1, "timeshifted": 1, "foil": 1},
				Weight:   dspOptW2,
			},
		},
	}

	dst = domain.SetCatalogue{
		Code:  "DST",
		Name:  "Demo Timeshifted (supplementary)",
		Cards: dstCards,
		Sheets: map[string]domain.Sheet{
			"timeshifted": {
				Name:        "timeshifted",
				Cards:       dstSpecialSheet,
				TotalWeight: sumWeights(dstSpecialSheet),
				Foil:        false,
			},
		},
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"timeshifted": 1}, Weight: 1},
		},
	}

	return dsp, dst
}

// sumWeights returns the total weight of a SheetCard slice.
func sumWeights(cards []domain.SheetCard) int {
	total := 0
	for _, sc := range cards {
		total += sc.Weight
	}
	return total
}

// sortSheetByID sorts a SheetCard slice in ascending card ID order.
func sortSheetByID(s []domain.SheetCard) {
	sort.Slice(s, func(i, j int) bool { return s[i].ID < s[j].ID })
}
