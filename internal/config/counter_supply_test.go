package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

func TestLoad_counterSupply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		yaml       string
		wantMode   string
		wantPerDay int
		wantErr    string
	}{
		{name: "default supplies nothing", yaml: "", wantMode: "none"},
		{name: "fixed supply", yaml: "counter_supply:\n  mode: fixed\n  boosters_per_day: 20\n", wantMode: "fixed", wantPerDay: 20},
		{name: "unknown mode", yaml: "counter_supply:\n  mode: bogus\n", wantErr: "counter_supply.mode"},
		{name: "fixed supply needs boosters", yaml: "counter_supply:\n  mode: fixed\n  boosters_per_day: 0\n", wantErr: "counter_supply.boosters_per_day"},
		{name: "energy supply", yaml: "counter_supply:\n  mode: energy\n", wantMode: "energy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(strings.NewReader(tc.yaml))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one naming %s", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.CounterSupply.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", cfg.CounterSupply.Mode, tc.wantMode)
			}
			if cfg.CounterSupply.BoostersPerDay != tc.wantPerDay {
				t.Errorf("BoostersPerDay = %d, want %d", cfg.CounterSupply.BoostersPerDay, tc.wantPerDay)
			}
		})
	}
}

func TestLoad_counterSupplyEnergy(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(strings.NewReader("counter_supply:\n  mode: energy\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	e := cfg.CounterSupply.Energy
	if e.RechargePerTick != 20 || e.ReserveCap != 100 || e.SavingsIntervalTicks != 1 || e.SavingsCap != 1000 {
		t.Errorf("default energy = %+v, want 20 per tick, reserve 100, savings every tick up to 1000", e)
	}

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "recharge", yaml: "counter_supply:\n  mode: energy\n  energy: {recharge_per_tick: 0}\n", wantErr: "counter_supply.energy.recharge_per_tick"},
		{name: "reserve cap", yaml: "counter_supply:\n  mode: energy\n  energy: {reserve_cap: 0}\n", wantErr: "counter_supply.energy.reserve_cap"},
		{name: "savings interval", yaml: "counter_supply:\n  mode: energy\n  energy: {savings_interval_ticks: 0}\n", wantErr: "counter_supply.energy.savings_interval_ticks"},
		{name: "savings cap", yaml: "counter_supply:\n  mode: energy\n  energy: {savings_cap: -1}\n", wantErr: "counter_supply.energy.savings_cap"},
		{name: "a booster it could never afford", yaml: "counter_supply:\n  mode: energy\n  energy: {reserve_cap: 5, savings_cap: 0}\n", wantErr: "counter_supply.energy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := config.Load(strings.NewReader(tc.yaml)); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one naming %s", err, tc.wantErr)
			}
		})
	}
}

func TestLoad_counterBudget(t *testing.T) {
	t.Parallel()

	def, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Load defaults: %v", err)
	}
	if def.CounterBudget.Enabled || def.CounterBudget.Initial != 100000 || def.CounterBudget.DailyFloat != 0 {
		t.Errorf("default budget = %+v, want it off, with 100000 initial and no daily float", def.CounterBudget)
	}
	if def.CounterSupply.CoinsPerEnergy != 0 {
		t.Errorf("default coins_per_energy = %d, want 0", def.CounterSupply.CoinsPerEnergy)
	}

	cfg, err := config.Load(strings.NewReader("counter_budget:\n  enabled: true\n  initial: 500\n  daily_float: 20\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.CounterBudget.Enabled || cfg.CounterBudget.Initial != 500 || cfg.CounterBudget.DailyFloat != 20 {
		t.Errorf("budget = %+v, want it on, with 500 initial and 20 a day", cfg.CounterBudget)
	}

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "negative initial", yaml: "counter_budget:\n  enabled: true\n  initial: -1\n", wantErr: "counter_budget.initial"},
		{name: "negative float", yaml: "counter_budget:\n  enabled: true\n  daily_float: -1\n", wantErr: "counter_budget.daily_float"},
		{name: "negative coins per energy", yaml: "counter_supply:\n  mode: energy\n  coins_per_energy: -1\n", wantErr: "counter_supply.coins_per_energy"},
		{name: "coins per energy without a budget", yaml: "counter_supply:\n  mode: energy\n  coins_per_energy: 100\n", wantErr: "counter_supply.coins_per_energy"},
		{name: "coins per energy in fixed mode", yaml: "counter_budget:\n  enabled: true\ncounter_supply:\n  mode: fixed\n  boosters_per_day: 5\n  coins_per_energy: 100\n", wantErr: "counter_supply.coins_per_energy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := config.Load(strings.NewReader(tc.yaml)); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one naming %s", err, tc.wantErr)
			}
		})
	}

	ok := "counter_budget:\n  enabled: true\ncounter_supply:\n  mode: energy\n  coins_per_energy: 100\n"
	if _, err := config.Load(strings.NewReader(ok)); err != nil {
		t.Errorf("coins per energy with a budget and energy mode: %v", err)
	}
}
