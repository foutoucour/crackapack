package domain_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// premiumSet is a set of n commons on one sheet whose booster draws perBooster of them.
func premiumSet(n, perBooster int) domain.SetCatalogue {
	cat := domain.SetCatalogue{Code: "PRM", Name: "Premium", Cards: map[domain.CardID]domain.Card{}}
	sheet := domain.Sheet{Name: "common", TotalWeight: n}
	for i := range n {
		id := domain.CardID(fmt.Sprintf("PRM-%02d", i))
		cat.Cards[id] = domain.Card{ID: id, Name: string(id), Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}}
		sheet.Cards = append(sheet.Cards, domain.SheetCard{ID: id, Weight: 1})
	}
	cat.Sheets = map[string]domain.Sheet{"common": sheet}
	cat.BoosterOptions = []domain.BoosterOption{{Contents: map[string]int{"common": perBooster}, Weight: 1}}
	return cat
}

func heldCopies(held map[domain.CardID]int) func(domain.CardID) int {
	return func(id domain.CardID) int { return held[id] }
}

func TestLackingCatalogue_boostersLeaveOutPlaysets(t *testing.T) {
	t.Parallel()
	cat := premiumSet(5, 2)
	lacking := domain.LackingCatalogue(cat, heldCopies(map[domain.CardID]int{"PRM-00": 4, "PRM-01": 7, "PRM-02": 3}))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rand.New(rand.NewPCG(1, 1)))

	for range 200 {
		b, err := opener.Open(domain.BoosterLayoutClassic, lacking)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		for _, key := range b.Cards {
			if key.ID == "PRM-00" || key.ID == "PRM-01" {
				t.Fatalf("the booster holds %s, of which the player has a playset: %v", key.ID, b.Cards)
			}
		}
	}
}

func TestLackingCatalogue_aSheetWithTooFewLackingCardsStaysWhole(t *testing.T) {
	t.Parallel()
	cat := premiumSet(3, 2)
	held := map[domain.CardID]int{"PRM-00": 4, "PRM-01": 4}
	lacking := domain.LackingCatalogue(cat, heldCopies(held))

	if got := len(lacking.Sheets["common"].Cards); got != 3 {
		t.Errorf("the sheet holds %d cards, want all 3: only one card is lacking and the booster draws 2", got)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rand.New(rand.NewPCG(2, 2)))
	if _, err := opener.Open(domain.BoosterLayoutClassic, lacking); err != nil {
		t.Errorf("Open: %v", err)
	}
}

func TestLackingCatalogue_leavesTheCatalogueAlone(t *testing.T) {
	t.Parallel()
	cat := premiumSet(5, 2)
	_ = domain.LackingCatalogue(cat, heldCopies(map[domain.CardID]int{"PRM-00": 4}))

	sheet := cat.Sheets["common"]
	if len(sheet.Cards) != 5 || sheet.TotalWeight != 5 {
		t.Errorf("the catalogue's sheet changed: %d cards, weight %d, want 5 and 5", len(sheet.Cards), sheet.TotalWeight)
	}
}

func TestLackingCatalogue_weightFollowsTheCardsKept(t *testing.T) {
	t.Parallel()
	lacking := domain.LackingCatalogue(premiumSet(5, 2), heldCopies(map[domain.CardID]int{"PRM-00": 4, "PRM-01": 4}))

	sheet := lacking.Sheets["common"]
	if len(sheet.Cards) != 3 || sheet.TotalWeight != 3 {
		t.Errorf("the sheet has %d cards and weight %d, want 3 and 3", len(sheet.Cards), sheet.TotalWeight)
	}
}
