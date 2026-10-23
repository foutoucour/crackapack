package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

func TestLoad_catchUpDays(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		yaml    string
		want    int
		wantErr bool
	}{
		{name: "default", yaml: "", want: 30},
		{name: "override", yaml: "catch_up_days: 7", want: 7},
		{name: "no day", yaml: "catch_up_days: 0", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(strings.NewReader(tc.yaml))
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "catch_up_days") {
					t.Errorf("Load error = %v, want one naming catch_up_days", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.CatchUpDays != tc.want {
				t.Errorf("CatchUpDays = %d, want %d", cfg.CatchUpDays, tc.want)
			}
		})
	}
}

// The Counter's parameters come from the config the same way for the simulation and the game.
func TestConfig_CounterParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		yaml  string
		shelf domain.ShelfLife
	}{
		{name: "cards stay forever by default", yaml: "", shelf: domain.ForeverOnShelf()},
		{name: "a sink", yaml: "counter_shelf_days: 0", shelf: domain.VanishAfterDays(0)},
		{name: "a shelf life", yaml: "counter_shelf_days: 3\ncounter_budget: {enabled: true}", shelf: domain.VanishAfterDays(3)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(strings.NewReader(tc.yaml))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			p := cfg.CounterParams()
			if p.ShelfLife != tc.shelf {
				t.Errorf("ShelfLife = %+v, want %+v", p.ShelfLife, tc.shelf)
			}
			if p.SellPct != 60 || p.BuyPct != 120 || p.FoilPct != 500 || p.CoinRate != 100 || p.FallbackPrice != 1 || p.InitialStock[domain.RarityRare] != 2 {
				t.Errorf("CounterParams = %+v, want the defaults", p)
			}
			if p.Treasury != cfg.CounterBudget.Enabled {
				t.Errorf("Treasury = %v, want %v", p.Treasury, cfg.CounterBudget.Enabled)
			}
		})
	}
}

// The NPC archetypes become the same domain specs for the simulation and the game. Their trade
// settings apply only when trades are on, so a crowd that does not trade behaves as it did before
// trades existed.
func TestNPCConfig_ArchetypeSpecs(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	casual := cfg.NPCs.Archetypes["casual"]
	for _, trades := range []bool{false, true} {
		spec := cfg.NPCs.ArchetypeSpecs(trades)["casual"]
		if spec.Share != casual.Share || spec.MinBoostersPerDay != casual.MinBoostersPerDay || spec.MaxBoostersPerDay != casual.MaxBoostersPerDay || spec.KeepCopies != casual.KeepCopies {
			t.Errorf("trades %v: supply of casual = %+v, want that of %+v", trades, spec, casual)
		}
		if got := spec.Behaviour != ""; got != trades {
			t.Errorf("trades %v: casual has the behaviour %q", trades, spec.Behaviour)
		}
	}
	if got := cfg.NPCs.ArchetypeSpecs(true)["ego"].ResponseDelay; got != 24*time.Hour {
		t.Errorf("response delay of ego = %v, want 24h", got)
	}
	if got := cfg.NPCs.ArchetypeSpecs(false)["ego"].ResponseDelay; got != 0 {
		t.Errorf("response delay of ego without trades = %v, want 0", got)
	}
	if got := cfg.NPCs.ArchetypeSpecs(true)["ego"].Traits; len(got) != 1 || got[0].Name != "ego" || got[0].Params["max_retries"] != 3 {
		t.Errorf("traits of ego = %+v, want ego with its params", got)
	}
}

func TestCounterEnergyConfig_Params(t *testing.T) {
	t.Parallel()
	e := config.CounterEnergyConfig{RechargePerTick: 1, ReserveCap: 2, SavingsIntervalTicks: 3, SavingsCap: 4}
	want := domain.EnergyParams{RechargePerTick: 1, ReserveCap: 2, SavingsIntervalTicks: 3, SavingsCap: 4}
	if got := e.Params(); got != want {
		t.Errorf("Params = %+v, want %+v", got, want)
	}
}
