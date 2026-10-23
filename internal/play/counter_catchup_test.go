package play_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// emptyCounter is counterRules with nothing in stock at the start.
func emptyCounter() play.CounterRules {
	r := counterRules()
	r.Params.InitialStock = map[domain.Rarity]int{}
	return r
}

// cardsInStock counts the cards of cat that the Counter has a copy of, as a player can tell from
// the quotes.
func cardsInStock(t *testing.T, g *play.Game, cat domain.SetCatalogue) int {
	t.Helper()
	n := 0
	for id := range cat.Cards {
		if mustQuote(t, g, domain.CardKey{ID: id, Finish: domain.FinishNonFoil}).InStock {
			n++
		}
	}
	return n
}

func TestCatchUp_theSupplyStocksTheCounterOnceAWholeDayHasGone(t *testing.T) {
	t.Parallel()
	set := playtest.TinySet("S", 10, 1)
	rules := emptyCounter()
	rules.Supply = play.SupplyRules{BoostersPerDay: 3}
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, rules, set)

	clock.Advance(day - time.Minute)
	if got := cardsInStock(t, g, set); got != 0 {
		t.Errorf("cards in stock before a whole day = %d, want 0", got)
	}
	clock.Advance(time.Minute)
	if got := cardsInStock(t, g, set); got < 1 || got > 3 {
		t.Errorf("cards in stock after a day of 3 boosters of one card = %d, want 1 to 3", got)
	}
}

// A long time away catches up on CatchUpDays days only, so that it cannot hold up the start, and
// the days beyond are skipped, not kept for later.
func TestCatchUp_isCappedAndTheDaysBeyondAreSkipped(t *testing.T) {
	t.Parallel()
	set := playtest.TinySet("S", 40, 1)
	rules := emptyCounter()
	rules.Supply = play.SupplyRules{BoostersPerDay: 1}

	uncappedClock := playtest.NewClock()
	uncapped := counterGame(t, &memStore{}, uncappedClock, rules, set)
	uncappedClock.Advance(10 * day)
	if got := cardsInStock(t, uncapped, set); got <= 2 {
		t.Fatalf("ten days of one booster gave %d cards in stock, too few for the test to tell a cap", got)
	}

	rules.CatchUpDays = 2
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, rules, set)
	clock.Advance(10 * day)
	if got := cardsInStock(t, g, set); got < 1 || got > 2 {
		t.Errorf("cards in stock after ten days capped at two = %d, want 1 or 2", got)
	}
	clock.Advance(day)
	if got := cardsInStock(t, g, set); got > 3 {
		t.Errorf("cards in stock a day later = %d, want at most 3: the skipped days are gone", got)
	}
}

// The NPCs sell the copies above what they keep to the Counter, which then has them in stock.
func TestCatchUp_theCrowdSellsItsSurplusToTheCounter(t *testing.T) {
	t.Parallel()
	set := playtest.TinySet("S", 3, 3)
	rules := emptyCounter()
	rules.Crowd = domain.PopulationParams{Size: 2, Archetypes: map[string]domain.ArchetypeSpec{
		"npc": {Share: 1, MinBoostersPerDay: 5, MaxBoostersPerDay: 5, KeepCopies: 1},
	}}
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, rules, set)

	clock.Advance(day)
	if got := cardsInStock(t, g, set); got != 3 {
		t.Errorf("cards in stock after a day of NPC surplus = %d, want 3", got)
	}
}

// A card that vanished from a limited shelf while the game was closed was last in stock at the
// instant of the day it vanished, not when the game was played again.
func TestCatchUp_aCardThatVanishedWasLastInStockWhenItsLifeEnded(t *testing.T) {
	t.Parallel()
	rules := emptyCounter()
	rules.Params.ShelfLife = domain.VanishAfterDays(1)
	clock := playtest.NewClock()
	start := clock.Now()
	g := counterGame(t, &memStore{}, clock, rules, counterSet())
	mustOpen(t, g, "S")
	clock.Advance(time.Hour)
	if _, err := g.Sell(cheap().ID, cheap().Finish, 1); err != nil {
		t.Fatalf("Sell: %v", err)
	}

	clock.Advance(3 * day)
	q := mustQuote(t, g, cheap())
	if want := start.Add(day); q.InStock || !q.LastInStock.Equal(want) {
		t.Errorf("quote = %+v, want out of stock since %v", q, want)
	}
}

