package main

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestPlayRules_milestonesPayWhatTheConfigSays(t *testing.T) {
	t.Parallel()
	sets, prices := costSets()
	cfg := costConfig(t, "milestones: {rare_pct: 300, colour_pct: 25}\n")

	rules, err := playRules(cfg, sets, map[string]int{"A": 5, "B": 5}, prices)

	if err != nil {
		t.Fatalf("playRules: %v", err)
	}
	if rules.Milestones == nil {
		t.Fatal("milestones = nil, want the rules of the config")
	}
	if got := rules.Milestones.Rarity[domain.RarityRare]; got != 300 {
		t.Errorf("rare reward = %d, want 300", got)
	}
	if got := rules.Milestones.Rarity[domain.RarityCommon]; got != 50 {
		t.Errorf("common reward = %d, want the default 50", got)
	}
	if rules.Milestones.Colour != 25 {
		t.Errorf("colour reward = %d, want 25", rules.Milestones.Colour)
	}
}
