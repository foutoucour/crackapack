package config

import "fmt"

// validateCounterSupply checks the Counter's supply block against the player's cost table, which
// sets the energy cost of the boosters the Counter opens in energy mode. It returns an error
// naming the offending key.
func validateCounterSupply(s CounterSupplyConfig, costTable []EnergyCost) error {
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

// validateCounterEnergy checks the Counter's energy. Like the player's, it must be able to hold the
// cost of every booster, or the Counter would never open that tier.
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
	for i, entry := range costTable {
		if entry.Cost > e.ReserveCap+e.SavingsCap {
			return fmt.Errorf(
				"config: counter_supply.energy: energy.cost_table[%d].cost (%d) exceeds reserve_cap (%d) + savings_cap (%d); the Counter could never open that booster",
				i, entry.Cost, e.ReserveCap, e.SavingsCap,
			)
		}
	}
	return nil
}
