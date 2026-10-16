package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

func TestEnergyConfig_CostTiersAreTheCostTableInOrder(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := cfg.Energy.CostTiers()

	want := []domain.CostTier{{Threshold: 50, Cost: 3}, {Threshold: 100, Cost: 5}, {Threshold: 200, Cost: 8}, {Threshold: 400, Cost: 12}}
	if len(got) != len(want) {
		t.Fatalf("CostTiers() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tier %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestEnergyConfig_EnergyParamsAreTheDocumentedRecharge(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := cfg.Energy.Params()

	// +1 every tick up to 10, then one savings point every 3 ticks up to 200, see
	// docs/core-features.md#energy.
	want := domain.EnergyParams{RechargePerTick: 1, ReserveCap: 10, SavingsIntervalTicks: 3, SavingsCap: 200}
	if got != want {
		t.Errorf("Params() = %+v, want %+v", got, want)
	}
}
