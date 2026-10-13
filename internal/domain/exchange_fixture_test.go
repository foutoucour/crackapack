package domain_test

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

const exchangeFallbackPrice domain.USDCents = 10

// exchangeFixture is an Exchange over a real crowd, Counter and journal. NPCs open no boosters
// (their rhythm is 0), so a test puts exactly the cards it wants on their shelves.
type exchangeFixture struct {
	exchange *domain.Exchange
	crowd    *domain.Crowd
	npcs     []*domain.NPC
	player   *domain.Collection
	journal  *domain.Journal
	prices   domain.PriceSnapshot
	// keys are the non-basic non-foil printings of the first set, sorted by ID.
	keys []domain.CardKey
}

// tradingSpec is an archetype of NPCs that trade with the given margin and never open a booster.
func tradingSpec(margin domain.Percent) domain.ArchetypeSpec {
	return domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", MarginPct: margin}
}

func withExchangeDefaults(p domain.ExchangeParams) domain.ExchangeParams {
	if p.ExpiryDays == 0 {
		p.ExpiryDays = 3
	}
	if p.MaxCardsPerSide == 0 {
		p.MaxCardsPerSide = 10
	}
	if p.DuplicateKeep == 0 {
		p.DuplicateKeep = 1
	}
	if p.MaxOffersPerDay == 0 {
		p.MaxOffersPerDay = 20
	}
	if p.HistorySize == 0 {
		p.HistorySize = 50
	}
	if p.CoinRate == 0 {
		p.CoinRate = 100
	}
	return p
}

func newExchangeFixture(t *testing.T, size int, spec domain.ArchetypeSpec, params domain.ExchangeParams) *exchangeFixture {
	t.Helper()
	sets := crowdSets()
	prices := domain.PriceSnapshot{}
	for i, cat := range sets {
		for id, price := range domaintest.NewPrices(cat, uint64(i+1)) {
			prices[id] = price
		}
	}
	valuer := domain.NewValuer(prices, exchangeFallbackPrice)
	rng := rand.New(rand.NewPCG(1, 1))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: size, Archetypes: map[string]domain.ArchetypeSpec{"npc": spec}, Valuer: valuer}, sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	journal := domain.NewJournal()
	counter, err := domain.NewCounter(sets, prices, journal, domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: exchangeFallbackPrice})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	crowd, err := domain.NewCrowd(npcs, sets, opener, counter, rng, domain.WithSurplusHold(2))
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	player := domain.NewCollection()
	exchange, err := domain.NewExchange(withExchangeDefaults(params), crowd, player, journal, valuer)
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}

	fx := &exchangeFixture{exchange: exchange, crowd: crowd, npcs: npcs, player: player, journal: journal, prices: prices}
	for id, card := range sets[0].Cards {
		if !card.IsBasicLand {
			fx.keys = append(fx.keys, domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
		}
	}
	slices.SortFunc(fx.keys, func(a, b domain.CardKey) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return fx
}

// setPrice sets the reference value of key's printing, for the Counter and the Exchange alike.
func (fx *exchangeFixture) setPrice(key domain.CardKey, cents domain.USDCents) {
	fx.prices[key.ID] = cents
}

func (fx *exchangeFixture) give(t *testing.T, key domain.CardKey, n int) {
	t.Helper()
	if err := fx.player.Add(key, n); err != nil {
		t.Fatalf("give: %v", err)
	}
}

func (fx *exchangeFixture) shelve(npcID int, key domain.CardKey, n int) {
	fx.crowd.ShelveForTest(npcID, key, n)
}

func (fx *exchangeFixture) fund(t *testing.T, coins domain.Coins) {
	t.Helper()
	if err := fx.journal.Post(domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: coins}); err != nil {
		t.Fatalf("fund: %v", err)
	}
}

// fixtureState is everything a refused or failed trade must leave untouched.
type fixtureState struct {
	Player  map[domain.CardKey]int
	NPCHeld map[domain.CardKey]int
	Shelf   []domain.TradeLine
	Balance domain.Coins
	TopUp   domain.Coins
}

func (fx *exchangeFixture) state(npcID int) fixtureState {
	npc := fx.npcs[npcID]
	return fixtureState{
		Player:  fx.player.Snapshot(),
		NPCHeld: npc.Collection().Snapshot(),
		Shelf:   npc.Shelf(),
		Balance: fx.journal.Balance(domain.AccountPlayer),
		TopUp:   fx.journal.Balance(domain.AccountTradeTopUp),
	}
}

func (fx *exchangeFixture) requireUnchanged(t *testing.T, npcID int, before fixtureState) {
	t.Helper()
	if after := fx.state(npcID); !reflect.DeepEqual(before, after) {
		t.Errorf("state changed:\n before %+v\n after  %+v", before, after)
	}
}

func lineOf(key domain.CardKey, qty int) []domain.TradeLine {
	return []domain.TradeLine{{Key: key, Qty: qty}}
}
