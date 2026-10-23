package sim_test

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

// smallConfig returns a minimal valid config suitable for fast tests.
func smallConfig() config.Config {
	return config.Config{
		CoinRate:         100,
		CounterSellPct:   60,
		CounterBuyPct:    120,
		WelcomeBonus:     200,
		DailyReward:      100,
		FallbackPrice:    1,
		CounterShelfDays: -1,
		SessionsPerDay:   3,
		RunDays:          5,
		Seed:             42,
		InitialStock: map[domain.Rarity]int{
			domain.RarityCommon:   20,
			domain.RarityUncommon: 8,
			domain.RarityRare:     2,
			domain.RaritySpecial:  2,
		},
		Energy: config.EnergyConfig{
			RechargePerTick:      1,
			ReserveCap:           10,
			SavingsIntervalTicks: 3,
			SavingsCap:           200,
			CostTable: []config.EnergyCost{
				{Threshold: 50, Cost: 3},
				{Threshold: 100, Cost: 5},
				{Threshold: 200, Cost: 8},
				{Threshold: 400, Cost: 12},
			},
		},
	}
}

// buildEngine creates an Engine with the synthetic RAV and CLS catalogues.
func buildEngine(t *testing.T, cfg config.Config, policy sim.PlayerPolicy) *sim.Engine {
	t.Helper()
	cats := []domain.SetCatalogue{
		domaintest.NewRavnicaCatalogue(),
		domaintest.NewClassicCatalogue(),
	}
	prices := mergedPrices(cats, cfg.Seed)
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	e, err := sim.NewEngine(cfg, cats, nil, prices, policy, rng)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

// mergedPrices generates prices for multiple catalogues without collision.
func mergedPrices(cats []domain.SetCatalogue, seed uint64) domain.PriceSnapshot {
	merged := make(domain.PriceSnapshot)
	for i, cat := range cats {
		snap := domaintest.NewPrices(cat, seed+uint64(i))
		for id, price := range snap {
			merged[id] = price
		}
	}
	return merged
}

func TestRun_ledgerInvariant(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Days) != cfg.RunDays {
		t.Fatalf("days = %d, want %d", len(result.Days), cfg.RunDays)
	}
	// Ledger sum invariant: all balances sum to zero after every day.
	// We verify on the last day's MoneySupply, which includes all account balances.
	last := result.Days[len(result.Days)-1]
	var sum domain.Coins
	for _, bal := range last.MoneySupply {
		sum += bal
	}
	if sum != 0 {
		t.Errorf("ledger sum = %d, want 0; balances: %v", sum, last.MoneySupply)
	}
}

func TestRun_playerBalanceNeverNegative(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, d := range result.Days {
		bal := d.MoneySupply[domain.AccountPlayer]
		if bal < 0 {
			t.Errorf("day %d: player balance = %d, want >= 0", d.Day, bal)
		}
	}
}

func TestRun_correctDayCount(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.RunDays = 7
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Days) != 7 {
		t.Errorf("days = %d, want 7", len(result.Days))
	}
}

func TestRun_dayNumbering(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.RunDays = 3
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for i, d := range result.Days {
		want := i + 1 // 1-based
		if d.Day != want {
			t.Errorf("result.Days[%d].Day = %d, want %d", i, d.Day, want)
		}
	}
}

func TestRun_welcomeAndDailyReward(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.RunDays = 1
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// After day 1 the player received WelcomeBonus + DailyReward minus any spending.
	// At minimum the balance should be >= 0 (enforced by the ledger).
	d := result.Days[0]
	// The welcome bonus account should show a debit (negative balance).
	wbBal := d.MoneySupply[domain.AccountWelcomeBonus]
	if wbBal > 0 {
		t.Errorf("welcome_bonus balance = %d; expect negative (debited)", wbBal)
	}
	drBal := d.MoneySupply[domain.AccountDailyReward]
	if drBal > 0 {
		t.Errorf("daily_reward balance = %d; expect negative (debited)", drBal)
	}
}

