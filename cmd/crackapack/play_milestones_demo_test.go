package main

import (
	"log/slog"
	"testing"
)

// The demo shows a milestone from the welcome boosters alone, so that a new demo has an earned goal,
// a locked copy and an energy reward to look at.
func TestPlayConfig_theDemoGameEarnsAMilestoneFromItsWelcomeBoosters(t *testing.T) {
	t.Parallel()
	game, _, err := startGame(demoGameArgs(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}

	for _, p := range game.Packs() {
		for range p.Count {
			if _, err := game.Open(p.SetCode); err != nil {
				t.Fatalf("Open(%q): %v", p.SetCode, err)
			}
		}
	}

	earned := game.TakeMilestones()
	if len(earned) == 0 {
		t.Fatal("the welcome boosters earned no milestone")
	}
	var paid int
	for _, e := range earned {
		paid += e.Energy
	}
	if paid == 0 {
		t.Errorf("the milestones %v paid no energy", earned)
	}
}
