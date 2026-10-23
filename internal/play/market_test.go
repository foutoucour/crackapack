package play_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The NPCs of marketRules, by ID: the archetype names sort in this order.
const (
	fastNPC = iota // a casual that answers at once and asks no margin
	slowNPC        // a casual that answers after slowDelay and asks 10%
	specNPC        // a speculator that opens nothing, answers after an hour and asks 20%
)

const slowDelay = 6 * time.Hour

// marketRules is counterRules with a market: three NPCs that trade, the fast and the slow one
// opening two boosters of counterSet a day, so that a new game's warm-up day leaves a copy of each
// card on their shelves.
func marketRules() play.CounterRules {
	return marketRulesWith(slowDelay)
}

func marketRulesWith(slow time.Duration, slowTraits ...domain.TraitSpec) play.CounterRules {
	r := counterRules()
	r.Crowd = domain.PopulationParams{Size: 3, Valuer: domain.NewValuer(r.Prices, 1), MaxMarginPct: 1000, Archetypes: map[string]domain.ArchetypeSpec{
		"a_fast": {Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 2, KeepCopies: 1, Behaviour: "casual"},
		"b_slow": {Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 2, KeepCopies: 1, Behaviour: "casual", MarginPct: 10, ResponseDelay: slow, Traits: slowTraits},
		"c_spec": {Share: 1, KeepCopies: 1, Behaviour: "speculator", MarginPct: 20, ResponseDelay: time.Hour},
	}}
	r.Market = &play.MarketRules{SurplusHoldDays: 7, Params: domain.MarketParams{Expiry: 3 * day, MaxCardsPerSide: 10, CoinRate: 100}}
	return r
}

func mustMarket(t *testing.T, g *play.Game) play.MarketView {
	t.Helper()
	v, err := g.Market()
	if err != nil {
		t.Fatalf("Market: %v", err)
	}
	return v
}

func mustAsk(t *testing.T, g *play.Game, key domain.CardKey) domain.MarketID {
	t.Helper()
	id, err := g.AskAround(key.ID, key.Finish)
	if err != nil {
		t.Fatalf("AskAround(%v): %v", key, err)
	}
	return id
}

// answerOf returns the answer of npc in the market, and fails the test without one.
func answerOf(t *testing.T, v play.MarketView, npc int) play.Answer {
	t.Helper()
	for _, r := range v.Requests {
		for _, a := range r.Answers {
			if a.NPC == npc {
				return a
			}
		}
	}
	for _, l := range v.Listings {
		for _, a := range l.Answers {
			if a.NPC == npc {
				return a
			}
		}
	}
	t.Fatalf("no answer of NPC %d in %+v", npc, v)
	return play.Answer{}
}

// Asking around: the NPCs that hold the card answer, each after its delay, and accepting one
// buys the card with coins at once, ends the request and saves the game.
func TestAskAround_theHoldersAnswerAndTheCardIsBought(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, marketRules(), counterSet())

	id := mustAsk(t, g, cheap())
	v := mustMarket(t, g)
	if len(v.Requests) != 1 || v.Requests[0].ID != id || v.Requests[0].Asked != 2 || v.Requests[0].Pending != 1 {
		t.Fatalf("requests = %+v, want one that asked 2 NPCs, 1 still to answer", v.Requests)
	}
	fast := answerOf(t, v, fastNPC)
	if fast.Coins != 100 || fast.Archetype != "a_fast" || v.Waiting != 1 {
		t.Errorf("fast answer = %+v, waiting %d, want 100 coins from a_fast, 1 waiting", fast, v.Waiting)
	}
	if v.Requests[0].Card.ID != cheap().ID || v.Requests[0].Finish != cheap().Finish {
		t.Errorf("request card = %+v %s, want %v", v.Requests[0].Card, v.Requests[0].Finish, cheap())
	}

	clock.Advance(slowDelay)
	v = mustMarket(t, g)
	if slow := answerOf(t, v, slowNPC); slow.Coins != 110 || v.Requests[0].Pending != 0 || v.Waiting != 2 {
		t.Errorf("slow answer = %+v, pending %d, waiting %d, want 110 coins, 0 and 2", slow, v.Requests[0].Pending, v.Waiting)
	}

	if _, err := g.AcceptAnswer(fast.ID); err != nil {
		t.Fatalf("AcceptAnswer: %v", err)
	}
	if g.Balance() != 100 || held(g, "S", cheap()) != 1 {
		t.Errorf("balance %d, copies %d, want 100 and 1", g.Balance(), held(g, "S", cheap()))
	}
	v = mustMarket(t, g)
	if len(v.Requests) != 0 || len(v.Results) != 1 || v.Results[0].Outcome != domain.MarketAccepted || v.Results[0].Card.ID != cheap().ID {
		t.Errorf("market = %+v, want no request and the accepted answer as the result", v)
	}
	if got := storedCopies(store.last(t), cheap()); got != 1 {
		t.Errorf("saved copies = %d, want 1", got)
	}
}

