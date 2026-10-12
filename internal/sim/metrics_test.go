package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// makeMetricsTestResult builds a Result where the RAV set reaches 50% CU on day 3
// and 100% on day 5 (out of a 5-day run).
func makeMetricsTestResult(runDays int) sim.Result {
	days := make([]sim.DayMetrics, runDays)
	for i := range runDays {
		day := i + 1
		cuOwned := 0
		if day >= 3 {
			cuOwned = 7 // 50% of 14
		}
		if day >= 5 {
			cuOwned = 14 // 100% of 14
		}
		days[i] = sim.DayMetrics{
			Day: day,
			MoneySupply: map[domain.Account]domain.Coins{
				domain.AccountPlayer:          100,
				domain.AccountWelcomeBonus:    -200,
				domain.AccountDailyReward:     domain.Coins(-100 * day),
				domain.AccountCounterSale:     0,
				domain.AccountCounterPurchase: 100 + domain.Coins(100*day),
			},
			SetProgress: map[string]sim.SetDayProgress{
				"RAV": {CUOwned: cuOwned, CUTotal: 14},
			},
		}
	}
	return sim.Result{Days: days}
}

func TestSummary_daysTo50pct(t *testing.T) {
	t.Parallel()
	cats := []domain.SetCatalogue{{Code: "RAV"}}
	r := makeMetricsTestResult(5)
	s := sim.ComputeSummaryForTest(r.Days, 5, cats, nil,
		domain.PriceSnapshot{}, 1, 100, 60, 120, 100)

	v, ok := s["days_to_50pct_cu.rav"]
	if !ok {
		t.Fatal("days_to_50pct_cu.rav not in summary")
	}
	if v != 3 {
		t.Errorf("days_to_50pct_cu.rav = %v, want 3", v)
	}
}

func TestSummary_daysTo100pct(t *testing.T) {
	t.Parallel()
	cats := []domain.SetCatalogue{{Code: "RAV"}}
	r := makeMetricsTestResult(5)
	s := sim.ComputeSummaryForTest(r.Days, 5, cats, nil,
		domain.PriceSnapshot{}, 1, 100, 60, 120, 100)

	v, ok := s["days_to_100pct_cu.rav"]
	if !ok {
		t.Fatal("days_to_100pct_cu.rav not in summary")
	}
	if v != 5 {
		t.Errorf("days_to_100pct_cu.rav = %v, want 5", v)
	}
}

func TestSummary_neverReachedSentinel(t *testing.T) {
	t.Parallel()
	cats := []domain.SetCatalogue{{Code: "RAV"}}
	// 3-day run; 50% is never reached (cuOwned stays 0).
	days := []sim.DayMetrics{
		{Day: 1, MoneySupply: zeroSupply(), SetProgress: map[string]sim.SetDayProgress{"RAV": {CUOwned: 0, CUTotal: 14}}},
		{Day: 2, MoneySupply: zeroSupply(), SetProgress: map[string]sim.SetDayProgress{"RAV": {CUOwned: 0, CUTotal: 14}}},
		{Day: 3, MoneySupply: zeroSupply(), SetProgress: map[string]sim.SetDayProgress{"RAV": {CUOwned: 0, CUTotal: 14}}},
	}
	s := sim.ComputeSummaryForTest(days, 3, cats, nil,
		domain.PriceSnapshot{}, 1, 100, 60, 120, 100)

	censored := float64(3 + 1) // RunDays + 1
	if v := s["days_to_50pct_cu.rav"]; v != censored {
		t.Errorf("days_to_50pct_cu.rav = %v, want censored %v", v, censored)
	}
	if v := s["days_to_100pct_cu.rav"]; v != censored {
		t.Errorf("days_to_100pct_cu.rav = %v, want censored %v", v, censored)
	}
}