func TestRun_deterministicOutput(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()

	var buf1, buf2 bytes.Buffer

	e1 := buildEngine(t, cfg, sim.NewCollectorPolicy())
	r1, err := e1.Run()
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if err := sim.WriteJSON(&buf1, r1); err != nil {
		t.Fatalf("write json 1: %v", err)
	}

	e2 := buildEngine(t, cfg, sim.NewCollectorPolicy())
	r2, err := e2.Run()
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if err := sim.WriteJSON(&buf2, r2); err != nil {
		t.Fatalf("write json 2: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("same seed + config produced different JSON output")
	}
}

func TestRun_coinRateChangesOutput(t *testing.T) {
	t.Parallel()
	cfg100 := smallConfig()
	cfg200 := smallConfig()
	cfg200.CoinRate = 200

	var buf100, buf200 bytes.Buffer

	e100 := buildEngine(t, cfg100, sim.NewCollectorPolicy())
	r100, err := e100.Run()
	if err != nil {
		t.Fatalf("run rate=100: %v", err)
	}
	if err := sim.WriteJSON(&buf100, r100); err != nil {
		t.Fatalf("write json: %v", err)
	}

	e200 := buildEngine(t, cfg200, sim.NewCollectorPolicy())
	r200, err := e200.Run()
	if err != nil {
		t.Fatalf("run rate=200: %v", err)
	}
	if err := sim.WriteJSON(&buf200, r200); err != nil {
		t.Fatalf("write json: %v", err)
	}

	if bytes.Equal(buf100.Bytes(), buf200.Bytes()) {
		t.Error("different coin rates produced identical JSON output")
	}
}

func TestRun_90daySmoke(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.RunDays = 90
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Days) != 90 {
		t.Errorf("days = %d, want 90", len(result.Days))
	}
	// Ledger invariant over all 90 days.
	for _, d := range result.Days {
		var sum domain.Coins
		for _, bal := range d.MoneySupply {
			sum += bal
		}
		if sum != 0 {
			t.Errorf("day %d: ledger sum = %d, want 0", d.Day, sum)
		}
		if bal := d.MoneySupply[domain.AccountPlayer]; bal < 0 {
			t.Errorf("day %d: player balance = %d, want >= 0", d.Day, bal)
		}
	}
}

func TestRun_unsoldDuplicateCopiesInSummary(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.RunDays = 5
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The metric must be present in the Summary regardless of its value.
	if _, ok := result.Summary["unsold_duplicate_copies"]; !ok {
		t.Error("unsold_duplicate_copies missing from Summary")
	}
}

// With one cost tier of 12, every booster needs the whole reserve plus 2 savings points, so the
// run reports the same wait whatever the card prices are.
func TestRun_maxBoosterWaitInSummary(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.Energy.CostTable = []config.EnergyCost{{Threshold: 400, Cost: 12}}
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())

	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	const want = 160 // 10 reserve ticks + 2 savings points x 3 ticks, at 10 minutes a tick
	if got := result.Summary["max_booster_wait_minutes"]; got != want {
		t.Errorf("max_booster_wait_minutes = %v, want %v", got, want)
	}
}

func TestRun_secondCallReturnsError(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.RunDays = 1
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())

	if _, err := e.Run(); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if _, err := e.Run(); err == nil {
		t.Error("second Run: want error, got nil")
	}
}

// TestNewEngine_errorWhenNoPricesMatchCatalogueCards reproduces defect 4.
// When the price snapshot matches none of the catalogue cards, NewEngine must
// return a descriptive error rather than silently falling back for every card.
func TestNewEngine_errorWhenNoPricesMatchCatalogueCards(t *testing.T) {
	t.Parallel()

	mainCard := domain.Card{
		ID:       "main-r-001",
		Name:     "Main Rare",
		Number:   "1",
		Rarity:   domain.RarityRare,
		Finishes: []domain.Finish{domain.FinishNonFoil},
	}
	mainCat := domain.SetCatalogue{
		Code:  "MAN",
		Name:  "Main Set",
		Cards: map[domain.CardID]domain.Card{"main-r-001": mainCard},
		Sheets: map[string]domain.Sheet{
			"rare": {
				Name:        "rare",
				Cards:       []domain.SheetCard{{ID: "main-r-001", Weight: 10}},
				TotalWeight: 10,
			},
		},
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"rare": 1}, Weight: 1},
		},
	}

	// Price snapshot with a completely wrong key — none of the catalogue cards are priced.
	wrongPrices := domain.PriceSnapshot{"totally-wrong-uuid": 100}

	cfg := smallConfig()
	cfg.RunDays = 1
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))

	_, err := sim.NewEngine(cfg, []domain.SetCatalogue{mainCat}, nil, wrongPrices, sim.NewCollectorPolicy(), rng)
	if err == nil {
		t.Fatal("want error when price snapshot matches no catalogue card, got nil")
	}
	if !strings.Contains(err.Error(), "price snapshot matches none") {
		t.Errorf("error %q does not contain expected message 'price snapshot matches none'", err.Error())
	}
}

// TestRun_unpricedCardsInSummary verifies the unpriced_cards metric is present
// and reflects cards absent from the price snapshot.
func TestRun_unpricedCardsInSummary(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.RunDays = 1
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// unpriced_cards must be present (domaintest.NewPrices intentionally omits the last card).
	v, ok := result.Summary["unpriced_cards"]
	if !ok {
		t.Fatal("unpriced_cards missing from Summary")
	}
	if v < 1 {
		t.Errorf("unpriced_cards = %v, want >= 1 (NewPrices omits the last card)", v)
	}
}

