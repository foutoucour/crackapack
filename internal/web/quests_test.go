package web_test

import (
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func questTemplate(period domain.QuestPeriod, action domain.QuestAction, target int64, reward domain.QuestRewardKind) domain.QuestTemplate {
	return domain.QuestTemplate{Period: period, Action: action, Target: target, Reward: reward, MaxValue: 10, Weight: 1}
}

// newQuestShop serves a game with a Counter on sets and the quests of rules.
func newQuestShop(t *testing.T, rules domain.QuestRules, sets ...domain.SetCatalogue) counterShop {
	t.Helper()
	clock := playtest.NewClock()
	game, err := play.NewGame(sets, nil, newOpener(), playtest.Rules(sets...), clock.Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := game.OpenCounter(counterRules(), play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	if err := game.OpenQuests(rules, play.NewStream(9)); err != nil {
		t.Fatalf("OpenQuests: %v", err)
	}
	srv, c, _ := serve(t, game, fakeImages{})
	return counterShop{srv: srv, c: c, game: game, clock: clock}
}

func TestQuests_thePageSaysWhatToDoAndWhatItGives(t *testing.T) {
	t.Parallel()
	rules := domain.QuestRules{DailyCount: 1, WeeklyCount: 1, Templates: []domain.QuestTemplate{
		questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardPremiumBooster),
		questTemplate(domain.QuestWeekly, domain.QuestCompleteDailies, 5, domain.RewardPremiumBooster),
	}}
	shop := newQuestShop(t, rules, counterSet())

	body := shop.get(t, "/quests")

	mustContain(t, "the quests page", body,
		"<h1>Quests</h1>",
		"Open 2 boosters of Set S",
		"Finish 5 daily quests",
		"A premium booster of Set S",
		`<progress value="0" max="2"`,
		"Quests updated in 12 hours",
		"Quests updated in 2 days 12 hours",
	)
}

func TestQuests_thePageSaysHowLongIsLeftBeforeTheQuestsUpdate(t *testing.T) {
	t.Parallel()
	rules := domain.QuestRules{DailyCount: 1, WeeklyCount: 1, Templates: []domain.QuestTemplate{
		questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardPremiumBooster),
		questTemplate(domain.QuestWeekly, domain.QuestCompleteDailies, 5, domain.RewardPremiumBooster),
	}}
	shop := newQuestShop(t, rules, counterSet())
	shop.clock.Advance(8*time.Hour + 40*time.Minute) // Friday 20:40 UTC

	body := shop.get(t, "/quests")

	mustContain(t, "the quests page", body, "Quests updated in 3 hours 20 mins", "Quests updated in 2 days 3 hours 20 mins")
}

func TestQuests_theWordingNamesEveryAction(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		action domain.QuestAction
		target int64
		filter domain.QuestFilterKind
		want   string
	}{
		{"sell", domain.QuestSellCards, 6, domain.QuestFilterNone, "Sell or trade 6 cards"},
		{"buy", domain.QuestBuyCards, 1, domain.QuestFilterNone, "Buy or trade for 1 card"},
		{"trade", domain.QuestWinTrade, 150, domain.QuestFilterNone, "Win a deal on the market worth ◎150 or more"},
		{"spend", domain.QuestSpendCoins, 500, domain.QuestFilterNone, "Spend ◎500"},
		{"win", domain.QuestWinCoins, 400, domain.QuestFilterNone, "Win ◎400"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tpl := questTemplate(domain.QuestDaily, tc.action, tc.target, domain.RewardPremiumBooster)
			shop := newQuestShop(t, domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}, counterSet())

			mustContain(t, "the quests page", shop.get(t, "/quests"), tc.want)
		})
	}
}

func TestQuests_aFinishedQuestSaysWhatItGave(t *testing.T) {
	t.Parallel()
	tpl := questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardWinCard)
	shop := newQuestShop(t, domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}, playtest.TinySet("S", 6, 2))
	shop.open(t, 1)

	body := shop.get(t, "/quests")

	mustContain(t, "the quests page", body, `<span class="tag done">Done</span>`, "You got", `<a href="/collection?set=S&amp;focus=S-`)
	mustContain(t, "the quests page", body, `<progress value="1" max="1"`)
}