func TestSummary_subCoinCards(t *testing.T) {
	t.Parallel()
	// Card with price 1 cent at rate 100, sell 60%:
	// PriceToCoins(1, 100, 60) = floor(1 * 100 * 60 / 10000) = floor(0.6) = 0 → ErrSubCoin.
	subCoinID := domain.CardID("sub-c")
	okID := domain.CardID("ok-c")
	cat := domain.SetCatalogue{
		Code: "TST",
		Cards: map[domain.CardID]domain.Card{
			subCoinID: {ID: subCoinID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
			okID:      {ID: okID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	prices := domain.PriceSnapshot{subCoinID: 1, okID: 500}
	days := []sim.DayMetrics{{Day: 1, MoneySupply: zeroSupply()}}
	s := sim.ComputeSummaryForTest(days, 1, []domain.SetCatalogue{cat}, nil, prices, 0, 100, 60, 120, 100)

	v := s["sub_coin_cards"]
	if v != 1 {
		t.Errorf("sub_coin_cards = %v, want 1", v)
	}
}

func TestSummary_dailyRewardCommons(t *testing.T) {
	t.Parallel()
	// Common card price: 100 cents. Rate 100. BuyPct 120.
	// buyCoins = floor(100 * 100 * 120 / 10000) = floor(1200) = 1200... wait that's wrong.
	// Let me recalculate: floor(100 * 100 * 120 / 10000) = floor(1200000/10000) = 120.
	// DailyReward = 100. floor(100 / 120) = 0? That doesn't work for a useful test.
	//
	// Use: price 42 cents, rate 100, BuyPct 120.
	// buyCoins = floor(42 * 100 * 120 / 10000) = floor(504000/10000) = 50.
	// DailyReward = 100. floor(100 / 50) = 2.
	commonID := domain.CardID("common-c")
	cat := domain.SetCatalogue{
		Code: "TST",
		Cards: map[domain.CardID]domain.Card{
			commonID: {ID: commonID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	prices := domain.PriceSnapshot{commonID: 42}
	days := []sim.DayMetrics{{Day: 1, MoneySupply: zeroSupply()}}
	s := sim.ComputeSummaryForTest(days, 1, []domain.SetCatalogue{cat}, nil, prices, 1, 100, 60, 120, 100)

	if v := s["daily_reward_commons"]; v != 2 {
		t.Errorf("daily_reward_commons = %v, want 2 (100 coins / 50 coins per common)", v)
	}
}

func TestSummary_unsoldDuplicateCopies(t *testing.T) {
	t.Parallel()
	// Pass nil collection to ComputeSummaryForTest → should return 0.
	days := []sim.DayMetrics{{Day: 1, MoneySupply: zeroSupply()}}
	s := sim.ComputeSummaryForTest(days, 1, nil, nil,
		domain.PriceSnapshot{}, 1, 100, 60, 120, 100)
	if v := s["unsold_duplicate_copies"]; v != 0 {
		t.Errorf("unsold_duplicate_copies with nil collection = %v, want 0", v)
	}
}

// TestSummary_subCoinCards_zeroPrice reproduces defect 2: a card with price 0
// returns (0, nil) from PriceToCoins but the Counter refuses it as sub-coin.
// countSubCoinCards must count it.
func TestSummary_subCoinCards_zeroPrice(t *testing.T) {
	t.Parallel()
	zeroPriceID := domain.CardID("zero-price")
	cat := domain.SetCatalogue{
		Code: "TST",
		Cards: map[domain.CardID]domain.Card{
			zeroPriceID: {ID: zeroPriceID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// price 0 → PriceToCoins(0, 100, 60) = (0, nil); Counter still refuses it.
	prices := domain.PriceSnapshot{zeroPriceID: 0}
	days := []sim.DayMetrics{{Day: 1, MoneySupply: zeroSupply()}}
	s := sim.ComputeSummaryForTest(days, 1, []domain.SetCatalogue{cat}, nil, prices, 0, 100, 60, 120, 100)

	if v := s["sub_coin_cards"]; v != 1 {
		t.Errorf("sub_coin_cards = %v, want 1 (zero-priced card must be counted)", v)
	}
}

// TestSummary_subCoinCardsIncludesSupplementary reproduces defect 3: supplementary
// catalogue cards must be counted in sub_coin_cards.
func TestSummary_subCoinCardsIncludesSupplementary(t *testing.T) {
	t.Parallel()
	suppID := domain.CardID("supp-cheap")
	suppCat := domain.SetCatalogue{
		Code: "SUP",
		Cards: map[domain.CardID]domain.Card{
			suppID: {ID: suppID, Rarity: domain.RaritySpecial, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// price 1 cent at rate 100, sell 60%: floor(1*100*60/10000) = 0 → ErrSubCoin.
	prices := domain.PriceSnapshot{suppID: 1}
	days := []sim.DayMetrics{{Day: 1, MoneySupply: zeroSupply()}}
	// cats is nil (no openable catalogues); supplementary has one sub-coin card.
	s := sim.ComputeSummaryForTest(days, 1, nil, []domain.SetCatalogue{suppCat}, prices, 0, 100, 60, 120, 100)

	if v := s["sub_coin_cards"]; v != 1 {
		t.Errorf("sub_coin_cards = %v, want 1 (supplementary card must be counted)", v)
	}
}

// TestSummary_unpricedCards reproduces defect 4: cards absent from the price snapshot
// must be counted in the unpriced_cards metric.
func TestSummary_unpricedCards(t *testing.T) {
	t.Parallel()
	pricedID := domain.CardID("priced-c")
	unpricedID := domain.CardID("unpriced-c")
	cat := domain.SetCatalogue{
		Code: "TST",
		Cards: map[domain.CardID]domain.Card{
			pricedID:   {ID: pricedID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
			unpricedID: {ID: unpricedID, Rarity: domain.RarityRare, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// Only pricedID is in the snapshot; unpricedID uses fallback.
	prices := domain.PriceSnapshot{pricedID: 100}
	days := []sim.DayMetrics{{Day: 1, MoneySupply: zeroSupply()}}
	s := sim.ComputeSummaryForTest(days, 1, []domain.SetCatalogue{cat}, nil, prices, 1, 100, 60, 120, 100)

	if v := s["unpriced_cards"]; v != 1 {
		t.Errorf("unpriced_cards = %v, want 1", v)
	}
}

// TestSummary_unpricedCards_includesSupplementary verifies that supplementary catalogue
// cards are also counted in unpriced_cards.
func TestSummary_unpricedCards_includesSupplementary(t *testing.T) {
	t.Parallel()
	mainID := domain.CardID("main-c")
	suppID := domain.CardID("supp-c")
	mainCat := domain.SetCatalogue{
		Code: "MAN",
		Cards: map[domain.CardID]domain.Card{
			mainID: {ID: mainID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	suppCat := domain.SetCatalogue{
		Code: "SUP",
		Cards: map[domain.CardID]domain.Card{
			suppID: {ID: suppID, Rarity: domain.RaritySpecial, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// mainID has a price; suppID does not.
	prices := domain.PriceSnapshot{mainID: 100}
	days := []sim.DayMetrics{{Day: 1, MoneySupply: zeroSupply()}}
	s := sim.ComputeSummaryForTest(days, 1, []domain.SetCatalogue{mainCat}, []domain.SetCatalogue{suppCat}, prices, 1, 100, 60, 120, 100)

	if v := s["unpriced_cards"]; v != 1 {
		t.Errorf("unpriced_cards = %v, want 1 (supplementary unpriced card must be counted)", v)
	}
}

// zeroSupply returns a MoneySupply where all accounts are zero (ledger balanced).
func zeroSupply() map[domain.Account]domain.Coins {
	return map[domain.Account]domain.Coins{
		domain.AccountPlayer:          0,
		domain.AccountWelcomeBonus:    0,
		domain.AccountDailyReward:     0,
		domain.AccountCounterSale:     0,
		domain.AccountCounterPurchase: 0,
	}
}

// binderDays builds days where each set's binder fills on the given day (Owned reaches Total),
// and the player's balance is 100 coins times the day number.
func binderDays(runDays int, completeOn map[string]int) []sim.DayMetrics {
	const total = 12
	days := make([]sim.DayMetrics, runDays)
	for i := range runDays {
		day := i + 1
		progress := make(map[string]sim.SetDayProgress, len(completeOn))
		for code, doneDay := range completeOn {
			owned := total - 2
			if doneDay != 0 && day >= doneDay {
				owned = total
			}
			progress[code] = sim.SetDayProgress{Owned: owned, Total: total}
		}
		days[i] = sim.DayMetrics{
			Day:         day,
			MoneySupply: map[domain.Account]domain.Coins{domain.AccountPlayer: domain.Coins(100 * day)},
			SetProgress: progress,
		}
	}
	return days
}

func TestSummary_daysToBinder(t *testing.T) {
	t.Parallel()
	const runDays = 6
	tests := []struct {
		name       string
		completeOn map[string]int
		want       map[string]float64
	}{
		{
			name:       "each set reports its own day",
			completeOn: map[string]int{"RAV": 4, "GPT": 2},
			want:       map[string]float64{"days_to_100pct.rav": 4, "days_to_100pct.gpt": 2},
		},
		{
			name:       "a set never completed is censored past the run horizon",
			completeOn: map[string]int{"RAV": 4, "GPT": 0},
			want:       map[string]float64{"days_to_100pct.rav": 4, "days_to_100pct.gpt": runDays + 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cats := []domain.SetCatalogue{{Code: "RAV"}, {Code: "GPT"}}

			s := sim.ComputeSummaryForTest(binderDays(runDays, tc.completeOn), runDays, cats, nil,
				domain.PriceSnapshot{}, 1, 100, 60, 120, 100)

			for key, want := range tc.want {
				if got, ok := s[key]; !ok || got != want {
					t.Errorf("%s = %v (present: %v), want %v", key, got, ok, want)
				}
			}
		})
	}
}

func TestSummary_coinsAfterFullBinder(t *testing.T) {
	t.Parallel()
	const runDays = 6
	tests := []struct {
		name       string
		completeOn map[string]int
		want       float64
	}{
		{
			name:       "counts from the day the last set completes",
			completeOn: map[string]int{"RAV": 2, "GPT": 4},
			want:       200, // balance 600 on day 6 minus 400 on day 4
		},
		{
			name:       "zero when one set never completes",
			completeOn: map[string]int{"RAV": 2, "GPT": 0},
			want:       0,
		},
		{
			name:       "zero when the binder completes on the last day",
			completeOn: map[string]int{"RAV": runDays, "GPT": 3},
			want:       0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cats := []domain.SetCatalogue{{Code: "RAV"}, {Code: "GPT"}}

			s := sim.ComputeSummaryForTest(binderDays(runDays, tc.completeOn), runDays, cats, nil,
				domain.PriceSnapshot{}, 1, 100, 60, 120, 100)

			if got := s["coins_after_full_binder"]; got != tc.want {
				t.Errorf("coins_after_full_binder = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMaxBoosterWaitMinutes(t *testing.T) {
	t.Parallel()
	params := domain.EnergyParams{RechargePerTick: 1, ReserveCap: 10, SavingsIntervalTicks: 3, SavingsCap: 200}
	costs := map[string]int{"CSP": 3, "RAV": 5, "TSP": 12}

	got, err := sim.MaxBoosterWaitMinutesForTest(params, costs)
	if err != nil {
		t.Fatal(err)
	}

	// The cost-12 booster needs the 10-point reserve (10 ticks) plus 2 savings (2 x 3 ticks).
	const want = 160
	if got != want {
		t.Errorf("max wait = %v minutes, want %v", got, want)
	}
}
