// Package domaintest provides deterministic synthetic catalogues and price
// snapshots for use in tests and in the demo dataset. All builders take an
// explicit seed so that results are reproducible.
package domaintest

import (
	"fmt"
	"math/rand/v2"
	"sort"

	"github.com/foutoucour/crackapack/internal/domain"
)

// NewCatalogue returns a deterministic synthetic SetCatalogue seeded with seed.
// The catalogue contains:
//   - 10 commons (alternating nonfoil-only and nonfoil+foil)
//   - 4 uncommons (nonfoil+foil)
//   - 2 rares (nonfoil+foil)
//   - 2 basic lands (common rarity, nonfoil+foil, IsBasicLand=true)
//   - a "colorshifted" sheet mixing 4 uncommons (weight 30) and 2 rares (weight 10)
//   - a "common" sheet (BalanceColors=true) covering commons and basics
//   - two BoosterOptions
//
// Sheet.Cards slices are sorted by ID ascending for reproducible weighted draws.
func NewCatalogue(seed uint64) domain.SetCatalogue {
	rng := rand.New(rand.NewPCG(seed, seed))

	cards := make(map[domain.CardID]domain.Card)

	// commonColors cycles the five Magic colours across the ten commons so that the
	// colour-balanced "common" sheet (BalanceColors=true) can guarantee all five colours.
	// Basic lands remain colourless (empty Colors).
	commonColors := [5]string{"W", "U", "B", "R", "G"}

	// Commons: synth-c-0000 through synth-c-0009.
	commonSheet := make([]domain.SheetCard, 0, 12)
	for i := range 10 {
		id := domain.CardID(fmt.Sprintf("synth-c-%04d", i))
		finishes := []domain.Finish{domain.FinishNonFoil}
		if i%2 == 0 {
			finishes = append(finishes, domain.FinishFoil)
		}
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Common %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityCommon,
			Finishes: finishes,
			Colors:   []string{commonColors[i%5]},
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: rng.IntN(100) + 1})
	}

	// Basic lands: synth-b-0000 through synth-b-0001 (in the common sheet).
	for i := range 2 {
		id := domain.CardID(fmt.Sprintf("synth-b-%04d", i))
		cards[id] = domain.Card{
			ID:          id,
			Name:        fmt.Sprintf("Basic Land %d", i),
			Number:      fmt.Sprintf("%d", 11+i),
			Rarity:      domain.RarityCommon,
			Finishes:    []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			IsBasicLand: true,
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: rng.IntN(100) + 1})
	}
	sort.Slice(commonSheet, func(i, j int) bool { return commonSheet[i].ID < commonSheet[j].ID })

	// Uncommons: synth-u-0000 through synth-u-0003.
	uncommonSheet := make([]domain.SheetCard, 0, 4)
	for i := range 4 {
		id := domain.CardID(fmt.Sprintf("synth-u-%04d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Uncommon %d", i),
			Number:   fmt.Sprintf("%d", 13+i),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		uncommonSheet = append(uncommonSheet, domain.SheetCard{ID: id, Weight: rng.IntN(100) + 1})
	}
	sort.Slice(uncommonSheet, func(i, j int) bool { return uncommonSheet[i].ID < uncommonSheet[j].ID })

	// Rares: synth-r-0000 through synth-r-0001.
	rareSheet := make([]domain.SheetCard, 0, 2)
	for i := range 2 {
		id := domain.CardID(fmt.Sprintf("synth-r-%04d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Rare %d", i),
			Number:   fmt.Sprintf("%d", 17+i),
			Rarity:   domain.RarityRare,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		rareSheet = append(rareSheet, domain.SheetCard{ID: id, Weight: rng.IntN(100) + 1})
	}
	sort.Slice(rareSheet, func(i, j int) bool { return rareSheet[i].ID < rareSheet[j].ID })

	// Colorshifted sheet: 4 uncommons (weight 30) + 2 rares (weight 10).
	// Mirrors Planar Chaos's colorshifted uncommon-or-rare slot (30:10 ratio).
	colorshiftedSheet := make([]domain.SheetCard, 0, 6)
	for i := range 4 {
		id := domain.CardID(fmt.Sprintf("synth-cs-u-%04d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Colorshifted Uncommon %d", i),
			Number:   fmt.Sprintf("%d", 19+i),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil},
		}
		colorshiftedSheet = append(colorshiftedSheet, domain.SheetCard{ID: id, Weight: 30})
	}
	for i := range 2 {
		id := domain.CardID(fmt.Sprintf("synth-cs-r-%04d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Colorshifted Rare %d", i),
			Number:   fmt.Sprintf("%d", 23+i),
			Rarity:   domain.RarityRare,
			Finishes: []domain.Finish{domain.FinishNonFoil},
		}
		colorshiftedSheet = append(colorshiftedSheet, domain.SheetCard{ID: id, Weight: 10})
	}
	sort.Slice(colorshiftedSheet, func(i, j int) bool { return colorshiftedSheet[i].ID < colorshiftedSheet[j].ID })

	sheets := map[string]domain.Sheet{
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
		"colorshifted": {
			Name:        "colorshifted",
			Cards:       colorshiftedSheet,
			TotalWeight: sumWeights(colorshiftedSheet),
		},
	}

	return domain.SetCatalogue{
		Code:   "SYN",
		Name:   "Synthetic Set",
		Cards:  cards,
		Sheets: sheets,
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"common": 10, "uncommon": 3, "rare": 1}, Weight: 4},
			{Contents: map[string]int{"common": 9, "uncommon": 3, "colorshifted": 1, "rare": 1}, Weight: 1},
		},
	}
}

