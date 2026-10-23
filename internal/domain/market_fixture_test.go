package domain_test

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// marketStart is the instant a market test starts at.
func marketStart() time.Time { return time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC) }

// The NPCs of a market fixture, by ID: the archetype names sort in this order.
const (
	fastNPC = iota // a casual that answers at once and asks no margin
	slowNPC        // a casual that answers after six hours and asks 10%
	specNPC        // a speculator that answers after an hour and asks 20%
)

const slowDelay = 6 * time.Hour

// marketExpiry is how long an answer of a market fixture stays open.
const marketExpiry = 72 * time.Hour

// marketFixture is a Market over a real crowd, Counter and journal. NPCs open no boosters and wish
// for nothing, so a test puts exactly the cards and wishes it wants on them.
type marketFixture struct {
	market  *domain.Market
	crowd   *domain.Crowd
	npcs    []*domain.NPC
	player  *domain.Collection
	journal *domain.Journal
	prices  domain.PriceSnapshot
	// keys are the non-basic non-foil printings of the first set, sorted by ID, each at $1.
	keys []domain.CardKey
}

func marketArchetypes(traits ...domain.TraitSpec) map[string]domain.ArchetypeSpec {
	return map[string]domain.ArchetypeSpec{
		"a_fast": {Share: 1, KeepCopies: 1, Behaviour: "casual"},
		"b_slow": {Share: 1, KeepCopies: 1, Behaviour: "casual", MarginPct: 10, ResponseDelay: slowDelay, Traits: traits},
		"c_spec": {Share: 1, KeepCopies: 1, Behaviour: "speculator", MarginPct: 20, ResponseDelay: time.Hour},
	}
}

func newMarketFixture(t *testing.T, traits ...domain.TraitSpec) *marketFixture {
	t.Helper()
	sets := crowdSets()
	fx := &marketFixture{prices: domain.PriceSnapshot{}, player: domain.NewCollection(), journal: domain.NewJournal()}
	for id, card := range sets[0].Cards {
		if !card.IsBasicLand {
			key := domain.CardKey{ID: id, Finish: domain.FinishNonFoil}
			fx.keys = append(fx.keys, key)
			fx.prices[id] = 100
		}
	}
	slices.SortFunc(fx.keys, func(a, b domain.CardKey) int { return cmpCardID(a.ID, b.ID) })
	valuer := domain.NewValuer(fx.prices, exchangeFallbackPrice)
	rng := rand.New(rand.NewPCG(1, 1))
	var err error
	fx.npcs, err = domain.GeneratePopulation(domain.PopulationParams{Size: 3, Archetypes: marketArchetypes(traits...), Valuer: valuer, MaxMarginPct: 1000}, sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	counter, err := domain.NewCounter(sets, fx.prices, fx.journal, domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: exchangeFallbackPrice})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	if fx.crowd, err = domain.NewCrowd(fx.npcs, sets, newOpenerOn(rng), counter, rng, domain.WithSurplusHold(7)); err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	for _, npc := range fx.npcs {
		fx.crowd.SetWishesForTest(npc.ID, nil)
	}
	fx.market, err = domain.NewMarket(domain.MarketParams{Expiry: marketExpiry, MaxCardsPerSide: 10, CoinRate: 100}, fx.crowd, fx.player, fx.journal, valuer)
	if err != nil {
		t.Fatalf("NewMarket: %v", err)
	}
	fx.fund(t, 1000)
	return fx
}

func cmpCardID(a, b domain.CardID) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func (fx *marketFixture) fund(t *testing.T, coins domain.Coins) {
	t.Helper()
	if err := fx.journal.Post(domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: coins}); err != nil {
		t.Fatalf("fund: %v", err)
	}
}

func (fx *marketFixture) give(t *testing.T, key domain.CardKey, n int) {
	t.Helper()
	if err := fx.player.Add(key, n); err != nil {
		t.Fatalf("give: %v", err)
	}
}

func (fx *marketFixture) balance() domain.Coins { return fx.journal.Balance(domain.AccountPlayer) }

func (fx *marketFixture) advance(t *testing.T, at time.Time) {
	t.Helper()
	if err := fx.market.Advance(at); err != nil {
		t.Fatalf("Advance(%v): %v", at, err)
	}
}

func (fx *marketFixture) ask(t *testing.T, key domain.CardKey, at time.Time) domain.MarketID {
	t.Helper()
	id, err := fx.market.AskAround(key, at)
	if err != nil {
		t.Fatalf("AskAround(%v): %v", key, err)
	}
	return id
}

func (fx *marketFixture) list(t *testing.T, key domain.CardKey, price domain.Coins, at time.Time) domain.MarketID {
	t.Helper()
	id, err := fx.market.List(key, price, at)
	if err != nil {
		t.Fatalf("List(%v): %v", key, err)
	}
	return id
}

// answerFrom returns the open answer of npc, and fails the test without one.
func (fx *marketFixture) answerFrom(t *testing.T, npc int) domain.MarketAnswer {
	t.Helper()
	for _, a := range fx.market.Answers() {
		if a.NPC == npc {
			return a
		}
	}
	t.Fatalf("no answer from NPC %d in %+v", npc, fx.market.Answers())
	return domain.MarketAnswer{}
}

// lastResult is the latest result, which the test fails without.
func (fx *marketFixture) lastResult(t *testing.T) domain.MarketResult {
	t.Helper()
	results := fx.market.Results()
	if len(results) == 0 {
		t.Fatal("no result")
	}
	return results[len(results)-1]
}

// cardsInPlay is every copy the player and the NPCs hold, shelves included: a trade moves cards,
// and never makes or destroys one.
func (fx *marketFixture) cardsInPlay() map[domain.CardKey]int {
	all := fx.player.Snapshot()
	for _, npc := range fx.npcs {
		for key, n := range npc.Collection().Snapshot() {
			all[key] += n
		}
		for _, l := range npc.Shelf() {
			all[l.Key] += l.Qty
		}
	}
	return all
}

// playerValue is what the player owns, in hundredths of a coin at rate 100, as the Exchange's
// invariant counts it: a trade must never raise it.
func (fx *marketFixture) playerValue() int64 {
	const rate = 100
	return int64(fx.balance())*100 + int64(fx.player.Value(fx.prices, exchangeFallbackPrice))*rate
}
