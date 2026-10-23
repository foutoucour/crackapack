package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

func TestLoad_unlockDefaultsAndOverride(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yaml string
		want config.UnlockConfig
	}{
		{name: "defaults", yaml: "", want: config.UnlockConfig{BinderThreshold: 0.9, SetsPerUnlock: 3, Stages: 4, StartSets: 5}},
		{
			name: "a partial block keeps the other defaults",
			yaml: "unlock: {binder_threshold: 0.25, start_sets: 1}",
			want: config.UnlockConfig{BinderThreshold: 0.25, SetsPerUnlock: 3, Stages: 4, StartSets: 1},
		},
		{name: "a whole binder", yaml: "unlock: {binder_threshold: 1}", want: config.UnlockConfig{BinderThreshold: 1, SetsPerUnlock: 3, Stages: 4, StartSets: 5}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(strings.NewReader(tc.yaml))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Unlock != tc.want {
				t.Errorf("Unlock = %+v, want %+v", cfg.Unlock, tc.want)
			}
		})
	}
}

func TestLoad_unlockRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		yaml    string
		wantKey string
	}{
		{"a threshold of zero", "unlock: {binder_threshold: 0}", "unlock.binder_threshold"},
		{"a threshold above one", "unlock: {binder_threshold: 1.5}", "unlock.binder_threshold"},
		{"no set per unlock", "unlock: {sets_per_unlock: 0}", "unlock.sets_per_unlock"},
		{"no stage", "unlock: {stages: 0}", "unlock.stages"},
		{"no start set", "unlock: {start_sets: 0}", "unlock.start_sets"},
		{"a fraction of a stage", "unlock: {stages: 2.5}", "unlock.stages"},
		{"an unknown key", "unlock: {threshold: 0.5}", "threshold"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.wantKey) {
				t.Errorf("Load(%q) error = %v, want one naming %s", tc.yaml, err, tc.wantKey)
			}
		})
	}
}