func storedCopies(snap play.Snapshot, key domain.CardKey) int {
	for _, c := range snap.Collection {
		if c.ID == key.ID && c.Finish == key.Finish {
			return c.Quantity
		}
	}
	return 0
}

// The slow NPC replies to a counter after its delay, by the acceptance rule.
func TestCounterAnswer_theNPCRepliesAfterItsDelay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		coins     domain.Coins
		want      domain.MarketOutcome
		wantCards int
		wantCoins domain.Coins
	}{
		{name: "accepted", coins: 110, want: domain.MarketAccepted, wantCards: 1, wantCoins: 90},
		{name: "refused", coins: 50, want: domain.MarketRefused, wantCoins: 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			clock := playtest.NewClock()
			g := counterGame(t, &memStore{}, clock, marketRules(), counterSet())
			mustAsk(t, g, cheap())
			clock.Advance(slowDelay)
			slow := answerOf(t, mustMarket(t, g), slowNPC)
			if err := g.CounterAnswer(slow.ID, tt.coins, nil); err != nil {
				t.Fatalf("CounterAnswer: %v", err)
			}
			clock.Advance(slowDelay - time.Minute)
			v := mustMarket(t, g)
			if got := answerOf(t, v, slowNPC); got.Counter == nil || got.Counter.Coins != tt.coins || v.Waiting != 1 {
				t.Fatalf("answer before the reply = %+v, waiting %d, want the counter waiting and only the fast answer waiting", got, v.Waiting)
			}
			clock.Advance(time.Minute)
			v = mustMarket(t, g)
			if len(v.Results) != 1 || v.Results[0].Outcome != tt.want {
				t.Errorf("results = %+v, want one %v", v.Results, tt.want)
			}
			if held(g, "S", cheap()) != tt.wantCards || g.Balance() != tt.wantCoins {
				t.Errorf("copies %d, balance %d, want %d and %d", held(g, "S", cheap()), g.Balance(), tt.wantCards, tt.wantCoins)
			}
		})
	}
}

// A listed card is reserved: the Counter does not buy it, and the speculator's offer sells it.
func TestList_reservesTheCardUntilAnOfferSellsIt(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, marketRules(), counterSet())
	mustOpen(t, g, "S")
	mustOpen(t, g, "S")
	if _, err := g.List(cheap().ID, cheap().Finish, 100); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, err := g.Sell(cheap().ID, cheap().Finish, 2); !errors.Is(err, play.ErrReserved) {
		t.Errorf("Sell of the listed copy: error = %v, want ErrReserved", err)
	}
	if got := g.Reserved(cheap().ID, cheap().Finish); got != 1 {
		t.Errorf("Reserved = %d, want 1", got)
	}
	if _, err := g.List(cheap().ID, cheap().Finish, 100); !errors.Is(err, play.ErrAlreadyListed) {
		t.Errorf("List twice: error = %v, want ErrAlreadyListed", err)
	}

	clock.Advance(time.Hour)
	offer := answerOf(t, mustMarket(t, g), specNPC)
	if offer.Coins != 83 || len(offer.Cards) != 0 {
		t.Fatalf("offer = %+v, want 83 coins: $1 less the speculator's 20%%", offer)
	}
	if _, err := g.AcceptAnswer(offer.ID); err != nil {
		t.Fatalf("AcceptAnswer: %v", err)
	}
	if g.Balance() != 283 || held(g, "S", cheap()) != 1 || g.Reserved(cheap().ID, cheap().Finish) != 0 {
		t.Errorf("balance %d, copies %d, reserved %d, want 283, 1 and 0", g.Balance(), held(g, "S", cheap()), g.Reserved(cheap().ID, cheap().Finish))
	}
	if _, err := g.Sell(cheap().ID, cheap().Finish, 1); err != nil {
		t.Errorf("Sell of the copy left: %v", err)
	}
	if snap := store.last(t); snap.Counter.Balances[domain.AccountMarketSale] != -83 {
		t.Errorf("saved market_sale = %d, want -83", snap.Counter.Balances[domain.AccountMarketSale])
	}
}

