package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

func TestLoad_milestonesDefaultsAndOverride(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yaml string
		want config.MilestonesConfig
	}{
		{name: "defaults", yaml: "", want: config.MilestonesConfig{CommonPct: 50, UncommonPct: 100, RarePct: 200, MythicPct: 400, ColourPct: 50}},
		{
			name: "a partial block keeps the other defaults",
			yaml: "milestones: {mythic_pct: 800}",
			want: config.MilestonesConfig{CommonPct: 50, UncommonPct: 100, RarePct: 200, MythicPct: 800, ColourPct: 50},
		},
		{
			name: "a reward of zero turns one off",
			yaml: "milestones: {colour_pct: 0}",
			want: config.MilestonesConfig{CommonPct: 50, UncommonPct: 100, RarePct: 200, MythicPct: 400, ColourPct: 0},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(strings.NewReader(tc.yaml))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Milestones != tc.want {
				t.Errorf("Milestones = %+v, want %+v", cfg.Milestones, tc.want)
			}
		})
	}
}

func TestLoad_milestonesRejectARewardBelowZero(t *testing.T) {
	t.Parallel()
	_, err := config.Load(strings.NewReader("milestones: {rare_pct: -1}"))
	if err == nil || !strings.Contains(err.Error(), "milestones.rare_pct") {
		t.Errorf("error = %v, want one naming milestones.rare_pct", err)
	}
}
