package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// fakeSession is a hand-written test double for sim.Session.
// It records actions and returns preconfigured responses.
type fakeSession struct {
	views       []sim.WorldView // views returned in sequence; last one repeated
	viewCalls   int
	openCalls   []string // setCode per OpenBooster call
	openErrs    []error  // errors to return per call (nil = success)
	sellCalls   []int    // keep values per SellDuplicates call
	sellSold    int
	sellRefused int
	buyCalls    []domain.CardKey
	buyErrs     map[domain.CardKey]error
}

func (f *fakeSession) View() sim.WorldView {
	idx := f.viewCalls
	if idx >= len(f.views) {
		idx = len(f.views) - 1
	}
	f.viewCalls++
	return f.views[idx]
}

func (f *fakeSession) OpenBooster(setCode string) error {
	f.openCalls = append(f.openCalls, setCode)
	if len(f.openErrs) > 0 {
		err := f.openErrs[0]
		f.openErrs = f.openErrs[1:]
		return err
	}
	return nil
}

func (f *fakeSession) SellDuplicates(keep int) (sold, refused int) {
	f.sellCalls = append(f.sellCalls, keep)
	return f.sellSold, f.sellRefused
}

func (f *fakeSession) BuyCard(key domain.CardKey) error {
	f.buyCalls = append(f.buyCalls, key)
	if f.buyErrs != nil {
		if err, ok := f.buyErrs[key]; ok {
			return err
		}
	}
	return nil
}

// makeView builds a WorldView for testing with sensible defaults.
func makeView(balance domain.Coins, dailyReward domain.Coins, energyTotal int,
	setProgress map[string]domain.Progress,
	boosterCosts map[string]int,
	cats []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	owned map[domain.CardKey]int,
) sim.WorldView {
	return sim.WorldView{
		Day:            1,
		Balance:        balance,
		DailyReward:    dailyReward,
		EnergyTotal:    energyTotal,
		SetProgress:    setProgress,
		BoosterCosts:   boosterCosts,
		Catalogues:     cats,
		Prices:         prices,
		OwnedCards:     owned,
		CoinRate:       100,
		CounterBuyPct:  120,
		CounterSellPct: 60,
		FallbackPrice:  1,
	}
}

// twoSetProgress returns progress for two sets with different CU ratios.
func twoSetProgress() (map[string]domain.Progress, []domain.SetCatalogue) {
	// SET-A: 2 of 4 CU owned (50%).
	// SET-B: 1 of 4 CU owned (25%) — lower ratio, should be chosen first.
	progA := domain.Progress{
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon:   {Owned: 1, Total: 2},
			domain.RarityUncommon: {Owned: 1, Total: 2},
			domain.RarityRare:     {Owned: 0, Total: 1},
		},
	}
	progB := domain.Progress{
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon:   {Owned: 0, Total: 2},
			domain.RarityUncommon: {Owned: 1, Total: 2},
			domain.RarityRare:     {Owned: 0, Total: 1},
		},
	}
	sp := map[string]domain.Progress{"SET-A": progA, "SET-B": progB}

	catA := domain.SetCatalogue{Code: "SET-A"}
	catB := domain.SetCatalogue{Code: "SET-B"}
	return sp, []domain.SetCatalogue{catA, catB}
}

func TestCollector_targetsLowestCUProgress(t *testing.T) {
	t.Parallel()
	sp, cats := twoSetProgress()

	// SET-B has lower CU ratio (25% vs 50%); expect two opens on SET-B, then energy runs out.
	v0 := makeView(500, 100, 10, sp, map[string]int{"SET-A": 5, "SET-B": 5}, cats, nil, nil)
	// After first open, energy drops to 5 — still enough for one more.
	v1 := makeView(500, 100, 5, sp, map[string]int{"SET-A": 5, "SET-B": 5}, cats, nil, nil)
	// After second open, energy = 0 — loop stops.
	v2 := makeView(500, 100, 0, sp, map[string]int{"SET-A": 5, "SET-B": 5}, cats, nil, nil)

	fs := &fakeSession{
		views: []sim.WorldView{v0, v1, v2},
		// After two opens the energy check fails; openErrs drives the stop via error.
		// Here we let the view's EnergyTotal drive the stop.
	}

	sim.NewCollectorPolicy().RunSession(fs)

	if len(fs.openCalls) < 1 {
		t.Fatal("expected at least one OpenBooster call")
	}
	if fs.openCalls[0] != "SET-B" {
		t.Errorf("first open = %q, want SET-B", fs.openCalls[0])
	}
}