// The duplicates sold to the Counter leave the copies a counter reserves.
func TestSellDuplicates_keepsTheReservedCopies(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, marketRules(), counterSet())
	for range 6 {
		mustOpen(t, g, "S")
	}
	mustAsk(t, g, dear())
	clock.Advance(slowDelay)
	slow := answerOf(t, mustMarket(t, g), slowNPC)
	if err := g.CounterAnswer(slow.ID, 0, []domain.TradeLine{{Key: cheap(), Qty: 5}}); err != nil {
		t.Fatalf("CounterAnswer: %v", err)
	}
	d, err := g.Duplicates("S")
	if err != nil {
		t.Fatalf("Duplicates: %v", err)
	}
	// One cheap copy is free above the five reserved, and two dear copies are above a playset.
	if d.Copies != 3 {
		t.Errorf("Duplicates = %+v, want 3 copies", d)
	}
	if _, err := g.SellDuplicates("S"); err != nil {
		t.Fatalf("SellDuplicates: %v", err)
	}
	if held(g, "S", cheap()) != 5 || held(g, "S", dear()) != 4 {
		t.Errorf("copies %d and %d, want 5 and 4", held(g, "S", cheap()), held(g, "S", dear()))
	}
}

// A game saved in the middle of a negotiation loads with it, and the loaded game gets the same
// replies at the same instants, however often the market is looked at meanwhile.
func TestPersist_aNegotiationGoesOnAfterAReload(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	storeA := &memStore{}
	a := counterGame(t, storeA, clock, marketRules(), counterSet())
	mustOpen(t, a, "S")
	mustAsk(t, a, dear())
	clock.Advance(slowDelay)
	if err := a.CounterAnswer(answerOf(t, mustMarket(t, a), slowNPC).ID, 100, []domain.TradeLine{{Key: cheap(), Qty: 1}}); err != nil {
		t.Fatalf("CounterAnswer: %v", err)
	}
	if _, err := a.List(dear().ID, dear().Finish, 300); err != nil {
		t.Fatalf("List: %v", err)
	}

	b := counterGame(t, storeA.copied(), clock, marketRules(), counterSet())
	if va, vb := mustMarket(t, a), mustMarket(t, b); !reflect.DeepEqual(va, vb) {
		t.Fatalf("market loaded = %+v, want %+v", vb, va)
	}
	for range 4 * 24 {
		clock.Advance(time.Hour)
		mustMarket(t, a)
	}
	va, vb := mustMarket(t, a), mustMarket(t, b)
	if !reflect.DeepEqual(va, vb) {
		t.Fatalf("four days on, market loaded = %+v, want %+v", vb, va)
	}
	if a.Balance() != b.Balance() || len(va.Results) < 2 {
		t.Errorf("balance %d loaded, %d saved; results %+v, want the reply and the lapses", b.Balance(), a.Balance(), va.Results)
	}
}

