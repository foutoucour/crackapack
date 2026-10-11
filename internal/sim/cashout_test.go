package sim_test

import (
	"bytes"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// TestCashOut_sellsAbove1Copy verifies that CashOutPolicy calls SellDuplicates
// with keep=1 (cashOutKeep), not the collector's PlaysetSize.
func TestCashOut_sellsAbove1Copy(t *testing.T) {
	t.Parallel()
	sp := map[string]domain.Progress{"X": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 1, Total: 1},
		},
	}}
	cats := []domain.SetCatalogue{{Code: "X"}}
	v := makeView(500, 100, 0, sp, map[string]int{"X": 5}, cats, nil, nil)
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewCashOutPolicy().RunSession(fs)

	if len(fs.sellCalls) != 1 {
		t.Fatalf("SellDuplicates called %d times, want 1", len(fs.sellCalls))
	}
	if fs.sellCalls[0] != 1 {
		t.Errorf("SellDuplicates(keep=%d), want 1", fs.sellCalls[0])
	}
}

// TestCashOut_neverCallsBuyCard verifies that CashOutPolicy never buys cards.
func TestCashOut_neverCallsBuyCard(t *testing.T) {
	t.Parallel()
	// A card the collector would buy: cheap, non-foil, not yet owned.
	id := domain.CardID("card-a")
	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			id: {ID: id, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	prices := domain.PriceSnapshot{id: 100}
	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 1},
		},
	}}
	// Give plenty of balance so the collector would buy.
	v := makeView(5000, 100, 0, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, nil)
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewCashOutPolicy().RunSession(fs)

	if len(fs.buyCalls) != 0 {
		t.Errorf("expected no BuyCard calls; got %d (cash-out never buys)", len(fs.buyCalls))
	}
}

// TestCashOut_opensUntilEnergyExhausted verifies that CashOutPolicy opens
// boosters until energy is insufficient, re-reading the view after each open.
func TestCashOut_opensUntilEnergyExhausted(t *testing.T) {
	t.Parallel()
	sp, cats := twoSetProgress()

	cost := 5
	boosterCosts := map[string]int{"SET-A": cost, "SET-B": cost}

	// Energy drops by cost after each open via successive view updates.
	v0 := makeView(500, 100, 10, sp, boosterCosts, cats, nil, nil) // 2 opens possible
	v1 := makeView(500, 100, 5, sp, boosterCosts, cats, nil, nil)  // 1 open possible
	v2 := makeView(500, 100, 0, sp, boosterCosts, cats, nil, nil)  // loop stops

	fs := &fakeSession{views: []sim.WorldView{v0, v1, v2}}

	sim.NewCashOutPolicy().RunSession(fs)

	if len(fs.openCalls) != 2 {
		t.Errorf("expected 2 OpenBooster calls, got %d", len(fs.openCalls))
	}
}

// TestCashOut_targetsLowestCUProgress verifies that CashOutPolicy picks the
// set with the lowest commons+uncommons ratio, identical to CollectorPolicy.
func TestCashOut_targetsLowestCUProgress(t *testing.T) {
	t.Parallel()
	sp, cats := twoSetProgress()
	// SET-B has 25% CU ratio (lower); SET-A has 50%. Expect SET-B first.

	v0 := makeView(500, 100, 5, sp, map[string]int{"SET-A": 5, "SET-B": 5}, cats, nil, nil)
	v1 := makeView(500, 100, 0, sp, map[string]int{"SET-A": 5, "SET-B": 5}, cats, nil, nil)

	fs := &fakeSession{views: []sim.WorldView{v0, v1}}
	sim.NewCashOutPolicy().RunSession(fs)

	if len(fs.openCalls) == 0 {
		t.Fatal("expected at least one OpenBooster call")
	}
	if fs.openCalls[0] != "SET-B" {
		t.Errorf("first open = %q, want SET-B", fs.openCalls[0])
	}
}

// TestCashOut_engine_integration runs the cash-out policy for a short real run
// and asserts:
//   - no cards are ever bought (CardsBought == 0 every day)
//   - the counter_purchase account balance is 0 at the end (no coins flowed
//     from player to Counter for purchases)
//   - at least some cards are sold across the run (CardsSold > 0 total)
//   - the ledger invariant holds (all account balances sum to zero)
//   - two runs with the same seed produce byte-identical JSON
func TestCashOut_engine_integration(t *testing.T) {
	t.Parallel()

	run := func() sim.Result {
		t.Helper()
		e := buildEngine(t, smallConfig(), sim.NewCashOutPolicy())
		r, err := e.Run()
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return r
	}

	r1 := run()
	r2 := run()

	// Byte-identical output across two runs.
	var buf1, buf2 bytes.Buffer
	if err := sim.WriteJSON(&buf1, r1); err != nil {
		t.Fatalf("WriteJSON 1: %v", err)
	}
	if err := sim.WriteJSON(&buf2, r2); err != nil {
		t.Fatalf("WriteJSON 2: %v", err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("cash-out runs with the same seed produced different JSON")
	}

	if len(r1.Days) == 0 {
		t.Fatal("no days in result")
	}

	var totalSold, totalBought int
	for _, d := range r1.Days {
		totalSold += d.CardsSold
		totalBought += d.CardsBought
	}

	// Cash-out never buys.
	if totalBought != 0 {
		t.Errorf("total CardsBought = %d, want 0 (cash-out never buys)", totalBought)
	}

	// Counter purchase account must stay at 0 (no player→counter coin flows).
	last := r1.Days[len(r1.Days)-1]
	if bal := last.MoneySupply[domain.AccountCounterPurchase]; bal != 0 {
		t.Errorf("counter_purchase balance = %d, want 0", bal)
	}

	// Cards should be sold across the run (duplicates accumulate).
	if totalSold == 0 {
		t.Error("total CardsSold = 0; expected some sales from duplicate booster cards")
	}

	// Ledger invariant: all balances sum to zero.
	var sum domain.Coins
	for _, bal := range last.MoneySupply {
		sum += bal
	}
	if sum != 0 {
		t.Errorf("ledger sum = %d, want 0; balances: %v", sum, last.MoneySupply)
	}
}
