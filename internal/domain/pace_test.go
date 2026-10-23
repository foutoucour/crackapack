package domain_test

import (
	"fmt"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// paceSet is a set of n commons on one sheet of equal weights and a booster of perBooster of them,
// plus a basic land on the sheet, which a binder never counts.
func paceSet(code string, n, perBooster int) domain.SetCatalogue {
	cat := domain.SetCatalogue{Code: code, Cards: make(map[domain.CardID]domain.Card)}
	sheet := domain.Sheet{Name: "common"}
	for i := range n {
		id := domain.CardID(fmt.Sprintf("%s-%03d", code, i))
		cat.Cards[id] = domain.Card{ID: id, Rarity: domain.RarityCommon}
		sheet.Cards = append(sheet.Cards, domain.SheetCard{ID: id, Weight: 1})
	}
	land := domain.CardID(code + "-land")
	cat.Cards[land] = domain.Card{ID: land, Rarity: domain.RarityCommon, IsBasicLand: true}
	sheet.Cards = append(sheet.Cards, domain.SheetCard{ID: land, Weight: 1})
	sheet.TotalWeight = len(sheet.Cards)
	cat.Sheets = map[string]domain.Sheet{"common": sheet}
	cat.BoosterOptions = []domain.BoosterOption{{Contents: map[string]int{"common": perBooster}, Weight: 1}}
	return cat
}

func TestBoostersToShare(t *testing.T) {
	t.Parallel()

	// A card on a second sheet that one option in ten draws: the binder needs it to be full.
	withRareSlot := paceSet("RAR", 9, 5)
	rare := domain.CardID("RAR-rare")
	withRareSlot.Cards[rare] = domain.Card{ID: rare, Rarity: domain.RarityRare}
	withRareSlot.Sheets["rare"] = domain.Sheet{Name: "rare", Cards: []domain.SheetCard{{ID: rare, Weight: 1}}, TotalWeight: 1}
	withRareSlot.BoosterOptions = []domain.BoosterOption{
		{Contents: map[string]int{"common": 5}, Weight: 9},
		{Contents: map[string]int{"common": 5, "rare": 1}, Weight: 1},
	}

	// The same rare in one booster in ten million.
	outOfReach := paceSet("RAR", 9, 5)
	outOfReach.Cards[rare] = withRareSlot.Cards[rare]
	outOfReach.Sheets["rare"] = withRareSlot.Sheets["rare"]
	outOfReach.BoosterOptions = []domain.BoosterOption{
		{Contents: map[string]int{"common": 5}, Weight: 9_999_999},
		{Contents: map[string]int{"common": 5, "rare": 1}, Weight: 1},
	}

	tests := []struct {
		name  string
		cat   domain.SetCatalogue
		share float64
		want  int
	}{
		// 10 binder cards and a land on the sheet: a card is in a booster with 1-(10/11)^5, so 90% of
		// the binder is expected after 5 boosters and not after 4.
		{name: "small set", cat: paceSet("SML", 10, 5), share: 0.9, want: 5},
		{name: "large set takes longer", cat: paceSet("LRG", 40, 5), share: 0.9, want: 19},
		{name: "a lower share is reached sooner", cat: paceSet("SML", 10, 5), share: 0.5, want: 2},
		// The rare is in one booster in ten, so it slows the binder down: 6 boosters, not the 5 of the
		// commons alone.
		{name: "a rare slot slows the binder down", cat: withRareSlot, share: 0.9, want: 6},
		// A whole binder is reached once less than half a card is expected to be missing.
		{name: "the whole binder waits for the rare", cat: withRareSlot, share: 1, want: 9},
		{name: "a binder out of reach stops at the bound", cat: outOfReach, share: 1, want: domain.MaxBoostersToShare},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.BoostersToShare(tc.cat, tc.share)
			if err != nil {
				t.Fatalf("BoostersToShare: %v", err)
			}
			if got != tc.want {
				t.Errorf("BoostersToShare(%s, %v) = %d, want %d", tc.cat.Code, tc.share, got, tc.want)
			}
		})
	}
}

func TestBoostersToShare_isTheSameOnEveryCall(t *testing.T) {
	t.Parallel()
	cat := paceSet("DET", 97, 7)
	first, err := domain.BoostersToShare(cat, 0.9)
	if err != nil {
		t.Fatalf("BoostersToShare: %v", err)
	}
	for range 20 {
		if got, _ := domain.BoostersToShare(cat, 0.9); got != first {
			t.Fatalf("BoostersToShare = %d, then %d on the same set", first, got)
		}
	}
}

func TestBoostersToShare_rejects(t *testing.T) {
	t.Parallel()
	noBooster := paceSet("NOB", 5, 1)
	noBooster.BoosterOptions = nil
	tests := []struct {
		name  string
		cat   domain.SetCatalogue
		share float64
	}{
		{name: "a share of zero", cat: paceSet("ZER", 5, 1), share: 0},
		{name: "a share above one", cat: paceSet("ONE", 5, 1), share: 1.01},
		{name: "a set without a booster", cat: noBooster, share: 0.9},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.BoostersToShare(tc.cat, tc.share); err == nil {
				t.Errorf("BoostersToShare(%s, %v) = nil error, want one", tc.cat.Code, tc.share)
			}
		})
	}
}
