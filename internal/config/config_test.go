package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// TestLoad_defaults verifies that an empty YAML mapping yields a Config
// with all documented default values.
func TestLoad_defaults(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load({}): unexpected error: %v", err)
	}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"CoinRate", cfg.CoinRate, domain.CoinRate(100)},
		{"CounterSellPct", cfg.CounterSellPct, domain.Percent(60)},
		{"CounterBuyPct", cfg.CounterBuyPct, domain.Percent(120)},
		{"WelcomeBonus", cfg.WelcomeBonus, domain.Coins(200)},
		{"DailyReward", cfg.DailyReward, domain.Coins(100)},
		{"Energy.RechargePerTick", cfg.Energy.RechargePerTick, 1},
		{"Energy.ReserveCap", cfg.Energy.ReserveCap, 10},
		{"Energy.SavingsIntervalTicks", cfg.Energy.SavingsIntervalTicks, 3},
		{"Energy.SavingsCap", cfg.Energy.SavingsCap, 200},
		{"FallbackPrice", cfg.FallbackPrice, domain.USDCents(1)},
		{"SessionsPerDay", cfg.SessionsPerDay, 3},
		{"RunDays", cfg.RunDays, 90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}

	// Initial stock defaults.
	wantStock := map[domain.Rarity]int{
		domain.RarityCommon:   20,
		domain.RarityUncommon: 8,
		domain.RarityRare:     2,
		domain.RaritySpecial:  2,
		domain.RarityMythic:   2,
	}
	for r, want := range wantStock {
		t.Run("InitialStock/"+string(r), func(t *testing.T) {
			t.Parallel()
			if got := cfg.InitialStock[r]; got != want {
				t.Errorf("InitialStock[%s]: got %d, want %d", r, got, want)
			}
		})
	}

	// Default cost table.
	wantTable := []config.EnergyCost{
		{Threshold: 50, Cost: 3},
		{Threshold: 100, Cost: 5},
		{Threshold: 200, Cost: 8},
		{Threshold: 400, Cost: 12},
	}
	if len(cfg.Energy.CostTable) != len(wantTable) {
		t.Fatalf("CostTable len: got %d, want %d", len(cfg.Energy.CostTable), len(wantTable))
	}
	for i, want := range wantTable {
		got := cfg.Energy.CostTable[i]
		if got.Threshold != want.Threshold || got.Cost != want.Cost {
			t.Errorf("CostTable[%d]: got {%d %d}, want {%d %d}",
				i, got.Threshold, got.Cost, want.Threshold, want.Cost)
		}
	}
}

// TestLoad_overrides verifies that explicit YAML values override all defaults.
func TestLoad_overrides(t *testing.T) {
	t.Parallel()
	const yaml = `
coin_rate: 200
counter_sell_pct: 50
counter_buy_pct: 110
welcome_bonus: 500
daily_reward: 150
energy:
  recharge_per_tick: 2
  reserve_cap: 20
  savings_interval_ticks: 6
  savings_cap: 100
  cost_table:
    - threshold: 100
      cost: 4
    - threshold: 200
      cost: 9
initial_stock:
  common: 30
  uncommon: 10
  rare: 5
  special: 3
fallback_price: 5
sessions_per_day: 5
run_days: 30
seed: 42
targets:
  - metric: daily_reward_commons
    comparison: at_least
    min: 5
`
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if cfg.CoinRate != 200 {
		t.Errorf("CoinRate: got %d, want 200", cfg.CoinRate)
	}
	if cfg.CounterSellPct != 50 {
		t.Errorf("CounterSellPct: got %d, want 50", cfg.CounterSellPct)
	}
	if cfg.CounterBuyPct != 110 {
		t.Errorf("CounterBuyPct: got %d, want 110", cfg.CounterBuyPct)
	}
	if cfg.WelcomeBonus != 500 {
		t.Errorf("WelcomeBonus: got %d, want 500", cfg.WelcomeBonus)
	}
	if cfg.DailyReward != 150 {
		t.Errorf("DailyReward: got %d, want 150", cfg.DailyReward)
	}
	if cfg.Energy.RechargePerTick != 2 {
		t.Errorf("Energy.RechargePerTick: got %d, want 2", cfg.Energy.RechargePerTick)
	}
	if cfg.Energy.ReserveCap != 20 {
		t.Errorf("Energy.ReserveCap: got %d, want 20", cfg.Energy.ReserveCap)
	}
	if cfg.Energy.SavingsIntervalTicks != 6 {
		t.Errorf("Energy.SavingsIntervalTicks: got %d, want 6", cfg.Energy.SavingsIntervalTicks)
	}
	if cfg.Energy.SavingsCap != 100 {
		t.Errorf("Energy.SavingsCap: got %d, want 100", cfg.Energy.SavingsCap)
	}
	if len(cfg.Energy.CostTable) != 2 {
		t.Fatalf("CostTable len: got %d, want 2", len(cfg.Energy.CostTable))
	}
	if cfg.InitialStock[domain.RarityCommon] != 30 {
		t.Errorf("InitialStock[common]: got %d, want 30", cfg.InitialStock[domain.RarityCommon])
	}
	if cfg.FallbackPrice != 5 {
		t.Errorf("FallbackPrice: got %d, want 5", cfg.FallbackPrice)
	}
	if cfg.SessionsPerDay != 5 {
		t.Errorf("SessionsPerDay: got %d, want 5", cfg.SessionsPerDay)
	}
	if cfg.RunDays != 30 {
		t.Errorf("RunDays: got %d, want 30", cfg.RunDays)
	}
	if cfg.Seed != 42 {
		t.Errorf("Seed: got %d, want 42", cfg.Seed)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].Metric != "daily_reward_commons" {
		t.Errorf("Targets: got %v", cfg.Targets)
	}
}

