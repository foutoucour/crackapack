package sim

import "github.com/foutoucour/crackapack/internal/domain"

// ComputeSummaryForTest exposes computeSummary for white-box tests in sim_test.
// It mirrors the internal signature exactly so test helpers can call it
// without embedding engine logic. Pass nil for collection when the test does
// not exercise unsold_duplicate_copies.
func ComputeSummaryForTest(
	days []DayMetrics,
	runDays int,
	cats []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	fallback domain.USDCents,
	coinRate domain.CoinRate,
	sellPct domain.Percent,
	buyPct domain.Percent,
	dailyReward domain.Coins,
) Summary {
	return computeSummary(days, runDays, cats, prices, fallback, coinRate, sellPct, buyPct, dailyReward, nil)
}
