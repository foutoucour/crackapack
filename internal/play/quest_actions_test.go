package play_test

import (
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// template is a quest of period that asks for target of action and gives reward, on cards worth at
// most ten coins.
func template(period domain.QuestPeriod, action domain.QuestAction, target int64, reward domain.QuestRewardKind) domain.QuestTemplate {
	return domain.QuestTemplate{Period: period, Action: action, Target: target, Reward: reward, MaxValue: 10, Weight: 1}
}

// firstQuest is the one quest of the board, which the test fails without.
func firstQuest(t *testing.T, g *play.Game) play.QuestView {
	t.Helper()
	board := mustQuests(t, g)
	switch {
	case len(board.Daily) > 0:
		return board.Daily[0]
	case len(board.Weekly) > 0:
		return board.Weekly[0]
	}
	t.Fatal("the board has no quest")
	return play.QuestView{}
}

// heldKeys lists the distinct cards that the latest save of store holds, in card order.
func heldKeys(t *testing.T, store *memStore) []domain.CardKey {
	t.Helper()
	var keys []domain.CardKey
	for _, c := range store.last(t).Collection {
		keys = append(keys, domain.CardKey{ID: c.ID, Finish: c.Finish})
	}
	return keys
}

func sellCopies(t *testing.T, g *play.Game, key domain.CardKey, qty int) {
	t.Helper()
	if _, err := g.Sell(key.ID, key.Finish, qty); err != nil {
		t.Fatalf("Sell(%v, %d): %v", key, qty, err)
	}
}

func TestQuests_cardsSoldAtTheCounterCount(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	g := questGame(t, store, playtest.NewClock(), dailyOnly(template(domain.QuestDaily, domain.QuestSellCards, 2, domain.RewardWinCard)), playtest.TinySet("S", 6, 2))
	mustOpen(t, g, "S")
	keys := heldKeys(t, store)

	sellCopies(t, g, keys[0], 1)
	if q := firstQuest(t, g); q.Progress != 1 || q.State != play.QuestOpen {
		t.Fatalf("after one sale the quest is %d/%d %v, want 1/2 open", q.Progress, q.Target, q.State)
	}
	sellCopies(t, g, keys[1], 1)

	q := firstQuest(t, g)
	if q.State != play.QuestDone || len(q.Reward.Got) != 1 || q.Reward.Got[0].Card.ID != q.Reward.Card.ID {
		t.Errorf("quest = %v got %+v, want it done with the card it named", q.State, q.Reward.Got)
	}
}

func TestQuests_aColourTakesOnlyTheCardsOfThatColour(t *testing.T) {
	t.Parallel()
	redSet := playtest.TinySet("S", 6, 2)
	for id, c := range redSet.Cards {
		c.Colors = []string{"R"}
		redSet.Cards[id] = c
	}
	sell := template(domain.QuestDaily, domain.QuestSellCards, 1, domain.RewardWinCard)
	sell.FilterBy = domain.QuestFilterColour

	for _, tc := range []struct {
		name   string
		colour domain.ColourGroup
		want   play.QuestState
	}{
		{"a red quest counts red cards", domain.ColourRed, play.QuestDone},
		{"a blue quest leaves them out", domain.ColourBlue, play.QuestOpen},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &memStore{}
			var g *play.Game
			for seed := uint64(1); ; seed++ {
				if seed > 200 {
					t.Fatalf("no seed drew a %s quest", tc.colour)
				}
				g = questGameOn(t, store, playtest.NewClock(), counterRules(), dailyOnly(sell), seed, redSet)
				if firstQuest(t, g).Filter.Colour == tc.colour {
					break
				}
				store = &memStore{}
			}
			mustOpen(t, g, "S")
			sellCopies(t, g, heldKeys(t, store)[0], 1)

			if q := firstQuest(t, g); q.State != tc.want {
				t.Errorf("quest is %v, want %v", q.State, tc.want)
			}
		})
	}
}

func TestQuests_cardsBoughtAtTheCounterCountAndWinAPremiumBooster(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	g := questGame(t, store, playtest.NewClock(), dailyOnly(template(domain.QuestDaily, domain.QuestBuyCards, 1, domain.RewardPremiumBooster)), counterSet())

	if _, err := g.Buy(cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("Buy: %v", err)
	}

	q := firstQuest(t, g)
	if q.State != play.QuestDone || len(q.Reward.Got) != 2 {
		t.Errorf("quest = %v with %d cards got, want it done with the 2 cards of a booster", q.State, len(q.Reward.Got))
	}
	total := 0
	for _, key := range heldKeys(t, store) {
		total += heldIn(t, store, key)
	}
	if total != 3 {
		t.Errorf("the player holds %d cards, want 3: the one bought and the booster's two", total)
	}
}

func TestQuests_aRewardIsNeitherASaleNorAPurchase(t *testing.T) {
	t.Parallel()
	rules := domain.QuestRules{DailyCount: 2, Templates: []domain.QuestTemplate{
		template(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardWinCard),
		template(domain.QuestDaily, domain.QuestBuyCards, 1, domain.RewardPremiumBooster),
	}}
	g := questGame(t, &memStore{}, playtest.NewClock(), rules, playtest.TinySet("S", 6, 2))

	mustOpen(t, g, "S")

	board := mustQuests(t, g)
	for _, q := range board.Daily {
		switch q.Action {
		case domain.QuestOpenBoosters:
			if q.State != play.QuestDone {
				t.Errorf("the booster quest is %v, want done", q.State)
			}
		case domain.QuestBuyCards:
			if q.Progress != 0 {
				t.Errorf("the buy quest is at %d, want 0: the card won is not bought", q.Progress)
			}
		}
	}
}

