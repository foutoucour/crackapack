package config

import "fmt"

// validateEnergy checks the energy block fields and the cost table.
// It returns an error naming the offending key on the first failure.
//
// In addition to the structural checks (positive recharge, non-negative savings
// cap, non-empty ascending cost table), it verifies that every cost-table entry
// has a cost that the player can actually accumulate: cost must not exceed
// reserve_cap + savings_cap, because that is the most energy the player can
// ever hold.
func validateEnergy(e EnergyConfig) error {
	if e.RechargePerTick <= 0 {
		return fmt.Errorf("config: energy.recharge_per_tick must be > 0, got %d", e.RechargePerTick)
	}
	if e.ReserveCap <= 0 {
		return fmt.Errorf("config: energy.reserve_cap must be > 0, got %d", e.ReserveCap)
	}
	if e.SavingsIntervalTicks <= 0 {
		return fmt.Errorf("config: energy.savings_interval_ticks must be > 0, got %d", e.SavingsIntervalTicks)
	}
	if e.SavingsCap < 0 {
		return fmt.Errorf("config: energy.savings_cap must be >= 0, got %d", e.SavingsCap)
	}
	if e.CoinsPerEnergy < 0 {
		return fmt.Errorf("config: energy.coins_per_energy must be >= 0, got %d", e.CoinsPerEnergy)
	}
	if len(e.CostTable) == 0 {
		return fmt.Errorf("config: energy.cost_table must not be empty")
	}
	maxAffordable := e.ReserveCap + e.SavingsCap
	for i, entry := range e.CostTable {
		if entry.Threshold <= 0 {
			return fmt.Errorf("config: energy.cost_table[%d].threshold must be > 0, got %d", i, entry.Threshold)
		}
		if entry.Cost <= 0 {
			return fmt.Errorf("config: energy.cost_table[%d].cost must be > 0, got %d", i, entry.Cost)
		}
		if entry.Cost > maxAffordable {
			return fmt.Errorf(
				"config: energy.cost_table[%d].cost (%d) exceeds reserve_cap (%d) + savings_cap (%d); player can never afford to open a booster",
				i, entry.Cost, e.ReserveCap, e.SavingsCap,
			)
		}
	}
	for i := 1; i < len(e.CostTable); i++ {
		if e.CostTable[i].Threshold <= e.CostTable[i-1].Threshold {
			return fmt.Errorf(
				"config: energy.cost_table: entry %d threshold (%d) must be > entry %d threshold (%d); table must be sorted ascending",
				i, e.CostTable[i].Threshold, i-1, e.CostTable[i-1].Threshold,
			)
		}
	}
	return nil
}
