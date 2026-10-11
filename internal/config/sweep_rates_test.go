package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// TestConfig_sweepRates_defaults verifies the default grid is [10,25,50,100,250,500].
func TestConfig_sweepRates_defaults(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []domain.CoinRate{10, 25, 50, 100, 250, 500}
	if !reflect.DeepEqual(cfg.SweepRates, want) {
		t.Errorf("SweepRates: got %v, want %v", cfg.SweepRates, want)
	}
}

// TestConfig_sweepRates_validOverride verifies a custom grid is accepted.
func TestConfig_sweepRates_validOverride(t *testing.T) {
	t.Parallel()
	const yaml = "sweep_rates: [1, 5, 20]\n"
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []domain.CoinRate{1, 5, 20}
	if !reflect.DeepEqual(cfg.SweepRates, want) {
		t.Errorf("SweepRates: got %v, want %v", cfg.SweepRates, want)
	}
}

// TestConfig_sweepRates_invalid is a table-driven test for all validation errors.
func TestConfig_sweepRates_invalid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		yaml    string
		wantMsg string
	}{
		{
			name:    "empty",
			yaml:    "sweep_rates: []\n",
			wantMsg: "sweep_rates must not be empty",
		},
		{
			name:    "zero rate",
			yaml:    "sweep_rates: [0, 50]\n",
			wantMsg: "sweep_rates[0]",
		},
		{
			name:    "negative rate",
			yaml:    "sweep_rates: [10, -5]\n",
			wantMsg: "sweep_rates[1]",
		},
		{
			name:    "duplicate",
			yaml:    "sweep_rates: [10, 50, 50]\n",
			wantMsg: "sweep_rates[2]",
		},
		{
			name:    "descending",
			yaml:    "sweep_rates: [100, 50]\n",
			wantMsg: "sweep_rates[1]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tc.yaml))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantMsg)
			}
		})
	}
}
