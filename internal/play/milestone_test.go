package play_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The reward of a milestone is a share of the energy bar: with a reserve of 10, 50 pays 5.
func milestoneRules() *play.MilestoneRules {
	return &play.MilestoneRules{
		Rarity: map[domain.Rarity]domain.Percent{
			domain.RarityCommon: 50, domain.RarityUncommon: 100, domain.RarityRare: 200, domain.RarityMythic: 400,
		},
		Colour: 50,
	}
}

// milestoneGame starts a game on sets that earns milestones under milestones, with a Counter, and
// loads what store holds.
func milestoneGame(t *testing.T, store play.Store, rules play.Rules, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	stream := play.NewStream(1)
	g, err := play.NewGame(sets, nil, newOpenerOn(stream), rules, playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := g.OpenCounter(counterRules(), play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	if err := g.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	return g
}

func milestoneRulesOn(sets ...domain.SetCatalogue) play.Rules {
	rules := playtest.Rules(sets...)
	rules.Milestones = milestoneRules()
	return rules
}

func TestMilestones_aCompleteBinderEarnsEveryGoalItCompletesAndLocksTheCards(t *testing.T) {
	t.Parallel()
	g := milestoneGame(t, &memStore{}, milestoneRulesOn(counterSet()), counterSet())

	mustOpen(t, g, "S")

	got := g.TakeMilestones()
	want := []domain.Milestone{
		{Set: "S", Kind: domain.MilestoneRarity, Group: "common"},
		{Set: "S", Kind: domain.MilestoneColour, Group: "colourless"},
		{Set: "S", Kind: domain.MilestoneBinder},
	}
	var kinds []domain.Milestone
	for _, e := range got {
		kinds = append(kinds, e.Milestone)
	}
	if !slices.Equal(kinds, want) {
		t.Fatalf("earned = %v, want %v", kinds, want)
	}
	if got[0].Energy != 5 || got[1].Energy != 5 || got[2].Energy != 0 {
		t.Errorf("energy paid = %d, %d, %d; want 5, 5, 0: half a bar each for the common and the colour, none for the binder", got[0].Energy, got[1].Energy, got[2].Energy)
	}
	if got[0].SetName != "Set S" {
		t.Errorf("set name = %q, want %q", got[0].SetName, "Set S")
	}
	if status := g.Energy(); status.Savings != 10 || status.Reserve != 10 {
		t.Errorf("energy = %d in reserve, %d saved; want 10 and 10: a reward goes to the savings", status.Reserve, status.Savings)
	}
	for _, key := range []domain.CardKey{cheap(), dear()} {
		if g.Locked(key.ID, key.Finish) != 1 {
			t.Errorf("Locked(%v) = %d, want 1", key, g.Locked(key.ID, key.Finish))
		}
	}
	if again := g.TakeMilestones(); len(again) != 0 {
		t.Errorf("TakeMilestones again = %v, want none: the notice is told once", again)
	}
}

func TestMilestones_aMilestoneIsPaidOnce(t *testing.T) {
	t.Parallel()
	g := milestoneGame(t, &memStore{}, milestoneRulesOn(counterSet()), counterSet())
	mustOpen(t, g, "S")
	g.TakeMilestones()

	mustOpen(t, g, "S")

	if got := g.TakeMilestones(); len(got) != 0 {
		t.Errorf("second booster earned %v, want nothing: a milestone is earned once", got)
	}
	if saved := g.Energy().Savings; saved != 10 {
		t.Errorf("savings = %d, want 10", saved)
	}
}

func TestMilestones_aBinderNotCompleteEarnsNothing(t *testing.T) {
	t.Parallel()
	half := playtest.TinySet("S", 2, 1)
	g := milestoneGame(t, &memStore{}, milestoneRulesOn(half), half)

	mustOpen(t, g, "S")

	if got := g.TakeMilestones(); len(got) != 0 {
		t.Errorf("one card of two earned %v, want nothing", got)
	}
}

func TestMilestones_theRewardFollowsTheSizeOfTheBar(t *testing.T) {
	t.Parallel()
	set := counterSet()
	rules := milestoneRulesOn(set)
	rules.Energy.ReserveCap = 12
	g := milestoneGame(t, &memStore{}, rules, set)

	mustOpen(t, g, "S")

	if got := g.TakeMilestones(); got[0].Energy != 6 {
		t.Errorf("half a bar of 12 paid %d, want 6", got[0].Energy)
	}
}

func TestMilestones_theRewardTellsWhatTheSavingsTook(t *testing.T) {
	t.Parallel()
	set := counterSet()
	rules := milestoneRulesOn(set)
	rules.Energy.SavingsCap = 7
	g := milestoneGame(t, &memStore{}, rules, set)

	mustOpen(t, g, "S")

	got := g.TakeMilestones()
	if got[0].Energy != 5 || got[1].Energy != 2 {
		t.Errorf("energy paid = %d, %d; want 5 and 2: the savings stop at 7", got[0].Energy, got[1].Energy)
	}
}

func TestMilestones_aGameWithoutRulesEarnsNone(t *testing.T) {
	t.Parallel()
	set := counterSet()
	g := milestoneGame(t, &memStore{}, playtest.Rules(set), set)

	mustOpen(t, g, "S")

	if got := g.TakeMilestones(); len(got) != 0 {
		t.Errorf("earned %v without rules", got)
	}
	if g.Locked(cheap().ID, cheap().Finish) != 0 {
		t.Error("a card is locked in a game without milestones")
	}
}

func TestMilestones_theBinderUnlocksSetsOnTopOfTheNinetyPercentUnlock(t *testing.T) {
	t.Parallel()
	sets := ladder()
	rules := milestoneRulesOn(sets...)
	g, err := play.NewGame(sets, nil, newOpener(), rules, playtest.NewClock().Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Stage(play.StageRules{Threshold: 0.9, SetsPerUnlock: 1, Stages: 2, StartSets: 1}, play.NewStream(1)); err != nil {
		t.Fatal(err)
	}

	opening := mustOpen(t, g, "A")

	if len(opening.Unlocked) != 1 {
		t.Fatalf("the 90%% unlock opened %v, want one set", opening.Unlocked)
	}
	var binder play.EarnedMilestone
	for _, e := range g.TakeMilestones() {
		if e.Milestone.Kind == domain.MilestoneBinder {
			binder = e
		}
	}
	if len(binder.Unlocked) != 1 {
		t.Fatalf("the binder milestone opened %v, want one set", binder.Unlocked)
	}
	if binder.Unlocked[0].Code == opening.Unlocked[0].Code {
		t.Errorf("both unlocks opened %q: the second draw is of the sets still locked", binder.Unlocked[0].Code)
	}
	if got := locked(g); len(got) != 2 {
		t.Errorf("locked = %v, want 2 of the 4 sets after two unlocks", got)
	}
}

func TestMilestones_theLockedCopyCannotBeSold(t *testing.T) {
	t.Parallel()
	set := counterSet()
	g := milestoneGame(t, &memStore{}, milestoneRulesOn(set), set)
	mustOpen(t, g, "S")
	mustOpen(t, g, "S")

	_, err := g.Sell(cheap().ID, cheap().Finish, 2)
	if !errors.Is(err, play.ErrCopyLocked) {
		t.Fatalf("selling both copies = %v, want ErrCopyLocked", err)
	}
	if _, err := g.Sell(cheap().ID, cheap().Finish, 1); err != nil {
		t.Fatalf("selling the copy above the lock: %v", err)
	}

	if _, err := g.Sell(cheap().ID, cheap().Finish, 1); !errors.Is(err, play.ErrCopyLocked) {
		t.Errorf("selling the last copy = %v, want ErrCopyLocked", err)
	}
	if got := held(g, "S", cheap()); got != 1 {
		t.Errorf("copies held = %d, want the locked one", got)
	}
}

func TestMilestones_aLoadedGameKeepsTheLocksAndPaysNothingTwice(t *testing.T) {
	t.Parallel()
	set := counterSet()
	store := &memStore{}
	g := milestoneGame(t, store, milestoneRulesOn(set), set)
	mustOpen(t, g, "S")

	loaded := milestoneGame(t, store.copied(), milestoneRulesOn(set), set)

	if loaded.Locked(cheap().ID, cheap().Finish) != 1 || loaded.Locked(dear().ID, dear().Finish) != 1 {
		t.Error("a loaded game lost the locks")
	}
	if got := loaded.TakeMilestones(); len(got) != 0 {
		t.Errorf("a loaded game told %v again", got)
	}
	if saved := loaded.Energy().Savings; saved != 10 {
		t.Errorf("savings = %d, want 10: the rewards were paid before the save", saved)
	}
}

func TestMilestones_aSaveFromBeforeMilestonesEarnsThemOnLoad(t *testing.T) {
	t.Parallel()
	set := counterSet()
	store := &memStore{}
	before := milestoneGame(t, store, playtest.Rules(set), set)
	mustOpen(t, before, "S")

	loaded := milestoneGame(t, store.copied(), milestoneRulesOn(set), set)

	if got := loaded.TakeMilestones(); len(got) != 3 {
		t.Errorf("a save that holds a complete binder earned %d milestones on load, want 3", len(got))
	}
	if loaded.Locked(cheap().ID, cheap().Finish) != 1 {
		t.Error("the cards of a milestone earned on load are not locked")
	}
}
