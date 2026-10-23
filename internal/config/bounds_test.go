package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// A key above its upper bound is rejected with a message naming it, and a key exactly at the bound
// is accepted. Unbounded values overflowed int64 and broke the ledger, or ran for minutes.
func TestLoad_upperBounds(t *testing.T) {
	t.Parallel()

	const energyMode = "counter_supply:\n  mode: energy\n"
	tests := []struct {
		key    string
		yaml   string // with %d where the value goes
		bound  int64
		extras string
	}{
		{key: "coin_rate", yaml: "coin_rate: %d\n", bound: 1_000},
		{key: "counter_sell_pct", yaml: "counter_sell_pct: %d\ncounter_buy_pct: 10000\n", bound: 10_000},
		{key: "counter_buy_pct", yaml: "counter_buy_pct: %d\n", bound: 10_000},
		{key: "counter_foil_pct", yaml: "counter_foil_pct: %d\n", bound: 10_000},
		{key: "expensive_booster_value", yaml: "expensive_booster_value: %d\n", bound: 100_000_000},
		{key: "welcome_bonus", yaml: "welcome_bonus: %d\n", bound: 1_000_000_000_000},
		{key: "daily_reward", yaml: "daily_reward: %d\n", bound: 1_000_000_000_000},
		{key: "counter_budget.initial", yaml: "counter_budget:\n  enabled: true\n  initial: %d\n", bound: 1_000_000_000_000},
		{key: "counter_budget.daily_float", yaml: "counter_budget:\n  enabled: true\n  daily_float: %d\n", bound: 1_000_000_000_000},
		{key: "counter_supply.coins_per_energy", yaml: "counter_budget:\n  enabled: true\ncounter_supply:\n  mode: energy\n  coins_per_energy: %d\n", bound: 1_000_000_000_000},
		{key: "counter_supply.boosters_per_day", yaml: "counter_supply:\n  mode: fixed\n  boosters_per_day: %d\n", bound: 100_000},
		{key: "energy.coins_per_energy", yaml: "energy:\n  coins_per_energy: %d\n", bound: 1_000_000_000_000},
		{key: "energy.recharge_per_tick", yaml: "energy:\n  recharge_per_tick: %d\n", bound: 1_000},
		{key: "energy.reserve_cap", yaml: "energy:\n  reserve_cap: %d\n", bound: 10_000},
		{key: "energy.savings_cap", yaml: "energy:\n  savings_cap: %d\n", bound: 100_000},
		{key: "energy.cost_table[0].cost", yaml: "energy:\n  reserve_cap: 10000\n  savings_cap: 100000\n  cost_table:\n    - {threshold: 100, cost: %d}\n", bound: 10_000},
		{key: "counter_supply.energy.recharge_per_tick", yaml: energyMode + "  energy: {recharge_per_tick: %d}\n", bound: 1_000},
		{key: "counter_supply.energy.reserve_cap", yaml: energyMode + "  energy: {reserve_cap: %d}\n", bound: 10_000},
		{key: "counter_supply.energy.savings_cap", yaml: energyMode + "  energy: {savings_cap: %d}\n", bound: 100_000},
		{key: "unlock.sets_per_unlock", yaml: "unlock: {sets_per_unlock: %d}\n", bound: 1_000},
		{key: "unlock.stages", yaml: "unlock: {stages: %d}\n", bound: 1_000},
		{key: "unlock.start_sets", yaml: "unlock: {start_sets: %d}\n", bound: 1_000},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			if _, err := config.Load(strings.NewReader(fmt.Sprintf(tc.yaml, tc.bound))); err != nil {
				t.Errorf("%s at its bound %d: %v, want it accepted", tc.key, tc.bound, err)
			}
			_, err := config.Load(strings.NewReader(fmt.Sprintf(tc.yaml, tc.bound+1)))
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Errorf("%s above its bound: error = %v, want one naming the key", tc.key, err)
			}
		})
	}
}
