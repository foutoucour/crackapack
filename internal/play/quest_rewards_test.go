package play_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// foilSet is TinySet whose cards exist as foils too.
func foilSet(code string, n, perBooster int) domain.SetCatalogue {
	cat := playtest.TinySet(code, n, perBooster)
	for id, c := range cat.Cards {
		c.Finishes = []domain.Finish{domain.FinishNonFoil, domain.FinishFoil}
		cat.Cards[id] = c
	}
	return cat
}

func TestQuests_aWeeklyQuestUnlocksTheSetItNames(t *testing.T) {
	t.Parallel()
	sets := []domain.SetCatalogue{playtest.TinySet("A", 2, 1), playtest.TinySet("B", 3, 1), playtest.TinySet("C", 4, 1), playtest.TinySet("D", 5, 1)}
	g, stream, err := newSeededGame(playtest.NewClock(), sets...)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Stage(play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, play.NewStream(3)); err != nil {
		t.Fatal(err)
	}
	if err := g.OpenCounter(counterRules(), play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatal(err)
	}
	unlock := domain.QuestRules{WeeklyCount: 1, Templates: []domain.QuestTemplate{template(domain.QuestWeekly, domain.QuestOpenBoosters, 1, domain.RewardUnlockSet)}}
	if err := g.OpenQuests(unlock, play.NewStream(questSeed)); err != nil {
		t.Fatal(err)
	}
	if err := g.Persist(play.Saves{Store: &memStore{}, Stream: stream}); err != nil {
		t.Fatal(err)
	}
	named := mustQuests(t, g).Weekly[0].Reward.SetCode
	if !slices.Contains(locked(g), named) {
		t.Fatalf("the quest names %q, which is open already: locked sets are %v", named, locked(g))
	}

	first := g.Packs()[0].SetCode
	mustOpen(t, g, first)

	if slices.Contains(locked(g), named) {
		t.Errorf("%q is still locked after the quest was done: locked sets are %v", named, locked(g))
	}
	if q := mustQuests(t, g).Weekly[0]; q.State != play.QuestDone {
		t.Errorf("quest is %v, want done", q.State)
	}
}

func TestQuests_aGameWithoutStagesCannotDrawAnUnlock(t *testing.T) {
	t.Parallel()
	unlock := domain.QuestRules{WeeklyCount: 1, Templates: []domain.QuestTemplate{template(domain.QuestWeekly, domain.QuestOpenBoosters, 1, domain.RewardUnlockSet)}}
	g := questGame(t, &memStore{}, playtest.NewClock(), unlock, playtest.TinySet("S", 6, 2))

	if board := mustQuests(t, g); len(board.Weekly) != 0 {
		t.Errorf("board has %d weekly quests, want none: every set is open already", len(board.Weekly))
	}
}

func TestQuests_doublingTheDuplicatesKeepsAPlaysetAndDoublesTheRest(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock() // Friday 2026-10-16 12:00 UTC
	store := &memStore{}
	double := domain.QuestRules{WeeklyCount: 1, Templates: []domain.QuestTemplate{template(domain.QuestWeekly, domain.QuestOpenBoosters, 7, domain.RewardDoubleDuplicates)}}
	g := questGame(t, store, clock, double, counterSet())
	// Nothing is owned yet, so no set can be named: the next week's board names one.
	mustOpen(t, g, "S")
	if got := len(mustQuests(t, g).Weekly); got != 0 {
		t.Fatalf("the first board has %d weekly quests, want 0: the player owns no card of a set", got)
	}
	clock.Advance(3 * day) // Monday

	if q := firstQuest(t, g); q.Reward.SetCode != "S" || q.Reward.Kind != domain.RewardDoubleDuplicates {
		t.Fatalf("quest = %+v, want a doubling of the duplicates of S", q.Reward)
	}
	for range 6 {
		mustOpen(t, g, "S")
	}
	// Seven copies of each card: three above a playset.
	if got := held(g, "S", cheap()); got != 7 {
		t.Fatalf("copies before the last booster = %d, want 7", got)
	}
	mustOpen(t, g, "S")

	// Eight copies, four above a playset, doubled: 4 + 4*2.
	if got := held(g, "S", cheap()); got != 12 {
		t.Errorf("copies after the reward = %d, want 12", got)
	}
	if q := firstQuest(t, g); q.State != play.QuestDone {
		t.Errorf("quest is %v, want done", q.State)
	}
}