func TestQuests_coinsSpentAndWonAreCounted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		action domain.QuestAction
		target int64
		play   func(t *testing.T, g *play.Game)
	}{
		{"coins spent at the Counter", domain.QuestSpendCoins, 100, func(t *testing.T, g *play.Game) {
			t.Helper()
			if _, err := g.Buy(cheap().ID, cheap().Finish); err != nil { // 120 coins
				t.Fatalf("Buy: %v", err)
			}
		}},
		{"coins won at the Counter", domain.QuestWinCoins, 50, func(t *testing.T, g *play.Game) {
			t.Helper()
			mustOpen(t, g, "S")
			sellCopies(t, g, cheap(), 1) // 60 coins
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := questGame(t, &memStore{}, playtest.NewClock(), dailyOnly(template(domain.QuestDaily, tc.action, tc.target, domain.RewardPremiumBooster)), counterSet())

			tc.play(t, g)

			if q := firstQuest(t, g); q.State != play.QuestDone || q.Progress != tc.target {
				t.Errorf("quest is %d/%d %v, want done at its target", q.Progress, q.Target, q.State)
			}
		})
	}
}

func TestQuests_aDealOnTheMarketThatPaysEnoughWinsATrade(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	// The speculator pays 83 coins for the $1 card, see TestList_reservesTheCardUntilAnOfferSellsIt.
	trade := template(domain.QuestDaily, domain.QuestWinTrade, 80, domain.RewardPremiumBooster)
	g := questGameOn(t, store, clock, marketRules(), dailyOnly(trade), questSeed, counterSet())
	mustOpen(t, g, "S")
	mustOpen(t, g, "S")
	if _, err := g.List(cheap().ID, cheap().Finish, 100); err != nil {
		t.Fatalf("List: %v", err)
	}
	clock.Advance(time.Hour)
	offer := answerOf(t, mustMarket(t, g), specNPC)

	if _, err := g.AcceptAnswer(offer.ID); err != nil {
		t.Fatalf("AcceptAnswer: %v", err)
	}

	if q := firstQuest(t, g); q.State != play.QuestDone {
		t.Errorf("quest is %d/%d %v, want done by a deal of %d coins", q.Progress, q.Target, q.State, offer.Coins)
	}
}

func TestQuests_aSaleToTheCounterIsNotATrade(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	g := questGame(t, store, playtest.NewClock(), dailyOnly(template(domain.QuestDaily, domain.QuestWinTrade, 10, domain.RewardPremiumBooster)), counterSet())
	mustOpen(t, g, "S")

	sellCopies(t, g, cheap(), 1)

	if q := firstQuest(t, g); q.Progress != 0 {
		t.Errorf("trade quest progress = %d after a sale to the Counter, want 0", q.Progress)
	}
}

func TestQuests_aPremiumBoosterHoldsTheCardsOfASet(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	g := questGame(t, store, playtest.NewClock(), dailyOnly(template(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardPremiumBooster)), playtest.TinySet("S", 6, 2))

	mustOpen(t, g, "S")

	q := firstQuest(t, g)
	if q.State != play.QuestDone || len(q.Reward.Got) != 2 || q.Reward.SetCode != "S" {
		t.Errorf("quest = %v, got %d cards of %q, want done with 2 cards of S", q.State, len(q.Reward.Got), q.Reward.SetCode)
	}
}

func TestQuests_completingTheDailiesCompletesAWeeklyQuest(t *testing.T) {
	t.Parallel()
	rules := domain.QuestRules{DailyCount: 1, WeeklyCount: 1, Templates: []domain.QuestTemplate{
		template(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardWinCard),
		template(domain.QuestWeekly, domain.QuestCompleteDailies, 1, domain.RewardPremiumBooster),
	}}
	g := questGame(t, &memStore{}, playtest.NewClock(), rules, playtest.TinySet("S", 6, 2))

	mustOpen(t, g, "S")

	board := mustQuests(t, g)
	if board.Daily[0].State != play.QuestDone || board.Weekly[0].State != play.QuestDone {
		t.Errorf("daily is %v and weekly is %v, want both done", board.Daily[0].State, board.Weekly[0].State)
	}
}

func TestQuests_aWeeklyQuestLastsUntilMonday(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock() // Friday 2026-10-16 12:00 UTC
	weekly := domain.QuestRules{WeeklyCount: 1, Templates: []domain.QuestTemplate{template(domain.QuestWeekly, domain.QuestOpenBoosters, 3, domain.RewardPremiumBooster)}}
	g := questGame(t, &memStore{}, clock, weekly, playtest.TinySet("S", 6, 2))
	mustOpen(t, g, "S")

	clock.Advance(2 * day) // Sunday
	board := mustQuests(t, g)
	if q := board.Weekly[0]; q.Progress != 1 {
		t.Errorf("the weekly quest is at %d on the Sunday, want 1", q.Progress)
	}
	if want := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC); !board.WeeklyResets.Equal(want) {
		t.Errorf("WeeklyResets = %v, want the Monday %v", board.WeeklyResets, want)
	}

	clock.Advance(day) // Monday
	if q := mustQuests(t, g).Weekly[0]; q.Progress != 0 {
		t.Errorf("the weekly quest is at %d on the Monday, want a new one at 0", q.Progress)
	}
}