// TestLoad_partialInitialStockMerges verifies that a partial initial_stock in YAML
// merges with the defaults: only specified rarities are overridden.
func TestLoad_partialInitialStockMerges(t *testing.T) {
	t.Parallel()
	const yaml = `
initial_stock:
  common: 50
`
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if got := cfg.InitialStock[domain.RarityCommon]; got != 50 {
		t.Errorf("common: got %d, want 50 (override)", got)
	}
	if got := cfg.InitialStock[domain.RarityUncommon]; got != 8 {
		t.Errorf("uncommon: got %d, want 8 (default kept)", got)
	}
	if got := cfg.InitialStock[domain.RarityRare]; got != 2 {
		t.Errorf("rare: got %d, want 2 (default kept)", got)
	}
	if got := cfg.InitialStock[domain.RaritySpecial]; got != 2 {
		t.Errorf("special: got %d, want 2 (default kept)", got)
	}
}

// TestLoad_errorCases covers invalid YAML inputs: unknown keys, type mismatches
// (which must name the key), and semantic validation failures (which name the key).
func TestLoad_errorCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		yaml        string
		wantKeyword string // substring that must appear in the error message
	}{
		// Unknown key: go.yaml.in/yaml/v3 with KnownFields already names the field.
		{
			name:        "unknown_key",
			yaml:        `foo: bar`,
			wantKeyword: "foo",
		},
		// Type mismatch on a top-level field: the enrichment must name the key.
		{
			name:        "type_mismatch_names_coin_rate",
			yaml:        "coin_rate: \"not-a-number\"",
			wantKeyword: "coin_rate",
		},
		{
			name:        "type_mismatch_names_counter_sell_pct",
			yaml:        "counter_sell_pct: \"bad\"",
			wantKeyword: "counter_sell_pct",
		},
		{
			name:        "type_mismatch_names_run_days",
			yaml:        "run_days: true",
			wantKeyword: "run_days",
		},
		// Semantic validation: coin_rate.
		{
			name:        "coin_rate_zero",
			yaml:        "coin_rate: 0",
			wantKeyword: "coin_rate",
		},
		// Semantic validation: counter percentages.
		{
			name:        "counter_sell_pct_zero",
			yaml:        "counter_sell_pct: 0",
			wantKeyword: "counter_sell_pct",
		},
		{
			name:        "counter_buy_pct_zero",
			yaml:        "counter_buy_pct: 0",
			wantKeyword: "counter_buy_pct",
		},
		{
			name:        "sell_above_buy",
			yaml:        "counter_sell_pct: 130\ncounter_buy_pct: 120",
			wantKeyword: "counter_sell_pct",
		},
		// Semantic validation: rewards.
		{
			name:        "negative_welcome_bonus",
			yaml:        "welcome_bonus: -1",
			wantKeyword: "welcome_bonus",
		},
		{
			name:        "negative_daily_reward",
			yaml:        "daily_reward: -1",
			wantKeyword: "daily_reward",
		},
		// Semantic validation: fallback_price.
		{
			name:        "negative_fallback_price",
			yaml:        "fallback_price: -1",
			wantKeyword: "fallback_price",
		},
		// Semantic validation: sessions_per_day.
		{
			name:        "sessions_per_day_zero",
			yaml:        "sessions_per_day: 0",
			wantKeyword: "sessions_per_day",
		},
		// Semantic validation: run_days.
		{
			name:        "run_days_zero",
			yaml:        "run_days: 0",
			wantKeyword: "run_days",
		},
		// Semantic validation: initial_stock.
		{
			name:        "negative_initial_stock",
			yaml:        "initial_stock:\n  common: -1",
			wantKeyword: "initial_stock",
		},
		// Semantic validation: energy block fields.
		{
			name:        "energy_recharge_per_tick_zero",
			yaml:        "energy:\n  recharge_per_tick: 0",
			wantKeyword: "energy.recharge_per_tick",
		},
		{
			name:        "energy_reserve_cap_zero",
			yaml:        "energy:\n  reserve_cap: 0",
			wantKeyword: "energy.reserve_cap",
		},
		{
			name:        "energy_savings_interval_ticks_zero",
			yaml:        "energy:\n  savings_interval_ticks: 0",
			wantKeyword: "energy.savings_interval_ticks",
		},
		{
			name:        "energy_savings_cap_negative",
			yaml:        "energy:\n  savings_cap: -1",
			wantKeyword: "energy.savings_cap",
		},
		{
			name:        "energy_cost_table_empty",
			yaml:        "energy:\n  cost_table: []",
			wantKeyword: "cost_table",
		},
		{
			name: "energy_cost_table_threshold_zero",
			yaml: `
energy:
  cost_table:
    - threshold: 0
      cost: 5
`,
			wantKeyword: "threshold",
		},
		{
			name: "energy_cost_table_cost_zero",
			yaml: `
energy:
  cost_table:
    - threshold: 100
      cost: 0
`,
			wantKeyword: "cost",
		},
		// Semantic validation: cost_table must be sorted ascending.
		{
			name: "cost_table_unsorted",
			yaml: `
energy:
  cost_table:
    - threshold: 200
      cost: 8
    - threshold: 100
      cost: 5
`,
			wantKeyword: "cost_table",
		},
		// Type mismatch inside a sequence item: full dotted path must appear.
		{
			name: "type_mismatch_in_target_seq_names_path",
			yaml: `
targets:
  - metric: supply
    comparison: at_most
    max: "bad"
`,
			wantKeyword: "targets[0].max",
		},
		{
			name: "type_mismatch_in_cost_table_seq_names_path",
			yaml: `
energy:
  cost_table:
    - threshold: "bad"
      cost: 5
`,
			wantKeyword: "energy.cost_table[0].threshold",
		},
		// CalibrationTarget: empty metric.
		{
			name: "target_empty_metric",
			yaml: `
targets:
  - metric: ""
    comparison: at_least
    min: 5
`,
			wantKeyword: "metric",
		},
		// CalibrationTarget: unknown comparison.
		{
			name: "target_unknown_comparison",
			yaml: `
targets:
  - metric: supply
    comparison: equals
`,
			wantKeyword: "comparison",
		},
		// CalibrationTarget: at_least requires min.
		{
			name: "at_least_missing_min",
			yaml: `
targets:
  - metric: supply
    comparison: at_least
`,
			wantKeyword: "min",
		},
		// CalibrationTarget: at_most requires max.
		{
			name: "at_most_missing_max",
			yaml: `
targets:
  - metric: supply
    comparison: at_most
`,
			wantKeyword: "max",
		},
		// CalibrationTarget: between requires both bounds.
		{
			name: "between_missing_min",
			yaml: `
targets:
  - metric: supply
    comparison: between
    max: 100
`,
			wantKeyword: "min",
		},
		{
			name: "between_missing_max",
			yaml: `
targets:
  - metric: supply
    comparison: between
    min: 10
`,
			wantKeyword: "max",
		},
		// CalibrationTarget: between requires min <= max.
		{
			name: "between_min_gt_max",
			yaml: `
targets:
  - metric: supply
    comparison: between
    min: 100
    max: 10
`,
			wantKeyword: "min",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tc.yaml))
			if err == nil {
				t.Fatalf("Load(%q): expected error, got nil", tc.yaml)
			}
			if !strings.Contains(err.Error(), tc.wantKeyword) {
				t.Errorf("Load(%q): error %q does not contain %q", tc.yaml, err.Error(), tc.wantKeyword)
			}
		})
	}
}