func TestCollector_tieBreakBySetCode(t *testing.T) {
	t.Parallel()
	// Both sets have identical CU ratio (0/4). Alphabetically smaller code wins.
	prog := domain.Progress{
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon:   {Owned: 0, Total: 2},
			domain.RarityUncommon: {Owned: 0, Total: 2},
		},
	}
	sp := map[string]domain.Progress{"ZZZ": prog, "AAA": prog}
	cats := []domain.SetCatalogue{{Code: "ZZZ"}, {Code: "AAA"}}

	v0 := makeView(500, 100, 5, sp, map[string]int{"ZZZ": 5, "AAA": 5}, cats, nil, nil)
	v1 := makeView(500, 100, 0, sp, map[string]int{"ZZZ": 5, "AAA": 5}, cats, nil, nil)

	fs := &fakeSession{views: []sim.WorldView{v0, v1}}
	sim.NewCollectorPolicy().RunSession(fs)

	if len(fs.openCalls) == 0 {
		t.Fatal("expected at least one open")
	}
	if fs.openCalls[0] != "AAA" {
		t.Errorf("tie-break: first open = %q, want AAA", fs.openCalls[0])
	}
}

func TestCollector_sellsDuplicatesAbove4(t *testing.T) {
	t.Parallel()
	// Minimal view: no energy, so phase 1 is skipped.
	sp := map[string]domain.Progress{"X": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 1, Total: 1},
		},
	}}
	cats := []domain.SetCatalogue{{Code: "X"}}
	v := makeView(500, 100, 0, sp, map[string]int{"X": 5}, cats, nil, nil)
	fs := &fakeSession{views: []sim.WorldView{v}, sellSold: 3, sellRefused: 0}

	sim.NewCollectorPolicy().RunSession(fs)

	if len(fs.sellCalls) != 1 {
		t.Fatalf("SellDuplicates called %d times, want 1", len(fs.sellCalls))
	}
	if fs.sellCalls[0] != domain.PlaysetSize {
		t.Errorf("SellDuplicates(keep=%d), want domain.PlaysetSize (%d)", fs.sellCalls[0], domain.PlaysetSize)
	}
}

func TestCollector_buysCheapestMissingFirst(t *testing.T) {
	t.Parallel()
	// Two cards: cheap (200 coins) and expensive (500 coins). Both missing.
	cheapID := domain.CardID("card-cheap")
	expID := domain.CardID("card-expensive")

	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			cheapID: {ID: cheapID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
			expID:   {ID: expID, Rarity: domain.RarityRare, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// cheap: 167 USD cents → floor(167 * 100 * 120 / 10000) = floor(200.4) = 200 coins
	// exp:   417 USD cents → floor(417 * 100 * 120 / 10000) = floor(500.4) = 500 coins
	prices := domain.PriceSnapshot{cheapID: 167, expID: 417}

	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 1},
			domain.RarityRare:   {Owned: 0, Total: 1},
		},
	}}

	v := makeView(1000, 100, 0, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, nil)
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewCollectorPolicy().RunSession(fs)

	if len(fs.buyCalls) < 1 {
		t.Fatal("expected at least one BuyCard call")
	}
	if fs.buyCalls[0].ID != cheapID {
		t.Errorf("first buy = %q, want cheap card %q", fs.buyCalls[0].ID, cheapID)
	}
}

