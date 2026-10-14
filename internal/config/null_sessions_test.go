package config_test

// Tests for defects D and E:
//   D – sessions_per_day above 144 silently collapses
//   E – A YAML null wipes defaults

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// ---------------------------------------------------------------------------
// D – sessions_per_day must be <= domain.TicksPerDay
// ---------------------------------------------------------------------------

// TestLoad_sessionsPerDay_boundary verifies that sessions_per_day == TicksPerDay
// (144) is accepted.
func TestLoad_sessionsPerDay_boundary(t *testing.T) {
	t.Parallel()
	const yaml = "sessions_per_day: 144\n"
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error at boundary sessions_per_day=%d: %v", domain.TicksPerDay, err)
	}
	if cfg.SessionsPerDay != domain.TicksPerDay {
		t.Errorf("SessionsPerDay = %d, want %d", cfg.SessionsPerDay, domain.TicksPerDay)
	}
}

// TestLoad_sessionsPerDay_aboveBound verifies that sessions_per_day == 145
// is rejected and the error names sessions_per_day.
func TestLoad_sessionsPerDay_aboveBound(t *testing.T) {
	t.Parallel()
	const yaml = "sessions_per_day: 145\n"
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatalf("Load: expected error for sessions_per_day=145 (> %d), got nil", domain.TicksPerDay)
	}
	if !strings.Contains(err.Error(), "sessions_per_day") {
		t.Errorf("error %q does not contain %q", err.Error(), "sessions_per_day")
	}
}

// ---------------------------------------------------------------------------
// E – YAML null is treated as absent (default applies)
// ---------------------------------------------------------------------------

// TestLoad_null_initialStock verifies that initial_stock: null keeps all
// rarity defaults instead of yielding an empty map.
func TestLoad_null_initialStock(t *testing.T) {
	t.Parallel()
	const yaml = "initial_stock: null\n"
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	wantStock := map[domain.Rarity]int{
		domain.RarityCommon:   20,
		domain.RarityUncommon: 8,
		domain.RarityRare:     2,
		domain.RaritySpecial:  2,
	}
	for r, want := range wantStock {
		if got := cfg.InitialStock[r]; got != want {
			t.Errorf("InitialStock[%s] = %d, want %d (default)", r, got, want)
		}
	}
}

// TestLoad_null_energy verifies that energy: null keeps all energy defaults.
func TestLoad_null_energy(t *testing.T) {
	t.Parallel()
	const yaml = "energy: null\n"
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Energy.RechargePerTick != 1 {
		t.Errorf("Energy.RechargePerTick = %d, want 1 (default)", cfg.Energy.RechargePerTick)
	}
	if cfg.Energy.ReserveCap != 10 {
		t.Errorf("Energy.ReserveCap = %d, want 10 (default)", cfg.Energy.ReserveCap)
	}
	if cfg.Energy.SavingsCap != 200 {
		t.Errorf("Energy.SavingsCap = %d, want 200 (default)", cfg.Energy.SavingsCap)
	}
	if len(cfg.Energy.CostTable) != 4 {
		t.Errorf("Energy.CostTable len = %d, want 4 (default)", len(cfg.Energy.CostTable))
	}
}

// TestLoad_null_energySubKey verifies that a null sub-key inside energy keeps
// its default while other specified sub-keys are applied.
func TestLoad_null_energySubKey(t *testing.T) {
	t.Parallel()
	const yaml = `
energy:
  savings_cap: null
  recharge_per_tick: 2
`
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Energy.SavingsCap != 200 {
		t.Errorf("Energy.SavingsCap = %d, want 200 (default; null key kept default)", cfg.Energy.SavingsCap)
	}
	if cfg.Energy.RechargePerTick != 2 {
		t.Errorf("Energy.RechargePerTick = %d, want 2 (override applied)", cfg.Energy.RechargePerTick)
	}
}

// TestLoad_null_costTable verifies that cost_table: null keeps the default table.
func TestLoad_null_costTable(t *testing.T) {
	t.Parallel()
	const yaml = "energy:\n  cost_table: null\n"
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if len(cfg.Energy.CostTable) != 4 {
		t.Errorf("Energy.CostTable len = %d, want 4 (default)", len(cfg.Energy.CostTable))
	}
}

// TestLoad_null_sweepRates verifies that sweep_rates: null keeps the default grid.
func TestLoad_null_sweepRates(t *testing.T) {
	t.Parallel()
	const yaml = "sweep_rates: null\n"
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	want := []domain.CoinRate{10, 25, 50, 100, 250, 500}
	if len(cfg.SweepRates) != len(want) {
		t.Fatalf("SweepRates len = %d, want %d (default)", len(cfg.SweepRates), len(want))
	}
	for i, w := range want {
		if cfg.SweepRates[i] != w {
			t.Errorf("SweepRates[%d] = %d, want %d", i, cfg.SweepRates[i], w)
		}
	}
}

// TestLoad_null_targets verifies that targets: null keeps the default (no targets).
func TestLoad_null_targets(t *testing.T) {
	t.Parallel()
	const yaml = "targets: null\n"
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if len(cfg.Targets) != 0 {
		t.Errorf("Targets len = %d, want 0 (default)", len(cfg.Targets))
	}
}

// TestLoad_null_partialInitialStock verifies that a partial initial_stock with
// some null entries keeps defaults for the null ones and applies the others.
func TestLoad_null_partialInitialStock(t *testing.T) {
	t.Parallel()
	const yaml = `
initial_stock:
  common: null
  uncommon: 15
`
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if got := cfg.InitialStock[domain.RarityCommon]; got != 20 {
		t.Errorf("InitialStock[common] = %d, want 20 (default; null key)", got)
	}
	if got := cfg.InitialStock[domain.RarityUncommon]; got != 15 {
		t.Errorf("InitialStock[uncommon] = %d, want 15 (override)", got)
	}
	if got := cfg.InitialStock[domain.RarityRare]; got != 2 {
		t.Errorf("InitialStock[rare] = %d, want 2 (default)", got)
	}
}

// TestLoad_null_listItemIsRejected verifies that a null item in a sequence is an error naming its
// path. The decoder would skip it silently, so "targets: [null]" would pass as "no targets" and a
// null inside cost_table would drop a tier unnoticed, see ADR-0017.
func TestLoad_null_listItemIsRejected(t *testing.T) {
	t.Parallel()
	const yaml = `
targets:
  - null
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil || !strings.Contains(err.Error(), "targets[0]") {
		t.Fatalf("error = %v, want one naming targets[0]", err)
	}
}
