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
	s := sim.ComputeSummaryForTest(r.Days, 5, cats,
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
	s := sim.ComputeSummaryForTest(r.Days, 5, cats,
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
	s := sim.ComputeSummaryForTest(days, 3, cats,
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
	s := sim.ComputeSummaryForTest(days, 1, []domain.SetCatalogue{cat}, prices, 0, 100, 60, 120, 100)

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
	s := sim.ComputeSummaryForTest(days, 1, []domain.SetCatalogue{cat}, prices, 1, 100, 60, 120, 100)

	if v := s["daily_reward_commons"]; v != 2 {
		t.Errorf("daily_reward_commons = %v, want 2 (100 coins / 50 coins per common)", v)
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