func TestQuests_aLostQuestSaysWhy(t *testing.T) {
	t.Parallel()
	swapSet := []domain.SetCatalogue{playtest.TinySet("S", 1, 1), playtest.TinySet("T", 3, 1)}
	tpl := questTemplate(domain.QuestDaily, domain.QuestWinCoins, 1, domain.RewardSwap)
	shop := newQuestShop(t, domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}, swapSet...)
	for range 5 {
		if _, err := shop.game.Open("S"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := shop.game.Open("T"); err != nil {
		t.Fatal(err)
	}
	shop.clock.Advance(24 * time.Hour) // the next day: a swap can be drawn
	mustContain(t, "the quests page", shop.get(t, "/quests"), "Swap 1 × S-00")
	if _, err := shop.game.Sell("S-00", domain.FinishNonFoil, 1); err != nil {
		t.Fatalf("Sell: %v", err)
	}

	body := shop.get(t, "/quests")

	mustContain(t, "the quests page", body, `<span class="tag lost">Lost</span>`, "the copies it needed were gone")
}

func TestQuests_aBoardWithNoQuestSaysSo(t *testing.T) {
	t.Parallel()
	shop := newQuestShop(t, domain.QuestRules{}, counterSet())

	mustContain(t, "the quests page", shop.get(t, "/quests"), "No daily quest today", "No weekly quest this week")
}

func TestQuests_theHeaderLinksToThePageOfAGameThatHasQuests(t *testing.T) {
	t.Parallel()
	with := newQuestShop(t, domain.QuestRules{}, counterSet())
	without := newCounterShop(t)

	mustContain(t, "the home page of a game with quests", with.get(t, "/"), `<a href="/quests">Quests</a>`)
	mustNotContain(t, "the home page of a game without quests", without.get(t, "/"), `href="/quests"`)
}

func TestQuests_aGameWithoutQuestsHasNoPage(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	resp, body := get(t, shop.c, shop.srv.URL+"/quests")

	if resp.StatusCode != 404 {
		t.Errorf("GET /quests = %d, want 404:\n%s", resp.StatusCode, body)
	}
}

func TestQuests_aCardNameIsEscaped(t *testing.T) {
	t.Parallel()
	set := playtest.TinySet("S", 6, 2)
	for id, c := range set.Cards {
		c.Name = "<script>alert(1)</script>"
		set.Cards[id] = c
	}
	tpl := questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardWinCard)
	shop := newQuestShop(t, domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}, set)

	body := shop.get(t, "/quests")

	if strings.Contains(body, "<script>alert(1)") {
		t.Errorf("the quests page writes a card name unescaped:\n%s", body)
	}
	mustContain(t, "the quests page", body, "&lt;script&gt;alert(1)&lt;/script&gt;")
}

func TestQuests_theCardOfAnOpenQuestLinksToItsSlotInTheBinder(t *testing.T) {
	t.Parallel()
	tpl := questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardWinCard)
	shop := newQuestShop(t, domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}, playtest.TinySet("S", 6, 2))

	body := shop.get(t, "/quests")

	mustContain(t, "the quests page", body, `<a href="/collection?set=S&amp;focus=S-`, "in its binder")
}

func TestQuests_aQuestDoneIsAnnouncedOnTheNextPageWithALinkToTheCardItGave(t *testing.T) {
	t.Parallel()
	tpl := questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, 1, domain.RewardWinCard)
	shop := newQuestShop(t, domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}, playtest.TinySet("S", 6, 2))
	shop.open(t, 1)

	first := shop.get(t, "/")
	second := shop.get(t, "/")

	mustContain(t, "the page after the quest was done", first, `role="status"`, "Quest done: Open 1 booster of Set S", `<a href="/collection?set=S&amp;focus=S-`)
	mustNotContain(t, "the next page", second, "Quest done")
}

func TestQuests_aQuestThatIsNotDoneIsNotAnnounced(t *testing.T) {
	t.Parallel()
	tpl := questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, 2, domain.RewardWinCard)
	shop := newQuestShop(t, domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}, playtest.TinySet("S", 6, 2))
	shop.open(t, 1)

	mustNotContain(t, "the page", shop.get(t, "/"), "Quest done")
}
