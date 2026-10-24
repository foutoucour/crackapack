package sim_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// marketConfig is a crowd that keeps its surplus on a shelf and answers the player after
// responseDelayHours, with the Exchange off: the player trades on the market, as in the game.
func marketConfig(population, responseDelayHours int) config.Config {
	cfg := tradesConfig(population)
	cfg.Trades.Enabled = false
	a := cfg.NPCs.Archetypes["npc"]
	a.ResponseDelayHours = responseDelayHours
	cfg.NPCs.Archetypes["npc"] = a
	return cfg
}

// marketScript runs script in every session, and tells the engine that the player uses the
// market.
type marketScript struct{ script func(sim.Session) }

func (marketScript) UsesMarket() bool { return true }

func (p marketScript) RunSession(s sim.Session) { p.script(s) }

// askForEverything asks around for every card of the catalogues, which refuses the cards no NPC
// holds.
func askForEverything(s sim.Session) {
	for _, cat := range s.View().Catalogues {
		for id := range cat.Cards {
			_, _ = s.MarketAsk(domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
		}
	}
}

func TestSession_marketIsDisabledWithoutAMarketPlayer(t *testing.T) {
	t.Parallel()
	var ran bool
	var askErr, listErr, acceptErr error
	var answers []domain.MarketAnswer
	policy := scriptPolicy{ran: &ran, script: func(s sim.Session) {
		_, askErr = s.MarketAsk(domain.CardKey{ID: "x", Finish: domain.FinishNonFoil})
		_, listErr = s.MarketList(domain.CardKey{ID: "x", Finish: domain.FinishNonFoil}, 5)
		acceptErr = s.MarketAccept(1)
		answers = s.MarketAnswers()
	}}

	runResult(t, marketConfig(5, 0), policy)

	if !ran {
		t.Fatal("the policy never ran")
	}
	for name, err := range map[string]error{"ask": askErr, "list": listErr, "accept": acceptErr} {
		if !errors.Is(err, sim.ErrMarketDisabled) {
			t.Errorf("market %s = %v, want ErrMarketDisabled", name, err)
		}
	}
	if len(answers) != 0 {
		t.Errorf("MarketAnswers = %v, want none", answers)
	}
}

func TestSession_marketAnswersArriveAfterTheDelayOfTheArchetype(t *testing.T) {
	t.Parallel()
	const delayHours = 24
	cfg := marketConfig(8, delayHours)
	cfg.RunDays = 12
	// firstAsked is the day the player first got an answer pending, firstAnswered the day an answer
	// reached it.
	firstAsked, firstAnswered := 0, 0
	policy := marketScript{script: func(s sim.Session) {
		day := s.View().Day
		askForEverything(s)
		if firstAsked == 0 && day > 1 {
			firstAsked = day
		}
		if firstAnswered == 0 && len(s.MarketAnswers()) > 0 {
			firstAnswered = day
		}
	}}

	runResult(t, cfg, policy)

	if firstAnswered == 0 {
		t.Fatal("no NPC ever answered over 12 days, want the answers of a delay of one day")
	}
	if firstAnswered < 2 {
		t.Errorf("the first answer came on day %d, want after the delay of %d hours", firstAnswered, delayHours)
	}
}

func TestSession_acceptingAnAnswerGivesTheCardForCoins(t *testing.T) {
	t.Parallel()
	cfg := marketConfig(8, 0)
	cfg.RunDays = 10
	var accepted domain.MarketAnswer
	var balanceBefore, balanceAfter domain.Coins
	var heldBefore, heldAfter int
	policy := marketScript{script: func(s sim.Session) {
		askForEverything(s)
		if accepted.ID != 0 {
			return
		}
		for _, a := range s.MarketAnswers() {
			v := s.View()
			if a.Kind != domain.AnswerAsk || a.Coins > v.Balance {
				continue
			}
			balanceBefore, heldBefore = v.Balance, v.OwnedCards[a.Card]
			if err := s.MarketAccept(a.ID); err != nil {
				t.Errorf("MarketAccept(%d) = %v", a.ID, err)
				return
			}
			after := s.View()
			accepted, balanceAfter, heldAfter = a, after.Balance, after.OwnedCards[a.Card]
			return
		}
	}}

	runResult(t, cfg, policy)

	if accepted.ID == 0 {
		t.Fatal("the player never had an answer it could pay for")
	}
	if heldAfter != heldBefore+1 {
		t.Errorf("copies held = %d, want %d: the NPC's card goes to the player", heldAfter, heldBefore+1)
	}
	if balanceBefore-balanceAfter != accepted.Coins {
		t.Errorf("coins paid = %d, want the asking price %d", balanceBefore-balanceAfter, accepted.Coins)
	}
}

func TestSession_aListedCopyIsNotSoldToTheCounter(t *testing.T) {
	t.Parallel()
	cfg := marketConfig(8, 24)
	cfg.RunDays = 4
	var listed bool
	var heldAfterSale int
	copiesOf := func(v sim.WorldView, id domain.CardID) int {
		return v.OwnedCards[nonFoil(id)] + v.OwnedCards[domain.CardKey{ID: id, Finish: domain.FinishFoil}]
	}
	policy := marketScript{script: func(s sim.Session) {
		if listed {
			return
		}
		v := s.View()
		var duplicates []domain.CardKey
		for key, qty := range v.OwnedCards {
			if qty >= 2 && key.Finish == domain.FinishNonFoil {
				duplicates = append(duplicates, key)
			}
		}
		slices.SortFunc(duplicates, func(a, b domain.CardKey) int { return compareIDs(a.ID, b.ID) })
		for _, key := range duplicates {
			if _, err := s.MarketList(key, 1000); err != nil {
				continue
			}
			listed = true
			s.SellDuplicates(1)
			heldAfterSale = copiesOf(s.View(), key.ID)
			return
		}
		openAll(s)
	}}

	runResult(t, cfg, policy)

	if !listed {
		t.Fatal("the player never listed a duplicate")
	}
	if heldAfterSale < 2 {
		t.Errorf("copies held after selling = %d, want at least 2: one is kept for the binder and one is listed", heldAfterSale)
	}
}

func TestRun_marketTraderDealsOnTheMarketAndReportsIt(t *testing.T) {
	t.Parallel()
	cfg := marketConfig(40, 0)
	cfg.NPCs.SurplusHoldDays = 10
	cfg.RunDays = 20

	result := runResult(t, cfg, sim.NewMarketTraderPolicy())

	// The test sets have two rares, which the Counter sells to the player before it needs to ask
	// around, so the number of requests is only reported.
	if _, ok := result.Summary["market_asks"]; !ok {
		t.Error("summary has no market_asks")
	}
	for _, key := range []string{"market_listings", "market_deals"} {
		if result.Summary[key] <= 0 {
			t.Errorf("%s = %v, want a market that was used over 20 days", key, result.Summary[key])
		}
	}
}

// firstDaySold is the cards the crowd sent to the Counter on the first day of a collector's run.
func firstDaySold(t *testing.T, opts ...sim.Option) int {
	t.Helper()
	cfg := marketConfig(8, 0)
	cfg.RunDays = 1
	cats := testCatalogues()
	e, err := sim.NewEngine(cfg, cats, nil, mergedPrices(cats, cfg.Seed), sim.NewCollectorPolicy(), newRNG(cfg.Seed), opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result.Days[0].NPC.CardsSold
}

func TestWithTradingCrowd_theCrowdKeepsItsSurplusWhateverThePlayerDoes(t *testing.T) {
	t.Parallel()

	plain := firstDaySold(t)
	shelved := firstDaySold(t, sim.WithTradingCrowd())

	if plain == 0 {
		t.Fatal("a crowd with no shelf sold nothing to the Counter on day 1, want it to sell at once")
	}
	if shelved != 0 {
		t.Errorf("a crowd with a shelf sold %d cards to the Counter on day 1, want none until the hold is over", shelved)
	}
}

func TestRun_aRunWithoutAMarketReportsNoMarketMetrics(t *testing.T) {
	t.Parallel()

	result := runResult(t, marketConfig(8, 0), sim.NewCollectorPolicy())

	for _, key := range []string{"market_asks", "market_listings", "market_deals"} {
		if _, ok := result.Summary[key]; ok {
			t.Errorf("summary has %s, want market metrics only in a run with a market", key)
		}
	}
}

// openAll opens boosters while the energy allows it.
func openAll(s sim.Session) {
	openOnlyPolicy{}.RunSession(s)
}
