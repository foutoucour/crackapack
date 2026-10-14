package config

import "fmt"

// Upper bounds of the numeric keys. Without them a value near the int64 limit overflows the
// ledger or a price computation, and an absurd energy or supply makes a run last minutes. They sit
// far above any sensible setting, see ADR-0014.
const (
	// maxCoinRate is the documented realistic bound of the price-to-coins conversion.
	maxCoinRate = 1_000
	// maxPercent bounds the Counter's sell and buy percentages, as the price conversion documents.
	maxPercent = 10_000
	// maxCoins bounds every amount of coins a key sets. A year of it still sits far below int64.
	maxCoins = 1_000_000_000_000
	// maxEnergyRecharge, maxEnergyReserve, maxEnergySavings and maxEnergyCost bound the energy keys of
	// the player and of the Counter alike.
	maxEnergyRecharge = 1_000
	maxEnergyReserve  = 10_000
	maxEnergySavings  = 100_000
	maxEnergyCost     = 10_000
	// maxBoostersPerDay bounds the Counter's fixed supply.
	maxBoostersPerDay = 100_000
)

// validateBounds checks every key that has an upper bound and returns an error naming the first one
// above it. It does not check lower bounds: each key's own validation does.
func validateBounds(cfg Config) error {
	checks := []struct {
		key   string
		value int64
		max   int64
	}{
		{"coin_rate", int64(cfg.CoinRate), maxCoinRate},
		{"counter_sell_pct", int64(cfg.CounterSellPct), maxPercent},
		{"counter_buy_pct", int64(cfg.CounterBuyPct), maxPercent},
		{"welcome_bonus", int64(cfg.WelcomeBonus), maxCoins},
		{"daily_reward", int64(cfg.DailyReward), maxCoins},
		{"counter_budget.initial", int64(cfg.CounterBudget.Initial), maxCoins},
		{"counter_budget.daily_float", int64(cfg.CounterBudget.DailyFloat), maxCoins},
		{"counter_supply.coins_per_energy", int64(cfg.CounterSupply.CoinsPerEnergy), maxCoins},
		{"counter_supply.boosters_per_day", int64(cfg.CounterSupply.BoostersPerDay), maxBoostersPerDay},
		{"energy.coins_per_energy", int64(cfg.Energy.CoinsPerEnergy), maxCoins},
		{"energy.recharge_per_tick", int64(cfg.Energy.RechargePerTick), maxEnergyRecharge},
		{"energy.reserve_cap", int64(cfg.Energy.ReserveCap), maxEnergyReserve},
		{"energy.savings_cap", int64(cfg.Energy.SavingsCap), maxEnergySavings},
		{"counter_supply.energy.recharge_per_tick", int64(cfg.CounterSupply.Energy.RechargePerTick), maxEnergyRecharge},
		{"counter_supply.energy.reserve_cap", int64(cfg.CounterSupply.Energy.ReserveCap), maxEnergyReserve},
		{"counter_supply.energy.savings_cap", int64(cfg.CounterSupply.Energy.SavingsCap), maxEnergySavings},
	}
	for i, entry := range cfg.Energy.CostTable {
		checks = append(checks, struct {
			key   string
			value int64
			max   int64
		}{fmt.Sprintf("energy.cost_table[%d].cost", i), int64(entry.Cost), maxEnergyCost})
	}
	for _, c := range checks {
		if c.value > c.max {
			return fmt.Errorf("config: %s must be <= %d, got %d", c.key, c.max, c.value)
		}
	}
	return nil
}
