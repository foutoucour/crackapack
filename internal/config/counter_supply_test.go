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