// TestLoad_emptyInput verifies that a completely empty reader is valid and
// returns a Config with all defaults.
func TestLoad_emptyInput(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Load(empty): unexpected error: %v", err)
	}
	if cfg.RunDays != 90 {
		t.Errorf("RunDays: got %d, want 90", cfg.RunDays)
	}
}

// TestLoad_targetBetweenValid verifies that a well-formed between target passes validation.
func TestLoad_targetBetweenValid(t *testing.T) {
	t.Parallel()
	const yaml = `
targets:
  - metric: days_to_complete_commons
    comparison: between
    min: 30
    max: 90
`
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("Targets len: got %d, want 1", len(cfg.Targets))
	}
	got := cfg.Targets[0]
	if got.Metric != "days_to_complete_commons" {
		t.Errorf("Metric: got %q, want %q", got.Metric, "days_to_complete_commons")
	}
	if got.Comparison != config.ComparisonBetween {
		t.Errorf("Comparison: got %q, want %q", got.Comparison, config.ComparisonBetween)
	}
	if got.Min == nil || *got.Min != 30 {
		t.Errorf("Min: got %v, want 30", got.Min)
	}
	if got.Max == nil || *got.Max != 90 {
		t.Errorf("Max: got %v, want 90", got.Max)
	}
}

func TestLoad_counterShelfDays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		want    int
		wantErr string
	}{
		{name: "default keeps cards forever", yaml: "", want: -1},
		{name: "zero is a sink", yaml: "counter_shelf_days: 0\n", want: 0},
		{name: "a number of days", yaml: "counter_shelf_days: 3\n", want: 3},
		{name: "explicit forever", yaml: "counter_shelf_days: -1\n", want: -1},
		{name: "below forever is rejected", yaml: "counter_shelf_days: -2\n", wantErr: "counter_shelf_days"},
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
			if cfg.CounterShelfDays != tc.want {
				t.Errorf("CounterShelfDays = %d, want %d", cfg.CounterShelfDays, tc.want)
			}
		})
	}
}
