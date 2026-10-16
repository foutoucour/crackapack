package domain_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func idsOf(cards []domain.Card) []domain.CardID {
	ids := make([]domain.CardID, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	return ids
}

func TestDrawableCards_areThePoolCardsTheSheetsOfTheSetReference(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()

	got := domain.DrawableCards(tsp, tsb)

	if len(got) != len(tsb.Cards) || len(got) == 0 {
		t.Fatalf("%d drawable cards, want the %d of the pool that the sheets reference", len(got), len(tsb.Cards))
	}
	ids := idsOf(got)
	if !slices.IsSorted(ids) {
		t.Errorf("ids are not sorted, so the result depends on map order: %v", ids)
	}
	for _, c := range got {
		if _, inPool := tsb.Cards[c.ID]; !inPool {
			t.Errorf("card %s is not a card of the pool", c.ID)
		}
	}
}

func TestDrawableCards_leavesOutWhatNoSheetReferences(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	tsb.Cards["tsp-tsb-unreachable"] = domain.Card{ID: "tsp-tsb-unreachable", Name: "Unreachable", Rarity: domain.RaritySpecial}

	got := domain.DrawableCards(tsp, tsb)

	if slices.Contains(idsOf(got), "tsp-tsb-unreachable") {
		t.Error("a pool card that no booster can draw is drawable")
	}
	if len(got) != len(tsb.Cards)-1 {
		t.Errorf("%d drawable cards, want %d", len(got), len(tsb.Cards)-1)
	}
}

func TestDrawableCards_leavesOutWhatTheSetHoldsItself(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	// The same card in both lists is the set's card, and must not be counted twice.
	shared := domain.CardID("tsp-tsb-000")
	tsp.Cards[shared] = tsb.Cards[shared]

	got := domain.DrawableCards(tsp, tsb)

	if slices.Contains(idsOf(got), shared) {
		t.Error("a card the set holds itself is also reported as a pool card")
	}
}

func TestDrawableCards_isEmptyForASetWhoseBoostersDrawOnlyItsOwnCards(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	_, tsb := domaintest.NewTimeSpiralCatalogues()

	if got := domain.DrawableCards(rav, tsb); len(got) != 0 {
		t.Errorf("%d drawable cards of an unrelated pool, want 0", len(got))
	}
}

func TestWithPools_addsTheDrawableCardsToTheBinderAndCountsThemInItsProgress(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	own := len(tsp.Cards)

	merged := domain.WithPools(tsp, []domain.SetCatalogue{tsb})

	if want := own + len(domain.DrawableCards(tsp, tsb)); len(merged.Cards) != want {
		t.Errorf("merged catalogue holds %d cards, want %d", len(merged.Cards), want)
	}
	if len(tsp.Cards) != own {
		t.Errorf("the set's own card map grew from %d to %d: WithPools must not write into it", own, len(tsp.Cards))
	}
	if merged.Code != tsp.Code || len(merged.Sheets) != len(tsp.Sheets) {
		t.Errorf("the merged catalogue is not the set's: code %q, %d sheets", merged.Code, len(merged.Sheets))
	}

	collection := domain.NewCollection()
	plain := collection.BinderProgress(tsp)
	counted := collection.BinderProgress(merged)
	if counted.Total != plain.Total+len(tsb.Cards) {
		t.Errorf("progress total = %d, want %d plus the %d timeshifted cards", counted.Total, plain.Total, len(tsb.Cards))
	}
	if got := counted.ByRarity[domain.RaritySpecial].Total; got != len(tsb.Cards) {
		t.Errorf("special cards in the progress = %d, want %d", got, len(tsb.Cards))
	}
}

func TestWithPools_withNoPoolIsTheSetItself(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()

	merged := domain.WithPools(rav, nil)

	if len(merged.Cards) != len(rav.Cards) {
		t.Errorf("merged catalogue holds %d cards, want the set's %d", len(merged.Cards), len(rav.Cards))
	}
}
