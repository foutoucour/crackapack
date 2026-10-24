package play_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func TestPersist_theQuestsGoOnAfterAReload(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	rules := dailyOnly(template(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardWinCard))
	g := questGame(t, store, clock, rules, playtest.TinySet("S", 6, 2))
	before := firstQuest(t, g)
	mustOpen(t, g, "S")

	reloaded := questGame(t, store.copied(), clock, rules, playtest.TinySet("S", 6, 2))

	q := firstQuest(t, reloaded)
	if q.Progress != 1 || q.State != play.QuestOpen || q.Reward.Card.ID != before.Reward.Card.ID || q.SetCode != before.SetCode {
		t.Errorf("the reloaded quest = %+v, want the same one at 1/2 with the card %s", q, before.Reward.Card.ID)
	}
}

func TestPersist_aRewardIsGivenOnce(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	rules := dailyOnly(template(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardPremiumBooster))
	g := questGame(t, store, clock, rules, playtest.TinySet("S", 6, 2))
	mustOpen(t, g, "S")
	cards := func(s *memStore) int {
		n := 0
		for _, c := range s.last(t).Collection {
			n += c.Quantity
		}
		return n
	}
	want := cards(store)

	reloaded := questGame(t, store.copied(), clock, rules, playtest.TinySet("S", 6, 2))

	q := firstQuest(t, reloaded)
	if q.State != play.QuestDone || len(q.Reward.Got) != 2 {
		t.Errorf("the reloaded quest is %v with %d cards got, want it done with the 2 cards of its booster", q.State, len(q.Reward.Got))
	}
	if _, err := reloaded.Quests(); err != nil {
		t.Fatalf("Quests: %v", err)
	}
	if got := cards(store); got != want {
		t.Errorf("cards held = %d, want %d: a reload must not give the reward again", got, want)
	}
}

func TestPersist_aReloadIsNotWhatThePlayerBoughtOrSold(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	rules := dailyOnly(template(domain.QuestDaily, domain.QuestBuyCards, 1, domain.RewardPremiumBooster))
	g := questGame(t, store, clock, rules, playtest.TinySet("S", 6, 2))
	mustOpen(t, g, "S")
	mustOpen(t, g, "S")

	reloaded := questGame(t, store.copied(), clock, rules, playtest.TinySet("S", 6, 2))

	if q := firstQuest(t, reloaded); q.Progress != 0 {
		t.Errorf("progress = %d after a reload with a full collection, want 0: nothing was bought", q.Progress)
	}
}

func TestPersist_theNextBoardFollowsTheSavedStream(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	rules := domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{
		template(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardWinCard),
		template(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardWinCard),
		template(domain.QuestDaily, domain.QuestOpenBoosters, 3, domain.RewardWinCard),
	}}
	sets := []domain.SetCatalogue{playtest.TinySet("S", 6, 2), playtest.TinySet("T", 6, 2)}
	g := questGame(t, store, clock, rules, sets...)
	mustQuests(t, g)
	reloaded := questGame(t, store.copied(), clock, rules, sets...)

	clock.Advance(day)
	want := firstQuest(t, g)
	got := firstQuest(t, reloaded)

	if got.Target != want.Target || got.SetCode != want.SetCode || got.Reward.Card.ID != want.Reward.Card.ID {
		t.Errorf("the reloaded game draws %+v, the first game %+v: the stream was not kept", got, want)
	}
}

func TestPersist_aSaveFromBeforeQuestsGetsTheBoardOfAGameWithQuests(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	old := counterGame(t, store, clock, counterRules(), playtest.TinySet("S", 6, 2))
	mustOpen(t, old, "S")

	g := questGame(t, store.copied(), clock, dailyOnly(template(domain.QuestDaily, domain.QuestBuyCards, 1, domain.RewardPremiumBooster)), playtest.TinySet("S", 6, 2))

	board := mustQuests(t, g)
	if len(board.Daily) != 1 || board.Daily[0].Progress != 0 {
		t.Errorf("board = %+v, want one quest at 0: what the save holds was not bought", board.Daily)
	}
}

func TestPersist_aQuestOfACardNoCatalogueHasIsDropped(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	rules := dailyOnly(template(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardWinCard))
	g := questGame(t, store, clock, rules, playtest.TinySet("S", 6, 2))
	mustQuests(t, g)
	snap := store.last(t)
	snap.Quests.Daily[0].Reward.Card = "gone"
	edited := &memStore{snap: snap, found: true}

	reloaded := questGame(t, edited, clock, rules, playtest.TinySet("S", 6, 2))

	if board := mustQuests(t, reloaded); len(board.Daily) != 0 {
		t.Errorf("board = %+v, want the quest of a card the game lost dropped", board.Daily)
	}
}

func TestPersist_aQuestOfAnUnknownActionIsRefused(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	rules := dailyOnly(template(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardWinCard))
	g := questGame(t, store, clock, rules, playtest.TinySet("S", 6, 2))
	mustQuests(t, g)
	snap := store.last(t)
	snap.Quests.Daily[0].Action = "dance"

	loaded, stream, err := newSeededGame(clock, playtest.TinySet("S", 6, 2))
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.OpenCounter(counterRules(), play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatal(err)
	}
	if err := loaded.OpenQuests(rules, play.NewStream(questSeed)); err != nil {
		t.Fatal(err)
	}
	if err := loaded.Persist(play.Saves{Store: &memStore{snap: snap, found: true}, Stream: stream}); err == nil {
		t.Error("Persist loaded a save whose quest has an action that is none")
	}
}
