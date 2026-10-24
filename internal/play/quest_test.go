package play_test

import (
	"errors"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// openTemplate asks for two boosters of a set and gives a card worth at most 10 coins.
func openTemplate() domain.QuestTemplate {
	return domain.QuestTemplate{
		Period: domain.QuestDaily, Action: domain.QuestOpenBoosters, Target: 2,
		Reward: domain.RewardWinCard, MaxValue: 10, Weight: 1,
	}
}

// dailyOnly is one daily quest drawn from t.
func dailyOnly(t domain.QuestTemplate) domain.QuestRules {
	return domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{t}}
}

// questGame starts a game on sets with a Counter, and quests under rules, on clock, and loads what
// store holds.
func questGame(t *testing.T, store play.Store, clock *playtest.Clock, rules domain.QuestRules, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	return questGameOn(t, store, clock, counterRules(), rules, questSeed, sets...)
}

// questSeed is the seed of the random stream that draws the quests of questGame.
const questSeed = 9

// questGameOn is questGame with the Counter, and the market and the NPCs, of counter, and quests drawn
// from the stream of seed.
func questGameOn(t *testing.T, store play.Store, clock *playtest.Clock, counter play.CounterRules, rules domain.QuestRules, seed uint64, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	g, stream, err := newSeededGame(clock, sets...)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := g.OpenCounter(counter, play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	if err := g.OpenQuests(rules, play.NewStream(seed)); err != nil {
		t.Fatalf("OpenQuests: %v", err)
	}
	if err := g.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	return g
}

func mustQuests(t *testing.T, g *play.Game) play.QuestBoard {
	t.Helper()
	board, err := g.Quests()
	if err != nil {
		t.Fatalf("Quests: %v", err)
	}
	return board
}

// heldIn is the copies of key that the latest save of store holds.
func heldIn(t *testing.T, store *memStore, key domain.CardKey) int {
	t.Helper()
	for _, c := range store.last(t).Collection {
		if c.ID == key.ID && c.Finish == key.Finish {
			return c.Quantity
		}
	}
	return 0
}

func copiesIn(op play.Opening, id domain.CardID) int {
	n := 0
	for _, c := range op.Cards {
		if c.Card.ID == id {
			n++
		}
	}
	return n
}

func TestQuests_aGameWithoutQuestsHasNoBoard(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), playtest.TinySet("S", 6, 2))
	if _, err := g.Quests(); !errors.Is(err, play.ErrNoQuests) {
		t.Errorf("Quests() error = %v, want ErrNoQuests", err)
	}
}

func TestOpenQuests_needsACounter(t *testing.T) {
	t.Parallel()
	g, _ := newGameWithClock(t, playtest.TinySet("S", 6, 2))
	if err := g.OpenQuests(dailyOnly(openTemplate()), play.NewStream(9)); !errors.Is(err, play.ErrNoCounter) {
		t.Errorf("OpenQuests error = %v, want ErrNoCounter", err)
	}
}

func TestOpenQuests_refusesRulesOutOfRange(t *testing.T) {
	t.Parallel()
	g, stream, err := newSeededGame(playtest.NewClock(), playtest.TinySet("S", 6, 2))
	if err != nil {
		t.Fatal(err)
	}
	_ = stream
	if err := g.OpenCounter(counterRules(), play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatal(err)
	}
	bad := dailyOnly(openTemplate())
	bad.Templates[0].Target = 0
	if err := g.OpenQuests(bad, play.NewStream(9)); err == nil {
		t.Error("OpenQuests accepted a template with a target of zero")
	}
}

func TestQuests_theBoardNamesTheSetAndTheCardOfTheReward(t *testing.T) {
	t.Parallel()
	g := questGame(t, &memStore{}, playtest.NewClock(), dailyOnly(openTemplate()), playtest.TinySet("S", 6, 2))

	board := mustQuests(t, g)

	if len(board.Daily) != 1 || len(board.Weekly) != 0 {
		t.Fatalf("board has %d daily and %d weekly quests, want 1 and 0", len(board.Daily), len(board.Weekly))
	}
	q := board.Daily[0]
	if q.Action != domain.QuestOpenBoosters || q.SetCode != "S" || q.Target != 2 || q.Progress != 0 || q.State != play.QuestOpen {
		t.Errorf("quest = %+v, want an open quest for 2 boosters of S at 0", q)
	}
	if q.Reward.Kind != domain.RewardWinCard || q.Reward.Card.ID == "" {
		t.Errorf("reward = %+v, want a named card to win", q.Reward)
	}
}

