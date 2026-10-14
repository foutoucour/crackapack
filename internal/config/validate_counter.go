package config

import "fmt"

// validateCounterBudget checks the Counter's treasury block. It returns an error naming the
// offending key.
func validateCounterBudget(b CounterBudgetConfig) error {
	switch {
	case b.Initial < 0:
		return fmt.Errorf("config: counter_budget.initial must be >= 0, got %d", b.Initial)
	case b.DailyFloat < 0:
		return fmt.Errorf("config: counter_budget.daily_float must be >= 0, got %d", b.DailyFloat)
	}
	return nil
}

// validateCounterSupply checks the Counter's supply block against the player's cost table, which
// sets the energy cost of the boosters the Counter opens in energy mode, and against the budget,
// which pays the coin fee. It returns an error naming the offending key.
func validateCounterSupply(cfg Config) error {
	s, costTable := cfg.CounterSupply, cfg.Energy.CostTable
	if s.CoinsPerEnergy < 0 {
		return fmt.Errorf("config: counter_supply.coins_per_energy must be >= 0, got %d", s.CoinsPerEnergy)
	}
	if s.CoinsPerEnergy > 0 && (s.Mode != CounterSupplyEnergy || !cfg.CounterBudget.Enabled) {
		return fmt.Errorf("config: counter_supply.coins_per_energy needs counter_supply.mode %q and counter_budget.enabled, as the treasury pays the fee", CounterSupplyEnergy)
	}
	switch s.Mode {
	case CounterSupplyNone:
		return nil
	case CounterSupplyFixed:
		if s.BoostersPerDay < 1 {
			return fmt.Errorf("config: counter_supply.boosters_per_day must be >= 1 in %s mode, got %d", CounterSupplyFixed, s.BoostersPerDay)
		}
		return nil
	case CounterSupplyEnergy:
		return validateCounterEnergy(s.Energy, costTable)
	default:
		return fmt.Errorf("config: counter_supply.mode must be %q, %q or %q, got %q",
			CounterSupplyNone, CounterSupplyFixed, CounterSupplyEnergy, s.Mode)
	}
}

// validateCounterEnergy checks the Counter's energy. The reserve must hold the dearest cost of the
// cost table: the Counter's pick stays until it can be paid, and savings only grow slowly while
// the reserve is full, so a cost that needs savings can leave the Counter nearly idle, see
// ADR-0015.
func validateCounterEnergy(e CounterEnergyConfig, costTable []EnergyCost) error {
	switch {
	case e.RechargePerTick <= 0:
		return fmt.Errorf("config: counter_supply.energy.recharge_per_tick must be > 0, got %d", e.RechargePerTick)
	case e.ReserveCap <= 0:
		return fmt.Errorf("config: counter_supply.energy.reserve_cap must be > 0, got %d", e.ReserveCap)
	case e.SavingsIntervalTicks <= 0:
		return fmt.Errorf("config: counter_supply.energy.savings_interval_ticks must be > 0, got %d", e.SavingsIntervalTicks)
	case e.SavingsCap < 0:
		return fmt.Errorf("config: counter_supply.energy.savings_cap must be >= 0, got %d", e.SavingsCap)
	}
	dearest, at := 0, -1
	for i, entry := range costTable {
		if entry.Cost > dearest {
			dearest, at = entry.Cost, i
		}
	}
	if dearest > e.ReserveCap {
		return fmt.Errorf(
			"config: counter_supply.energy.reserve_cap (%d) must hold the dearest energy cost, %d (energy.cost_table[%d].cost); savings do not count, the Counter could stay nearly idle",
			e.ReserveCap, dearest, at,
		)
	}
	return nil
}
