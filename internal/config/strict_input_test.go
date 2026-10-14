package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

func TestLoad_rejectsInputThatWasAcceptedSilently(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "a null item in a sequence", yaml: "energy:\n  cost_table:\n    - null\n    - {threshold: 400, cost: 12}\n", wantErr: "energy.cost_table[0]"},
		{name: "a table of only a null", yaml: "energy:\n  cost_table: [null]\n", wantErr: "energy.cost_table[0]"},
		{name: "a float in an integer field", yaml: "coin_rate: 1.5\n", wantErr: "coin_rate"},
		{name: "a float with no fractional part", yaml: "daily_reward: 100.0\n", wantErr: "daily_reward"},
		{name: "a float in a nested integer field", yaml: "counter_budget:\n  enabled: true\ncounter_supply:\n  mode: energy\n  coins_per_energy: 2.5\n", wantErr: "counter_supply.coins_per_energy"},
		{name: "a float in a sequence of integers", yaml: "sweep_rates: [50, 100.5]\n", wantErr: "sweep_rates[1]"},
		{name: "an unknown rarity in initial_stock", yaml: "initial_stock: {comon: 5}\n", wantErr: "initial_stock"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one naming %s", err, tc.wantErr)
			}
		})
	}
}

func TestLoad_stillAcceptsFloatsWhereTheyBelong(t *testing.T) {
	t.Parallel()
	const yaml = "targets:\n  - {metric: sub_coin_cards, comparison: between, min: 0.5, max: 5.5}\n"

	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Errorf("a float target bound: %v, want it accepted", err)
	}
}

func TestLoad_counterEnergyReserveMustHoldTheDearestTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "savings do not make up for a small reserve", yaml: "counter_supply:\n  mode: energy\n  energy: {reserve_cap: 6, savings_cap: 6}\n", wantErr: "counter_supply.energy.reserve_cap"},
		{name: "reserve exactly the dearest cost", yaml: "counter_supply:\n  mode: energy\n  energy: {reserve_cap: 12, savings_cap: 0}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tc.yaml))
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("error = %v, want it accepted", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "12") {
				t.Errorf("error = %v, want one naming %s and the dearest cost 12", err, tc.wantErr)
			}
		})
	}
}