func TestCollector_neverDropsBelowFloor(t *testing.T) {
	t.Parallel()
	// Balance = 150, DailyReward = 100, floor = 100.
	// One card costs 120 coins → after buying balance = 30 < 100 floor → should NOT buy.
	cardID := domain.CardID("card-a")
	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			cardID: {ID: cardID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// 84 USD cents → floor(84 * 100 * 120 / 10000) = floor(100.8) = 100 coins
	prices := domain.PriceSnapshot{cardID: 84}
	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 1},
		},
	}}

	// Balance 150, floor = 100. Buying costs 100. 150 - 100 = 50 < 100. Should skip.
	v := makeView(150, 100, 0, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, nil)
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewCollectorPolicy().RunSession(fs)

	if len(fs.buyCalls) != 0 {
		t.Errorf("expected no BuyCard calls; got %d (would drop below floor)", len(fs.buyCalls))
	}
}

// TestCollector_zeroPricedCardDoesNotBlockBuying reproduces defect 1.
// A catalogue with one 0-priced unowned card and one normally-priced unowned
// card must result in the normally-priced card being bought; the 0-priced card
// must not appear in buy calls at all, and must not cause the loop to stop.
func TestCollector_zeroPricedCardDoesNotBlockBuying(t *testing.T) {
	t.Parallel()
	zeroID := domain.CardID("card-zero")
	normalID := domain.CardID("card-normal")
	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			zeroID:   {ID: zeroID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
			normalID: {ID: normalID, Rarity: domain.RarityRare, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// zeroID price 0 → buyCoins 0 (zero price returns (0,nil) from PriceToCoins).
	// normalID price 167 cents → floor(167*100*120/10000) = 200 coins.
	prices := domain.PriceSnapshot{zeroID: 0, normalID: 167}
	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 1},
			domain.RarityRare:   {Owned: 0, Total: 1},
		},
	}}
	v := makeView(1000, 100, 0, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, nil)
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewCollectorPolicy().RunSession(fs)

	normalKey := domain.CardKey{ID: normalID, Finish: domain.FinishNonFoil}
	zeroKey := domain.CardKey{ID: zeroID, Finish: domain.FinishNonFoil}

	bought := make(map[domain.CardKey]bool)
	for _, k := range fs.buyCalls {
		bought[k] = true
	}
	if bought[zeroKey] {
		t.Error("zero-priced card should never be attempted for purchase (buildBuyCandidates must skip it)")
	}
	if !bought[normalKey] {
		t.Errorf("normal-priced card should be bought after zero-priced card is skipped; buyCalls = %v", fs.buyCalls)
	}
}

func TestCollector_skipsOutOfStockCards(t *testing.T) {
	t.Parallel()
	cardA := domain.CardID("card-a")
	cardB := domain.CardID("card-b")

	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			cardA: {ID: cardA, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
			cardB: {ID: cardB, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// Both priced the same to ensure cardA (alphabetically first) is tried first.
	prices := domain.PriceSnapshot{cardA: 100, cardB: 100}
	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 2},
		},
	}}

	keyA := domain.CardKey{ID: cardA, Finish: domain.FinishNonFoil}
	keyB := domain.CardKey{ID: cardB, Finish: domain.FinishNonFoil}

	v := makeView(2000, 100, 0, sp, map[string]int{"SET": 5}, []domain.SetCatalogue{cat}, prices, nil)
	fs := &fakeSession{
		views: []sim.WorldView{v},
		buyErrs: map[domain.CardKey]error{
			keyA: domain.ErrOutOfStock,
		},
	}

	sim.NewCollectorPolicy().RunSession(fs)

	// cardA should be skipped; cardB should be bought.
	bought := make(map[domain.CardKey]bool)
	for _, k := range fs.buyCalls {
		bought[k] = true
	}
	if !bought[keyB] {
		t.Error("expected cardB to be bought after cardA was out of stock")
	}
}
