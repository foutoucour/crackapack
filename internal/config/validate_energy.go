package config

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// validateEnergy checks the energy block fields and the cost table.
// It returns an error naming the offending key on the first failure.
//
// In addition to the structural checks (positive recharge, non-negative savings
// cap, non-empty cost table ascending in both threshold and cost), it verifies that every cost-table entry
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
	if err := validateEnergyCoinTopup(e.CoinTopup, e.CoinsPerEnergy); err != nil {
		return err
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
		if e.CostTable[i].Cost <= e.CostTable[i-1].Cost {
			return fmt.Errorf(
				"config: energy.cost_table[%d].cost (%d) must be > entry %d cost (%d); a dearer booster must cost more energy",
				i, e.CostTable[i].Cost, i-1, e.CostTable[i-1].Cost,
			)
		}
	}
	return nil
}

// validateEnergyCoinTopup checks the energy coin top-up sub-block.
// coinsPerEnergy is the outer energy.coins_per_energy; coin_topup needs it above zero.
func validateEnergyCoinTopup(c EnergyCoinTopupConfig, coinsPerEnergy domain.Coins) error {
	if !c.Enabled {
		return nil
	}
	if coinsPerEnergy <= 0 {
		return fmt.Errorf("config: energy.coin_topup.enabled requires energy.coins_per_energy > 0")
	}
	switch c.Variant {
	case CoinTopupVariantMinEnergy, CoinTopupVariantEscalating, CoinTopupVariantBoth:
		// valid
	default:
		return fmt.Errorf("config: energy.coin_topup.variant must be %q, %q or %q, got %q",
			CoinTopupVariantMinEnergy, CoinTopupVariantEscalating, CoinTopupVariantBoth, c.Variant)
	}
	// min_energy_units must be >= 1 for all variants (can never open with coins only).
	if c.MinEnergyUnits < 1 {
		return fmt.Errorf("config: energy.coin_topup.min_energy_units must be >= 1, got %d", c.MinEnergyUnits)
	}
	if c.Variant == CoinTopupVariantEscalating || c.Variant == CoinTopupVariantBoth {
		if c.EscalationStep < 1 {
			return fmt.Errorf("config: energy.coin_topup.escalation_step must be >= 1, got %d", c.EscalationStep)
		}
	}
	return nil
}