// An answer due while the game is closed sees the NPCs as they stood on the day it fell due, so
// that coming back later changes no answer: the proud NPC asks more for a card the longer it has
// held it, and its price is the same whether the player looked the day it came or days after.
func TestMarket_anAnswerSeesTheNPCsOfTheDayItFellDue(t *testing.T) {
	t.Parallel()
	ego := domain.TraitSpec{Name: "ego", Params: map[string]int{"max_retries": 0, "endowment_pct_per_day": 5}}
	rules := marketRulesWith(30*time.Hour, ego)
	clock := playtest.NewClock()
	store := &memStore{}
	a := counterGame(t, store, clock, rules, counterSet())
	mustAsk(t, a, cheap())
	b := counterGame(t, store.copied(), clock, rules, counterSet())

	clock.Advance(30 * time.Hour)
	watched := answerOf(t, mustMarket(t, a), slowNPC)
	clock.Advance(2 * day)
	late := answerOf(t, mustMarket(t, b), slowNPC)
	if watched.Coins != late.Coins || !watched.At.Equal(late.At) {
		t.Errorf("answer seen days later = %+v, want %+v, as on the day it came", late, watched)
	}
	if watched.Coins <= 110 {
		t.Errorf("the proud NPC asks %d, want more than its 110 for a card held a day", watched.Coins)
	}
}

func TestMarket_refusalsChangeNothingAndAreNotSaved(t *testing.T) {
	t.Parallel()
	alone := marketRules()
	alone.Crowd.Archetypes = map[string]domain.ArchetypeSpec{"c_spec": alone.Crowd.Archetypes["c_spec"]}
	tests := []struct {
		name  string
		rules play.CounterRules
		act   func(g *play.Game) error
		want  error
	}{
		{name: "asking for an unknown card", rules: marketRules(), act: func(g *play.Game) error {
			_, err := g.AskAround("nope", domain.FinishNonFoil)
			return err
		}, want: play.ErrUnknownCard},
		{name: "asking for a card nobody has", rules: alone, act: func(g *play.Game) error {
			_, err := g.AskAround(cheap().ID, cheap().Finish)
			return err
		}, want: play.ErrNobodyHasIt},
		{name: "listing a card the player lacks", rules: marketRules(), act: func(g *play.Game) error {
			_, err := g.List(cheap().ID, cheap().Finish, 10)
			return err
		}, want: play.ErrNotHeld},
		{name: "accepting an unknown answer", rules: marketRules(), act: func(g *play.Game) error {
			_, err := g.AcceptAnswer(99)
			return err
		}, want: play.ErrNotOnMarket},
		{name: "unlisting an unknown listing", rules: marketRules(), act: func(g *play.Game) error { return g.Unlist(99) }, want: play.ErrNotOnMarket},
		{name: "a game without a market", rules: counterRules(), act: func(g *play.Game) error {
			_, err := g.Market()
			return err
		}, want: play.ErrNoMarket},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := &playtest.FlakyStore{}
			g := counterGame(t, store, playtest.NewClock(), tt.rules, counterSet())
			saved := store.Saved()
			if err := tt.act(g); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
			if store.Saved() != saved {
				t.Error("a refusal was saved")
			}
		})
	}
}

// A save whose crowd is not the game's, after the population changed, loads with the game's new
// crowd: what the old NPCs said is dropped, and what the player asked for stays.
func TestPersist_aSaveOfAnotherCrowdGetsTheNewOne(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, marketRules(), counterSet())
	mustAsk(t, g, cheap())
	bigger := marketRules()
	bigger.Crowd.Size = 4

	loaded := counterGame(t, store.copied(), clock, bigger, counterSet())
	v := mustMarket(t, loaded)
	if len(v.Requests) != 1 || v.Requests[0].Pending != 0 || len(v.Requests[0].Answers) != 0 {
		t.Errorf("requests = %+v, want the request with no answer", v.Requests)
	}
	if got := len(store.copied().last(t).Counter.Crowd.NPCs); got != 3 {
		t.Fatalf("the test saved %d NPCs, want 3", got)
	}
}