// TestRun_subCoinCardsIncludesSupplementaryInSummary reproduces defect 3.
// A supplementary catalogue with a cheap card must contribute to sub_coin_cards.
func TestRun_subCoinCardsIncludesSupplementaryInSummary(t *testing.T) {
	t.Parallel()

	mainCard := domain.Card{
		ID:       "main-r-002",
		Name:     "Main Rare",
		Number:   "1",
		Rarity:   domain.RarityRare,
		Finishes: []domain.Finish{domain.FinishNonFoil},
	}
	mainCat := domain.SetCatalogue{
		Code:  "MAN",
		Name:  "Main Set",
		Cards: map[domain.CardID]domain.Card{"main-r-002": mainCard},
		Sheets: map[string]domain.Sheet{
			"rare": {
				Name:        "rare",
				Cards:       []domain.SheetCard{{ID: "main-r-002", Weight: 10}},
				TotalWeight: 10,
			},
		},
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"rare": 1}, Weight: 1},
		},
	}

	// Supplementary card with a price of zero: the Counter refuses it, see ADR-0044.
	suppCard := domain.Card{
		ID:       "supp-sp-001",
		Name:     "Supp Special",
		Number:   "1",
		Rarity:   domain.RaritySpecial,
		Finishes: []domain.Finish{domain.FinishNonFoil},
	}
	suppCat := domain.SetCatalogue{
		Code:  "SUP",
		Name:  "Supplementary Set",
		Cards: map[domain.CardID]domain.Card{"supp-sp-001": suppCard},
	}

	prices := domain.PriceSnapshot{
		"main-r-002":  500, // well-priced
		"supp-sp-001": 0,   // refused by the Counter
	}

	cfg := smallConfig()
	cfg.RunDays = 1
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))

	e, err := sim.NewEngine(cfg, []domain.SetCatalogue{mainCat}, []domain.SetCatalogue{suppCat}, prices, sim.NewCollectorPolicy(), rng)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	v, ok := result.Summary["sub_coin_cards"]
	if !ok {
		t.Fatal("sub_coin_cards missing from Summary")
	}
	// supp-sp-001 is sub-coin; sub_coin_cards must be >= 1.
	if v < 1 {
		t.Errorf("sub_coin_cards = %v, want >= 1 (supplementary sub-coin card must be counted)", v)
	}
}

// TestRun_supplementaryCatalogues verifies that cards from a supplementary catalogue
// (cards known to the Counter but not tracked in binder progress or energy cost
// computation) can be sold without ErrUnknownCard.
func TestRun_supplementaryCatalogues(t *testing.T) {
	t.Parallel()

	// Main catalogue: one rare card only (simplest possible set).
	mainCard := domain.Card{
		ID:       "main-r-000",
		Name:     "Main Rare",
		Number:   "1",
		Rarity:   domain.RarityRare,
		Finishes: []domain.Finish{domain.FinishNonFoil},
	}
	mainCat := domain.SetCatalogue{
		Code:  "MAN",
		Name:  "Main Set",
		Cards: map[domain.CardID]domain.Card{"main-r-000": mainCard},
		Sheets: map[string]domain.Sheet{
			"rare": {
				Name:        "rare",
				Cards:       []domain.SheetCard{{ID: "main-r-000", Weight: 10}},
				TotalWeight: 10,
			},
		},
		BoosterOptions: []domain.BoosterOption{
			{Contents: map[string]int{"rare": 1}, Weight: 1},
		},
	}

	// Supplementary catalogue: cards referenced by booster sheets but not openable.
	suppCard := domain.Card{
		ID:       "supp-c-000",
		Name:     "Supp Common",
		Number:   "1",
		Rarity:   domain.RarityCommon,
		Finishes: []domain.Finish{domain.FinishNonFoil},
	}
	suppCat := domain.SetCatalogue{
		Code:  "SUP",
		Name:  "Supplementary Set",
		Cards: map[domain.CardID]domain.Card{"supp-c-000": suppCard},
	}

	prices := domain.PriceSnapshot{
		"main-r-000": 500,
		"supp-c-000": 50,
	}

	cfg := smallConfig()
	cfg.RunDays = 2
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))

	e, err := sim.NewEngine(cfg, []domain.SetCatalogue{mainCat}, []domain.SetCatalogue{suppCat}, prices, sim.NewCollectorPolicy(), rng)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run with supplementary catalogues: %v", err)
	}
	// The supplementary set should not appear in SetProgress.
	for _, d := range result.Days {
		if _, ok := d.SetProgress["SUP"]; ok {
			t.Error("supplementary set SUP should not appear in SetProgress")
		}
		if _, ok := d.SetProgress["MAN"]; !ok {
			t.Error("main set MAN should appear in SetProgress")
		}
	}
}
