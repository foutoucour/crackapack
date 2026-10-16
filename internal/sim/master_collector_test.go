package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// foilCatalogue returns a one-set catalogue of cards that exist in both finishes, plus the price
// snapshot and the progress map a view needs. Every card is already owned in its non-foil, so the
// collector has nothing left to buy and only the foils are missing.
func foilCatalogue(centsByCard map[domain.CardID]domain.USDCents) (domain.SetCatalogue, domain.PriceSnapshot, map[string]domain.Progress, map[domain.CardKey]int) {
	cards := make(map[domain.CardID]domain.Card, len(centsByCard))
	owned := make(map[domain.CardKey]int, len(centsByCard))
	for id := range centsByCard {
		cards[id] = domain.Card{ID: id, Rarity: domain.RarityRare, Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil}}
		owned[domain.CardKey{ID: id, Finish: domain.FinishNonFoil}] = 1
	}
	progress := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{domain.RarityRare: {Owned: len(cards), Total: len(cards)}},
	}}
	return domain.SetCatalogue{Code: "SET", Cards: cards}, domain.PriceSnapshot(centsByCard), progress, owned
}

func foilKey(id domain.CardID) domain.CardKey {
	return domain.CardKey{ID: id, Finish: domain.FinishFoil}
}

func TestMasterCollector_buysMissingFoilsCheapestFirst(t *testing.T) {
	t.Parallel()
	cat, prices, progress, owned := foilCatalogue(map[domain.CardID]domain.USDCents{"cheap": 100, "dear": 500})
	v := makeView(10000, 100, 0, progress, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, owned)
	v.CounterFoilPct = 100
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewMasterCollectorPolicy().RunSession(fs)

	if len(fs.buyCalls) != 2 || fs.buyCalls[0] != foilKey("cheap") || fs.buyCalls[1] != foilKey("dear") {
		t.Errorf("bought %v, want the foils of cheap then dear", fs.buyCalls)
	}
}

func TestMasterCollector_skipsFoilsAlreadyOwned(t *testing.T) {
	t.Parallel()
	cat, prices, progress, owned := foilCatalogue(map[domain.CardID]domain.USDCents{"have": 100, "lack": 100})
	owned[foilKey("have")] = 1
	v := makeView(10000, 100, 0, progress, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, owned)
	v.CounterFoilPct = 100
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewMasterCollectorPolicy().RunSession(fs)

	if len(fs.buyCalls) != 1 || fs.buyCalls[0] != foilKey("lack") {
		t.Errorf("bought %v, want only the missing foil", fs.buyCalls)
	}
}

func TestMasterCollector_pricesAFoilAtTheFoilPercentage(t *testing.T) {
	t.Parallel()
	// A $5.00 card sells at 120% of 500 cents × 100 coins = 600 coins; the foil at 300% costs 1,800.
	// A balance of 1,800 less the floor of 100 cannot pay it, so nothing is bought.
	cat, prices, progress, owned := foilCatalogue(map[domain.CardID]domain.USDCents{"dear": 500})
	v := makeView(1800, 100, 0, progress, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, owned)
	v.CounterFoilPct = 300
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewMasterCollectorPolicy().RunSession(fs)

	if len(fs.buyCalls) != 0 {
		t.Errorf("bought %v, want nothing: the foil costs more than the balance above the floor", fs.buyCalls)
	}
}

func TestMasterCollector_skipsAnOutOfStockFoil(t *testing.T) {
	t.Parallel()
	cat, prices, progress, owned := foilCatalogue(map[domain.CardID]domain.USDCents{"gone": 100, "here": 200})
	v := makeView(10000, 100, 0, progress, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, owned)
	v.CounterFoilPct = 100
	fs := &fakeSession{
		views:   []sim.WorldView{v},
		buyErrs: map[domain.CardKey]error{foilKey("gone"): domain.ErrOutOfStock},
	}

	sim.NewMasterCollectorPolicy().RunSession(fs)

	if last := fs.buyCalls[len(fs.buyCalls)-1]; last != foilKey("here") {
		t.Errorf("bought %v, want the next foil after the out-of-stock one", fs.buyCalls)
	}
}
