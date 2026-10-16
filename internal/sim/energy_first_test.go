package sim_test

// energy_first_test.go covers the energy-first variant of the ConvertingCollectorPolicy, which
// saves the coins for the next top-up before it buys cards.

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// energyFirstCardView returns a view with one missing card that costs 100 coins to buy, a
// booster of 5 energy, a daily reward of 100 and a top-up price of 10 coins per energy.
// The player holds no energy, so the session reaches the card-buying phase.
func energyFirstCardView(balance domain.Coins, topUpEnabled bool) sim.WorldView {
	cardID := domain.CardID("card-a")
	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			cardID: {ID: cardID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	prices := domain.PriceSnapshot{cardID: 84} // 84 cents: 100 coins at the 120% buy price
	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 1},
		},
	}}
	v := makeView(balance, 100, 0, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, nil)
	v.CoinTopupEnabled = topUpEnabled
	v.CoinTopupMinEnergyUnits = 1
	v.CoinTopupRateNow = 10
	return v
}

func TestEnergyFirst_cardPurchaseKeepsTheTopUpReserve(t *testing.T) {
	t.Parallel()
	// The reserve is the top-up of a whole booster: (5 energy - 1 minimum real energy) x 10 coins,
	// on top of the daily reward of 100. Buying the 100-coin card needs a balance of at least
	// 100 + 100 + 40 = 240.
	tests := []struct {
		name         string
		balance      domain.Coins
		topUpEnabled bool
		wantBought   bool
	}{
		{name: "balance only covers the plain floor", balance: 230, topUpEnabled: true, wantBought: false},
		{name: "balance covers the floor and the reserve", balance: 240, topUpEnabled: true, wantBought: true},
		{name: "no reserve when top-up is off", balance: 230, topUpEnabled: false, wantBought: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := &fakeSession{views: []sim.WorldView{energyFirstCardView(tc.balance, tc.topUpEnabled)}}

			sim.NewEnergyFirstPolicy().RunSession(fs)

			if got := len(fs.buyCalls) > 0; got != tc.wantBought {
				t.Errorf("bought a card = %v, want %v (balance %d, top-up on %v)",
					got, tc.wantBought, tc.balance, tc.topUpEnabled)
			}
		})
	}
}

func TestEnergyFirst_stillTopsUpWhenShortOnEnergy(t *testing.T) {
	t.Parallel()
	sp, cats := twoSetProgress()
	costs := map[string]int{"SET-A": 5, "SET-B": 5}
	short := makeView(500, 100, 2, sp, costs, cats, nil, nil)
	short.CoinTopupEnabled = true
	short.CoinTopupMinEnergyUnits = 1
	short.CoinTopupRateNow = 10
	drained := makeView(470, 100, 0, sp, costs, cats, nil, nil)
	drained.CoinTopupEnabled = true
	drained.CoinTopupMinEnergyUnits = 1
	drained.CoinTopupRateNow = 10
	fs := &fakeSession{views: []sim.WorldView{short, drained}}

	sim.NewEnergyFirstPolicy().RunSession(fs)

	if len(fs.openTopUpCalls) == 0 {
		t.Fatal("policy did not call OpenBoosterTopUp when short on energy with enough coins")
	}
}