// NewTimeshiftedCatalogue returns a deterministic synthetic SetCatalogue seeded
// with seed where every card has RaritySpecial. It represents the TSB (Time Spiral
// Timeshifted) file used in tests for the timeshifted booster slot.
// Sheet.Cards are sorted by ID ascending.
func NewTimeshiftedCatalogue(seed uint64) domain.SetCatalogue {
	rng := rand.New(rand.NewPCG(seed, seed))

	cards := make(map[domain.CardID]domain.Card)
	specialSheet := make([]domain.SheetCard, 0, 10)

	for i := range 10 {
		id := domain.CardID(fmt.Sprintf("synth-tsb-%04d", i))
		cards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Timeshifted %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RaritySpecial,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		specialSheet = append(specialSheet, domain.SheetCard{ID: id, Weight: rng.IntN(100) + 1})
	}
	sort.Slice(specialSheet, func(i, j int) bool { return specialSheet[i].ID < specialSheet[j].ID })

	return domain.SetCatalogue{
		Code:  "TSB",
		Name:  "Time Spiral Timeshifted",
		Cards: cards,
		Sheets: map[string]domain.Sheet{
			"timeshifted": {
				Name:        "timeshifted",
				Cards:       specialSheet,
				TotalWeight: sumWeights(specialSheet),
				Foil:        false,
			},
		},
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"timeshifted": 1}, Weight: 1},
		},
	}
}

// NewPrices returns a deterministic PriceSnapshot for the cards in cat, seeded with seed.
// The snapshot guarantees:
//   - at least one card priced below 10 USDCents (exercises sub-coin rounding)
//   - at least one card with no entry (exercises the fallback price path)
//   - remaining cards priced between 10 and 1999 USDCents
//
// Card iteration order is deterministic: IDs are sorted before prices are assigned.
func NewPrices(cat domain.SetCatalogue, seed uint64) domain.PriceSnapshot {
	rng := rand.New(rand.NewPCG(seed, seed))

	ids := make([]domain.CardID, 0, len(cat.Cards))
	for id := range cat.Cards {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	if len(ids) == 0 {
		return domain.PriceSnapshot{}
	}

	// Allocate one fewer slot: the last card (by sorted ID) intentionally has no price.
	snap := make(domain.PriceSnapshot, len(ids)-1)
	for i, id := range ids {
		if i == len(ids)-1 {
			// No entry for the last card — exercises the fallback price path.
			continue
		}
		if i == 0 {
			// 5 cents < 10: exercises sub-coin rounding in later stories.
			snap[id] = 5
			continue
		}
		snap[id] = domain.USDCents(rng.IntN(1990) + 10)
	}
	return snap
}

// sumWeights returns the total weight of a SheetCard slice.
func sumWeights(cards []domain.SheetCard) int {
	total := 0
	for _, sc := range cards {
		total += sc.Weight
	}
	return total
}