func TestQuests_aCardIsTurnedIntoAFoil(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		sell  func(named, other domain.CardKey) domain.CardKey
		state play.QuestState
	}{
		{"the card named is still there", func(_, other domain.CardKey) domain.CardKey { return other }, play.QuestDone},
		{"the card named was sold meanwhile", func(named, _ domain.CardKey) domain.CardKey { return named }, play.QuestLost},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clock := playtest.NewClock()
			store := &memStore{}
			foil := template(domain.QuestDaily, domain.QuestWinCoins, 1, domain.RewardFoilCard)
			foil.MaxValue = 1000
			g := questGame(t, store, clock, dailyOnly(foil), foilSet("S", 6, 2))
			mustOpen(t, g, "S") // two cards, so that one can be sold and the other named
			clock.Advance(day)  // the board is drawn again, with cards to name

			named := domain.CardKey{ID: firstQuest(t, g).Reward.Card.ID, Finish: domain.FinishNonFoil}
			var other domain.CardKey
			for _, key := range heldKeys(t, store) {
				if key.ID != named.ID {
					other = key
				}
			}
			sold := tc.sell(named, other)
			copiesBefore := heldIn(t, store, named)
			sellCopies(t, g, sold, heldIn(t, store, sold))

			q := firstQuest(t, g)
			if q.State != tc.state {
				t.Fatalf("quest is %v, want %v", q.State, tc.state)
			}
			foils := heldIn(t, store, domain.CardKey{ID: named.ID, Finish: domain.FinishFoil})
			wantFoils := map[play.QuestState]int{play.QuestDone: 1, play.QuestLost: 0}[tc.state]
			if foils != wantFoils {
				t.Errorf("foil copies of the card named = %d, want %d", foils, wantFoils)
			}
			if tc.state == play.QuestDone && heldIn(t, store, named) != copiesBefore-1 {
				t.Errorf("non-foil copies = %d, want %d: one became the foil", heldIn(t, store, named), copiesBefore-1)
			}
		})
	}
}

// swapGame is a game on a set of one card worth $1 and a set of three cheap ones, with the player
// holding five copies of the dear card, one above a playset, and one card of the cheap set. The
// board, drawn the next day, swaps one spare copy for a card of the cheap set.
func swapGame(t *testing.T) (*play.Game, *memStore) {
	t.Helper()
	clock := playtest.NewClock()
	store := &memStore{}
	swap := template(domain.QuestDaily, domain.QuestWinCoins, 1, domain.RewardSwap)
	g := questGame(t, store, clock, dailyOnly(swap), playtest.TinySet("S", 1, 1), playtest.TinySet("T", 3, 1))
	for range 5 {
		mustOpen(t, g, "S")
	}
	mustOpen(t, g, "T")
	clock.Advance(day)
	return g, store
}

func TestQuests_aSwapGivesTheSpareCopiesForTheCardNamed(t *testing.T) {
	t.Parallel()
	g, store := swapGame(t)
	q := firstQuest(t, g)
	if q.Reward.Kind != domain.RewardSwap || len(q.Reward.Give) != 1 || q.Reward.Give[0].Qty != 1 || q.Reward.Give[0].Card.ID != "S-00" || q.Reward.Card.ID[0] != 'T' {
		t.Fatalf("reward = %+v, want one spare S-00 for a card of T", q.Reward)
	}
	won := domain.CardKey{ID: q.Reward.Card.ID, Finish: domain.FinishNonFoil}
	var cheapOne domain.CardKey
	for _, key := range heldKeys(t, store) {
		if key.ID[0] == 'T' {
			cheapOne = key
		}
	}

	sellCopies(t, g, cheapOne, 1) // wins a coin, which finishes the quest

	if q := firstQuest(t, g); q.State != play.QuestDone {
		t.Fatalf("quest is %v, want done", q.State)
	}
	if got := heldIn(t, store, domain.CardKey{ID: "S-00", Finish: domain.FinishNonFoil}); got != 4 {
		t.Errorf("copies of S-00 = %d, want 4: the spare one was given", got)
	}
	if got := heldIn(t, store, won); got != 1 {
		t.Errorf("copies of the card named = %d, want 1", got)
	}
}

func TestQuests_aSwapIsLostWhenTheSpareCopiesAreGone(t *testing.T) {
	t.Parallel()
	g, store := swapGame(t)
	won := domain.CardKey{ID: firstQuest(t, g).Reward.Card.ID, Finish: domain.FinishNonFoil}

	sellCopies(t, g, domain.CardKey{ID: "S-00", Finish: domain.FinishNonFoil}, 1) // the spare copy

	if q := firstQuest(t, g); q.State != play.QuestLost {
		t.Errorf("quest is %v, want lost", q.State)
	}
	if got := heldIn(t, store, won); got != 0 {
		t.Errorf("copies of the card named = %d, want 0: nothing was given", got)
	}
	if got := heldIn(t, store, domain.CardKey{ID: "S-00", Finish: domain.FinishNonFoil}); got != 4 {
		t.Errorf("copies of S-00 = %d, want 4: only the sold one left", got)
	}
}