// A game saved with its Counter, its supply and its crowd loads back as it was, and goes on as the
// game that was not closed does.
func TestPersist_theCounterIsSavedAndLoadsBackAsItWas(t *testing.T) {
	t.Parallel()
	set := playtest.TinySet("S", 3, 3)
	rules := counterRules()
	rules.Params.ShelfLife = domain.VanishAfterDays(2)
	rules.Supply = play.SupplyRules{BoostersPerDay: 2}
	rules.Crowd = domain.PopulationParams{Size: 2, Archetypes: map[string]domain.ArchetypeSpec{
		"npc": {Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 4, KeepCopies: 1},
	}}
	clock := playtest.NewClock()
	storeA := &memStore{}
	a := counterGame(t, storeA, clock, rules, set)
	for range 3 {
		mustOpen(t, a, "S")
	}
	mustSell(t, a, "S-00", 2)
	clock.Advance(2*day + 3*time.Hour)
	if _, err := a.Buy("S-00", domain.FinishNonFoil); err != nil {
		t.Fatalf("Buy: %v", err)
	}

	storeB := storeA.copied()
	b := counterGame(t, storeB, clock, rules, set)
	if a.Balance() != b.Balance() {
		t.Errorf("Balance = %d loaded, %d saved", b.Balance(), a.Balance())
	}
	clock.Advance(2 * day)
	for id := range set.Cards {
		key := domain.CardKey{ID: id, Finish: domain.FinishNonFoil}
		if qa, qb := mustQuote(t, a, key), mustQuote(t, b, key); qa != qb {
			t.Errorf("quote of %s = %+v loaded, %+v saved", id, qb, qa)
		}
	}
	mustSell(t, a, "S-01", 1)
	mustSell(t, b, "S-01", 1)
	if sa, sb := storeA.last(t).Counter, storeB.last(t).Counter; !reflect.DeepEqual(sa, sb) {
		t.Errorf("the loaded game went on otherwise:\nloaded %+v\nsaved  %+v", sb, sa)
	}

	var sum domain.Coins
	for account, balance := range storeA.last(t).Counter.Balances {
		sum += balance
		if account == domain.AccountPlayer && balance < 0 {
			t.Errorf("the player's balance is %d", balance)
		}
	}
	if sum != 0 {
		t.Errorf("the saved balances sum to %d, want 0", sum)
	}
}

func mustSell(t *testing.T, g *play.Game, id domain.CardID, qty int) {
	t.Helper()
	if _, err := g.Sell(id, domain.FinishNonFoil, qty); err != nil {
		t.Fatalf("Sell(%s, %d): %v", id, qty, err)
	}
}

// A save written before the game had coins loads with the welcome bonus and a Counter as a new
// game has them.
func TestPersist_aVersion1SaveGetsTheWelcomeBonusAndAFreshCounter(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	data, err := os.ReadFile("testdata/save-v1.json")
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "save.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("copy the fixture: %v", err)
	}

	g := counterGame(t, play.NewFileStore(path), playtest.NewClock(), counterRules(), rav)

	if got := g.Balance(); got != 200 {
		t.Errorf("Balance = %d, want 200", got)
	}
	if q := mustQuote(t, g, domain.CardKey{ID: "rav-c-001", Finish: domain.FinishNonFoil}); !q.InStock {
		t.Errorf("quote of a common = %+v, want it in the initial stock", q)
	}
}

