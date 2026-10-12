package domaintest

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// NewPlanarChaosCatalogue returns a deterministic synthetic SetCatalogue that
// reproduces the Planar Chaos booster composition and foil rules as documented
// in docs/core-features.md.
//
// Card inventory (all in plc.Cards):
//   - 8  regular commons          (plc-c-000 … plc-c-007), colourless
//   - 3  colorshifted commons     (plc-cs-c-000 … plc-cs-c-002), colourless
//   - 4  regular uncommons        (plc-u-000 … plc-u-003), colourless
//   - 15 colorshifted uncommons   (plc-cs-u-000 … plc-cs-u-014), colourless
//   - 1  regular rare             (plc-r-000), colourless
//   - 10 colorshifted rares       (plc-cs-r-000 … plc-cs-r-009), colourless
//
// Sheets (BalanceColors=false on all — Planar Chaos is not listed in the colour-
// balance paragraph of docs/core-features.md):
//
//   - "common"              8 regular commons, Foil=false, equal weight 10
//
//   - "colorshifted-common" 3 colorshifted commons, Foil=false, equal weight 10
//
//   - "uncommon"            4 regular uncommons, Foil=false, equal weight 10
//
//   - "rare"                1 regular rare, weight 10
//
//   - "colorshifted-slot"   15 CS uncommons (weight 2) + 10 CS rares (weight 1)
//     Total weight 40; CS rare fraction = 10/40 = 25%
//     (docs: "weighted 30 to 10" means aggregate 30:10,
//     one card in four is rare)
//
//   - "foil"                ALL 41 cards, Foil=true.
//     "Whole set" includes colorshifted cards grouped by
//     their printed rarity: CS commons → common group,
//     CS uncommons → uncommon group, CS rares → rare group.
//
//     Rarity groups:
//     common   = 8 regular + 3 CS = 11 cards
//     uncommon = 4 regular + 15 CS = 19 cards
//     rare     = 1 regular + 10 CS = 11 cards
//
//     Need integer per-card weights with groups in ratio 60:25:15.
//     Let 11·w_c : 19·w_u : 11·w_r = 12 : 5 : 3 (= 60:25:15 scaled).
//     Solving for integers:
//     11·w_c = 12k  → gcd(11,12)=1, so 11|k
//     19·w_u =  5k  → gcd(19, 5)=1, so 19|k
//     Minimum k = lcm(11,19) = 209
//     w_c = 12·209/11 = 228
//     w_u =  5·209/19 =  55
//     w_r =  3·209/11 =  57
//     Total = 11·228 + 19·55 + 11·57
//     = 2508   + 1045  +  627 = 4180
//     Verification:
//     common   2508/4180 = 60% ✓
//     uncommon 1045/4180 = 25% ✓
//     rare      627/4180 = 15% ✓
//
// BoosterOptions (weight sum 1000, both produce 15 cards):
//
//	Option 1 (775): 8 common + 3 colorshifted-common + 2 uncommon + 1 rare + 1 colorshifted-slot
//	Option 2 (225): 7 common + 3 colorshifted-common + 2 uncommon + 1 rare + 1 colorshifted-slot + 1 foil
func NewPlanarChaosCatalogue() domain.SetCatalogue {
	plcCards := make(map[domain.CardID]domain.Card)

	// foilSheet collects all 41 entries with rarity-group weights (see doc above).
	foilSheet := make([]domain.SheetCard, 0, 41)

	// ---- regular commons (8) ----
	commonSheet := make([]domain.SheetCard, 0, 8)
	for i := range 8 {
		id := domain.CardID(fmt.Sprintf("plc-c-%03d", i))
		plcCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Planar Chaos Common %d", i),
			Number:   fmt.Sprintf("%d", i+1),
			Rarity:   domain.RarityCommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		commonSheet = append(commonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 228})
	}

	// ---- colorshifted commons (3) ----
	csCommonSheet := make([]domain.SheetCard, 0, 3)
	for i := range 3 {
		id := domain.CardID(fmt.Sprintf("plc-cs-c-%03d", i))
		plcCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Planar Chaos Colorshifted Common %d", i),
			Number:   fmt.Sprintf("%d", 9+i),
			Rarity:   domain.RarityCommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		csCommonSheet = append(csCommonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 228})
	}

	// ---- regular uncommons (4) ----
	uncommonSheet := make([]domain.SheetCard, 0, 4)
	for i := range 4 {
		id := domain.CardID(fmt.Sprintf("plc-u-%03d", i))
		plcCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Planar Chaos Uncommon %d", i),
			Number:   fmt.Sprintf("%d", 12+i),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		uncommonSheet = append(uncommonSheet, domain.SheetCard{ID: id, Weight: 10})
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 55})
	}

	// ---- colorshifted uncommons (15) — shared with "colorshifted-slot" sheet ----
	csSlotSheet := make([]domain.SheetCard, 0, 25) // 15 CS uncommons + 10 CS rares
	for i := range 15 {
		id := domain.CardID(fmt.Sprintf("plc-cs-u-%03d", i))
		plcCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Planar Chaos Colorshifted Uncommon %d", i),
			Number:   fmt.Sprintf("%d", 16+i),
			Rarity:   domain.RarityUncommon,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		// colorshifted-slot: weight 2 per uncommon (aggregate 30 for uncommons)
		csSlotSheet = append(csSlotSheet, domain.SheetCard{ID: id, Weight: 2})
		// foil sheet: CS uncommons count as uncommon group (w_u = 55)
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 55})
	}

	// ---- regular rare (1) ----
	rareID := domain.CardID("plc-r-000")
	plcCards[rareID] = domain.Card{
		ID:       rareID,
		Name:     "Planar Chaos Rare",
		Number:   "31",
		Rarity:   domain.RarityRare,
		Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
	}
	rareSheet := []domain.SheetCard{{ID: rareID, Weight: 10}}
	foilSheet = append(foilSheet, domain.SheetCard{ID: rareID, Weight: 57})

	// ---- colorshifted rares (10) — shared with "colorshifted-slot" sheet ----
	for i := range 10 {
		id := domain.CardID(fmt.Sprintf("plc-cs-r-%03d", i))
		plcCards[id] = domain.Card{
			ID:       id,
			Name:     fmt.Sprintf("Planar Chaos Colorshifted Rare %d", i),
			Number:   fmt.Sprintf("%d", 32+i),
			Rarity:   domain.RarityRare,
			Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
		}
		// colorshifted-slot: weight 1 per rare (aggregate 10 for rares)
		csSlotSheet = append(csSlotSheet, domain.SheetCard{ID: id, Weight: 1})
		// foil sheet: CS rares count as rare group (w_r = 57)
		foilSheet = append(foilSheet, domain.SheetCard{ID: id, Weight: 57})
	}

	sortSheetByID(commonSheet)
	sortSheetByID(csCommonSheet)
	sortSheetByID(uncommonSheet)
	sortSheetByID(csSlotSheet)
	sortSheetByID(foilSheet)

	return domain.SetCatalogue{
		Code:  "PLC",
		Name:  "Planar Chaos (synthetic)",
		Cards: plcCards,
		Sheets: map[string]domain.Sheet{
			"common": {
				Name:        "common",
				Cards:       commonSheet,
				TotalWeight: sumWeights(commonSheet),
				Foil:        false,
			},
			"colorshifted-common": {
				Name:        "colorshifted-common",
				Cards:       csCommonSheet,
				TotalWeight: sumWeights(csCommonSheet),
				Foil:        false,
			},
			"uncommon": {
				Name:        "uncommon",
				Cards:       uncommonSheet,
				TotalWeight: sumWeights(uncommonSheet),
				Foil:        false,
			},
			"rare": {
				Name:        "rare",
				Cards:       rareSheet,
				TotalWeight: sumWeights(rareSheet),
				Foil:        false,
			},
			"colorshifted-slot": {
				Name:        "colorshifted-slot",
				Cards:       csSlotSheet,
				TotalWeight: sumWeights(csSlotSheet),
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
				Contents: map[string]int{
					"common": 8, "colorshifted-common": 3,
					"uncommon": 2, "rare": 1, "colorshifted-slot": 1,
				},
				Weight: 775,
			},
			{
				Contents: map[string]int{
					"common": 7, "colorshifted-common": 3,
					"uncommon": 2, "rare": 1, "colorshifted-slot": 1, "foil": 1,
				},
				Weight: 225,
			},
		},
	}
}
