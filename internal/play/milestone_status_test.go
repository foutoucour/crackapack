package play_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func TestMilestones_listsTheGoalsOfABinderWithTheirRewardAndWhetherTheyAreDone(t *testing.T) {
	t.Parallel()
	set := counterSet()
	g := milestoneGame(t, &memStore{}, milestoneRulesOn(set), set)

	before, err := g.Milestones("S")
	if err != nil {
		t.Fatalf("Milestones: %v", err)
	}
	mustOpen(t, g, "S")
	after, err := g.Milestones("S")
	if err != nil {
		t.Fatalf("Milestones: %v", err)
	}

	wantRewards := []int{5, 5, 0}
	if len(before) != 3 || len(after) != 3 {
		t.Fatalf("goals = %d before and %d after, want 3: the common, the colourless group and the binder", len(before), len(after))
	}
	for i := range before {
		if before[i].Done || !after[i].Done {
			t.Errorf("goal %v done = %v before and %v after, want false and true", before[i].Milestone, before[i].Done, after[i].Done)
		}
		if before[i].Reward != wantRewards[i] {
			t.Errorf("goal %v reward = %d, want %d", before[i].Milestone, before[i].Reward, wantRewards[i])
		}
	}
}

func TestMilestones_listsNoGoalInAGameWithoutRulesAndRefusesAnUnknownSet(t *testing.T) {
	t.Parallel()
	set := counterSet()
	plain := milestoneGame(t, &memStore{}, playtest.Rules(set), set)
	rich := milestoneGame(t, &memStore{}, milestoneRulesOn(set), set)

	if got, err := plain.Milestones("S"); err != nil || len(got) != 0 {
		t.Errorf("Milestones without rules = %v, %v; want none", got, err)
	}
	if _, err := rich.Milestones("NOPE"); !errors.Is(err, play.ErrUnknownSet) {
		t.Errorf("Milestones of an unknown set = %v, want ErrUnknownSet", err)
	}
}

func TestMilestones_theBinderShowsWhichCopyIsLocked(t *testing.T) {
	t.Parallel()
	set := counterSet()
	g := milestoneGame(t, &memStore{}, milestoneRulesOn(set), set)
	mustOpen(t, g, "S")

	binder, err := g.Binder("S")
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	for _, slot := range binder.Slots {
		if slot.RegularLocked != 1 || slot.FoilLocked != 0 {
			t.Errorf("%s locked %d regular and %d foil, want 1 and 0", slot.Card.ID, slot.RegularLocked, slot.FoilLocked)
		}
	}
}
