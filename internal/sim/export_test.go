package sim

import "github.com/foutoucour/crackapack/internal/domain"

// ComputeSummaryForTest exposes computeSummary for white-box tests in sim_test.
// It mirrors the internal signature exactly so test helpers can call it
// without embedding engine logic. Pass nil for cats or supplementary when the
// test does not need those catalogues. Pass nil for collection when the test does
// not exercise unsold_duplicate_copies.
func ComputeSummaryForTest(
	days []DayMetrics,
	runDays int,
	cats []domain.SetCatalogue,
	supplementary []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	fallback domain.USDCents,
	coinRate domain.CoinRate,
	sellPct domain.Percent,
	buyPct domain.Percent,
	dailyReward domain.Coins,
) Summary {
	return computeSummary(days, runDays, cats, supplementary, prices, fallback, coinRate, sellPct, buyPct, dailyReward, nil)
}

// MaxBoosterWaitMinutesForTest exposes maxBoosterWaitMinutes for white-box tests in sim_test.
func MaxBoosterWaitMinutesForTest(p domain.EnergyParams, costs map[string]int) (float64, error) {
	return maxBoosterWaitMinutes(p, costs)
}

// CoinFeesForTest exposes coinFees for white-box tests in sim_test.
func CoinFeesForTest(p domain.Coins, energyCap int, coinRate domain.CoinRate, sellPct domain.Percent, avgValues []domain.USDCents) (map[string]domain.Coins, error) {
	cats := make([]domain.SetCatalogue, len(avgValues))
	for i := range cats {
		cats[i].Code = string(rune('A' + i))
	}
	return coinFees(p, energyCap, coinRate, sellPct, cats, avgValues)
}