func TestPersist_aCounterThatDoesNotFitIsRefused(t *testing.T) {
	t.Parallel()
	market := marketRules()
	tests := []struct {
		name  string
		rules play.CounterRules
		edit  func(*play.CounterSnapshot)
	}{
		{name: "balances that do not sum to zero", rules: counterRules(), edit: func(s *play.CounterSnapshot) { s.Balances[domain.AccountPlayer]++ }},
		{name: "a negative stock", rules: counterRules(), edit: func(s *play.CounterSnapshot) {
			s.Stock = append(s.Stock, play.CardCount{ID: "S-00", Finish: domain.FinishNonFoil, Quantity: -1})
		}},
		{name: "a crowd the game does not have", rules: counterRules(), edit: func(s *play.CounterSnapshot) {
			s.Crowd = &play.CrowdSnapshot{Rand: play.OpenerSnapshot{Seed: 8, State: []byte("x")}}
		}},
		{name: "a market the game does not have", rules: counterRules(), edit: func(s *play.CounterSnapshot) {
			s.Market = &play.MarketSnapshot{NextID: 1}
		}},
		{name: "cards of an NPC that do not read", rules: market, edit: func(s *play.CounterSnapshot) { s.Crowd.NPCs[0].Held = "S-00=0" }},
		{name: "a shelf of an NPC that does not read", rules: market, edit: func(s *play.CounterSnapshot) { s.Crowd.NPCs[0].Shelf = "S-00" }},
		{name: "an answer of an unknown kind", rules: market, edit: func(s *play.CounterSnapshot) {
			s.Market.Answers = append(s.Market.Answers, play.AnswerSnapshot{ID: 1, Kind: "bid"})
		}},
		{name: "a listing of a card the player lacks", rules: market, edit: func(s *play.CounterSnapshot) {
			s.Market.NextID = 2
			s.Market.Listings = []play.ListingSnapshot{{ID: 1, Card: "S-00", Finish: domain.FinishNonFoil, Price: 5}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			saved := &memStore{}
			counterGame(t, saved, playtest.NewClock(), tt.rules, counterSet())
			snap := saved.last(t)
			tt.edit(snap.Counter)
			if _, err := tryCounterGame(&memStore{snap: snap, found: true}, playtest.NewClock(), tt.rules, counterSet()); err == nil {
				t.Error("the save loaded")
			}
		})
	}
}

// The Counter's memory of when it last had a card survives a reload.
func TestPersist_aReloadRemembersWhenTheCounterLastHadACard(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, counterRules(), counterSet())
	clock.Advance(time.Hour)
	boughtAt := clock.Now()
	if _, err := g.Buy(cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("Buy: %v", err)
	}

	clock.Advance(time.Hour)
	loaded := counterGame(t, store.copied(), clock, counterRules(), counterSet())
	if q := mustQuote(t, loaded, cheap()); q.InStock || !q.LastInStock.Equal(boughtAt) {
		t.Errorf("quote after a reload = %+v, want out of stock since %v", q, boughtAt)
	}
}

// A game without a Counter writes back the Counter of its save as it was, so that trying such a
// game on a save loses no coin.
func TestPersist_aGameWithoutACounterKeepsTheCounterOfTheSave(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	g := counterGame(t, store, playtest.NewClock(), counterRules(), counterSet())
	if _, err := g.Buy(cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("Buy: %v", err)
	}
	saved := store.last(t).Counter

	plain, stream, err := newSeededGame(playtest.NewClock(), counterSet())
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := plain.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	mustOpen(t, plain, "S")
	if got := store.last(t).Counter; !reflect.DeepEqual(got, saved) {
		t.Errorf("Counter after a game without one = %+v, want %+v", got, saved)
	}
}

// The Counter's limited shelf is written compactly, and a save from before, which lists its copies
// one batch at a time, still loads.
func TestPersist_theCountersShelfIsCompactAndAnOlderOneLoads(t *testing.T) {
	t.Parallel()
	rules := emptyCounter()
	rules.Params.ShelfLife = domain.VanishAfterDays(2)
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, rules, counterSet())
	mustOpen(t, g, "S")
	mustSell(t, g, "S-00", 1)
	snap := store.last(t)
	if snap.Counter.Shelved == "" || snap.Counter.Shelf != nil {
		t.Fatalf("saved shelf = %q and %+v, want it compact only", snap.Counter.Shelved, snap.Counter.Shelf)
	}

	snap.Counter.Shelved = ""
	snap.Counter.Shelf = []play.ShelfCount{{ID: "S-00", Finish: domain.FinishNonFoil, Tick: 0, Quantity: 1}}
	loaded := counterGame(t, &memStore{snap: snap, found: true}, clock, rules, counterSet())
	if q := mustQuote(t, loaded, cheap()); !q.InStock {
		t.Errorf("quote after loading the older shelf = %+v, want the copy in stock", q)
	}
}