func TestQuests_boostersOfTheNamedSetFinishTheQuestAndWinTheCard(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	g := questGame(t, store, playtest.NewClock(), dailyOnly(openTemplate()), playtest.TinySet("S", 6, 2))
	won := domain.CardKey{ID: mustQuests(t, g).Daily[0].Reward.Card.ID, Finish: domain.FinishNonFoil}

	if _, err := g.Open("S"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if q := mustQuests(t, g).Daily[0]; q.Progress != 1 || q.State != play.QuestOpen {
		t.Fatalf("after one booster the quest is %d/%d %v, want 1/2 open", q.Progress, q.Target, q.State)
	}
	before := heldIn(t, store, won)
	second, err := g.Open("S")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	q := mustQuests(t, g).Daily[0]
	if q.Progress != 2 || q.State != play.QuestDone {
		t.Errorf("quest is %d/%d %v, want 2/2 done", q.Progress, q.Target, q.State)
	}
	if got, want := heldIn(t, store, won), before+copiesIn(second, won.ID)+1; got != want {
		t.Errorf("copies of the card won = %d, want %d: the booster's own copies and the reward", got, want)
	}
}

func TestQuests_aBoosterOfAnotherSetDoesNotCount(t *testing.T) {
	t.Parallel()
	g := questGame(t, &memStore{}, playtest.NewClock(), dailyOnly(openTemplate()), playtest.TinySet("S", 6, 2), playtest.TinySet("T", 6, 2))
	named := mustQuests(t, g).Daily[0].SetCode
	other := map[string]string{"S": "T", "T": "S"}[named]

	if _, err := g.Open(other); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if q := mustQuests(t, g).Daily[0]; q.Progress != 0 {
		t.Errorf("progress = %d after a booster of %s, want 0 for a quest on %s", q.Progress, other, named)
	}
}

func TestQuests_aBoosterIsNotABoughtCard(t *testing.T) {
	t.Parallel()
	buy := domain.QuestTemplate{Period: domain.QuestDaily, Action: domain.QuestBuyCards, Target: 1, Reward: domain.RewardPremiumBooster, Weight: 1}
	g := questGame(t, &memStore{}, playtest.NewClock(), dailyOnly(buy), playtest.TinySet("S", 6, 2))

	if _, err := g.Open("S"); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if q := mustQuests(t, g).Daily[0]; q.Progress != 0 {
		t.Errorf("progress = %d after opening a booster, want 0: its cards were not bought", q.Progress)
	}
}

func TestQuests_theBoardRenewsAtMidnightUTC(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock() // 2026-10-16 12:00 UTC, a Friday
	g := questGame(t, &memStore{}, clock, dailyOnly(openTemplate()), playtest.TinySet("S", 6, 2))
	if _, err := g.Open("S"); err != nil {
		t.Fatalf("Open: %v", err)
	}

	clock.Advance(11 * time.Hour) // 23:00, the same day
	if q := mustQuests(t, g).Daily[0]; q.Progress != 1 {
		t.Errorf("progress = %d before midnight, want 1", q.Progress)
	}
	clock.Advance(2 * time.Hour) // 01:00 the next day

	board := mustQuests(t, g)
	if q := board.Daily[0]; q.Progress != 0 || q.State != play.QuestOpen {
		t.Errorf("the new day's quest is %d/%d %v, want a fresh 0/2 open", q.Progress, q.Target, q.State)
	}
	if want := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC); !board.DailyResets.Equal(want) {
		t.Errorf("DailyResets = %v, want %v, the midnight after the new day", board.DailyResets, want)
	}
}