// A save from before the market, whose NPCs list the cards they keep one by one, loads.
func TestPersist_theCardsOfTheNPCsOfAnOlderSaveLoad(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	counterGame(t, store, playtest.NewClock(), marketRules(), counterSet())
	snap := store.last(t)
	npc := &snap.Counter.Crowd.NPCs[fastNPC]
	npc.Held = ""
	npc.Cards = []play.CardCount{{ID: "S-00", Finish: domain.FinishNonFoil, Quantity: 1}, {ID: "S-01", Finish: domain.FinishNonFoil, Quantity: 1}}
	loadedStore := &memStore{snap: snap, found: true}
	g := counterGame(t, loadedStore, playtest.NewClock(), marketRules(), counterSet())
	mustOpen(t, g, "S")
	if got := loadedStore.last(t).Counter.Crowd.NPCs[fastNPC]; got.Held == "" || got.Cards != nil {
		t.Errorf("NPC saved again = %+v, want its cards in the compact form", got)
	}
}

// The NPCs of a game in stages open the sets the player can open, so that the market holds the
// cards the player looks for: a locked set's cards come to them once it unlocks.
func TestMarket_theNPCsOpenTheSetsThePlayerCanOpen(t *testing.T) {
	t.Parallel()
	// A gives its whole binder in one booster and ranks first; B is locked until A is complete.
	a, b := playtest.TinySet("A", 2, 2), playtest.TinySet("B", 3, 1)
	clock := playtest.NewClock()
	store := &memStore{}
	g, stream, err := newSeededGame(clock, b, a)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := g.Stage(play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, play.NewStream(3)); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	rules := marketRules()
	rules.Prices = domain.PriceSnapshot{}
	if err := g.OpenCounter(rules, play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	if err := g.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	npcHolds := func(prefix string) bool {
		for _, n := range store.last(t).Counter.Crowd.NPCs {
			if strings.Contains(" "+n.Held+" "+n.Shelf, " "+prefix) {
				return true
			}
		}
		return false
	}

	// Opening a booster saves the game; the market caught the NPCs up first.
	clock.Advance(3 * day)
	mustMarket(t, g)
	mustOpen(t, g, "A")
	if !npcHolds("A-") || npcHolds("B-") {
		t.Fatalf("NPCs hold A %v and B %v, want A only while B is locked", npcHolds("A-"), npcHolds("B-"))
	}
	clock.Advance(3 * day)
	mustMarket(t, g)
	mustOpen(t, g, "B")
	if !npcHolds("B-") {
		t.Error("three days after B unlocked, no NPC holds a card of B")
	}
}

// A save from before the game had NPCs loads with a crowd as a new game has it, whose shelves hold
// cards to trade at once.
func TestPersist_aSaveWithoutNPCsGetsACrowdReadyToTrade(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	counterGame(t, store, playtest.NewClock(), counterRules(), counterSet())
	g := counterGame(t, store.copied(), playtest.NewClock(), marketRules(), counterSet())
	if _, err := g.AskAround(cheap().ID, cheap().Finish); err != nil {
		t.Errorf("AskAround after loading a save without NPCs: %v", err)
	}
}

// A save from before the game had coins, with cards in it, loads into a game with a market that
// trades those cards, and what the market moves is saved.
func TestPersist_aSaveFromBeforeCoinsTradesItsCardsOnTheMarket(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	plain, stream, err := newSeededGame(clock, counterSet())
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := plain.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	mustOpen(t, plain, "S")

	g := counterGame(t, store, clock, marketRules(), counterSet())
	if _, err := g.List(dear().ID, dear().Finish, 300); err != nil {
		t.Fatalf("List of a card of the save: %v", err)
	}
	mustAsk(t, g, cheap())
	if _, err := g.AcceptAnswer(answerOf(t, mustMarket(t, g), fastNPC).ID); err != nil {
		t.Fatalf("AcceptAnswer: %v", err)
	}
	if held(g, "S", cheap()) != 2 || g.Balance() != 100 {
		t.Fatalf("copies %d, balance %d, want 2 and 100", held(g, "S", cheap()), g.Balance())
	}
	loaded := counterGame(t, store.copied(), clock, marketRules(), counterSet())
	if held(loaded, "S", cheap()) != 2 || loaded.Balance() != 100 || loaded.Reserved(dear().ID, dear().Finish) != 1 {
		t.Errorf("after a reload: copies %d, balance %d, reserved %d, want 2, 100 and 1",
			held(loaded, "S", cheap()), loaded.Balance(), loaded.Reserved(dear().ID, dear().Finish))
	}
}
