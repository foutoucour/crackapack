package sim

import "github.com/foutoucour/crackapack/internal/domain"

// WorldView is a point-in-time snapshot of the simulation state visible to a
// PlayerPolicy. The engine recomputes it fresh on each call to Session.View(),
// so the policy always sees the state after its most recent action.
type WorldView struct {
	// Day is the current simulated day (1-based; day 1 is the first day).
	Day int

	// Balance is the player's current coin balance.
	Balance domain.Coins

	// DailyReward is the configured coins awarded per day, used by policies to
	// compute coin floors without needing direct access to config.
	DailyReward domain.Coins

	// EnergyTotal is the sum of reserve and savings energy available right now.
	EnergyTotal int

	// SetProgress maps each openable set's code to the player's binder progress.
	// Basic lands are excluded per domain rules.
	SetProgress map[string]domain.Progress

	// BoosterCosts maps each openable set's code to its energy cost per booster.
	BoosterCosts map[string]int

	// BoosterFees maps each openable set's code to the coins a booster of it costs on top of its
	// energy. A set with no fee is absent, and the map is empty when the run has no fee price.
	BoosterFees map[string]domain.Coins

	// Catalogues contains the openable set catalogues (not supplementary ones).
	Catalogues []domain.SetCatalogue

	// Prices is the fixed price snapshot for the run.
	Prices domain.PriceSnapshot

	// OwnedCards maps each CardKey the player currently holds to its quantity.
	// The map is a snapshot copy; mutations do not affect the simulation.
	OwnedCards map[domain.CardKey]int

	// CoinRate is the configured coins-per-USD rate.
	CoinRate domain.CoinRate

	// CounterBuyPct is the percentage of reference value the player pays when
	// buying a card from the Counter (default 120).
	CounterBuyPct domain.Percent

	// CounterSellPct is the percentage of reference value the player receives
	// when selling a card to the Counter (default 60).
	CounterSellPct domain.Percent

	// FallbackPrice is the price in USDCents for cards absent from Prices.
	FallbackPrice domain.USDCents
}
