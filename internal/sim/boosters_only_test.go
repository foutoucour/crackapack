package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// TestBoostersOnly_opensBoostersAndNeverTouchesTheCounter pins the baseline of the market study: the
// player opens what its energy allows and neither sells to nor buys from the Counter.
func TestBoostersOnly_opensBoostersAndNeverTouchesTheCounter(t *testing.T) {
	t.Parallel()
	id := domain.CardID("card-a")
	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			id: {ID: id, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 1},
		},
	}}
	enough := makeView(5000, 100, 10, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, domain.PriceSnapshot{id: 100}, nil)
	spent := makeView(5000, 100, 0, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, domain.PriceSnapshot{id: 100}, nil)
	fs := &fakeSession{views: []sim.WorldView{enough, spent}}

	sim.NewBoostersOnlyPolicy().RunSession(fs)

	if len(fs.openCalls) != 1 {
		t.Errorf("OpenBooster called %d times, want 1", len(fs.openCalls))
	}
	if len(fs.sellCalls) != 0 {
		t.Errorf("SellDuplicates called %d times, want 0", len(fs.sellCalls))
	}
	if len(fs.buyCalls) != 0 {
		t.Errorf("BuyCard called %d times, want 0", len(fs.buyCalls))
	}
}
