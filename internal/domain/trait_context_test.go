package domain_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// marginContextSpy is a margin adder that records every context the Exchange gives it.
type marginContextSpy struct{ seen []domain.MarginContext }

func (m *marginContextSpy) Name() string { return "margin_context_spy" }

func (m *marginContextSpy) MarginAdd(ctx domain.MarginContext) domain.Percent {
	m.seen = append(m.seen, ctx)
	return 0
}

func TestExchangeGivesMarginAddersTheNPCAndTheTick(t *testing.T) {
	t.Parallel()
	spy := &marginContextSpy{}
	reg := domain.TraitRegistry{
		"spy": func(map[string]int, domain.TraitEnv) (domain.Trait, error) { return spy, nil },
	}
	spec := domain.ArchetypeSpec{
		Share:      1,
		KeepCopies: 1,
		Behaviour:  "casual",
		Traits:     []domain.TraitSpec{{Name: "spy"}},
	}
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{}, reg, 1000)
	fx.setPrice(fx.keys[0], 500)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 1)
	fx.shelve(0, fx.keys[1], 1)
	fx.exchange.StartDay(1)

	const ticks = 3
	for range ticks {
		if _, err := fx.crowd.Tick(); err != nil {
			t.Fatalf("Tick: %v", err)
		}
	}

	_, err := fx.exchange.Propose(0,
		[]domain.TradeLine{{Key: fx.keys[0], Qty: 1}},
		[]domain.TradeLine{{Key: fx.keys[1], Qty: 1}}, 0)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if len(spy.seen) == 0 {
		t.Fatal("the margin adder was never asked")
	}
	got := spy.seen[0]
	if got.NPC == nil || got.NPC.ID != 0 {
		t.Errorf("MarginContext.NPC = %v, want the NPC 0 that gives the cards", got.NPC)
	}
	if got.Offer.NPC != 0 {
		t.Errorf("MarginContext.Offer.NPC = %d, want 0", got.Offer.NPC)
	}
	if got.Tick != ticks {
		t.Errorf("MarginContext.Tick = %d, want %d ticks run", got.Tick, ticks)
	}
}

func TestTraitConstructorsReceiveTheCrowdEnvironment(t *testing.T) {
	t.Parallel()
	const released = "2005-10-07"
	sets := crowdSets()
	sets[0].ReleaseDate = released
	sets[1].ReleaseDate = "" // clear any date so sets[1] is the undated fixture

	var env domain.TraitEnv
	reg := domain.TraitRegistry{
		"probe": func(_ map[string]int, e domain.TraitEnv) (domain.Trait, error) {
			env = e
			return &marginContextSpy{}, nil
		},
	}
	valuer := domain.NewValuer(domain.PriceSnapshot{}, exchangeFallbackPrice)
	_, err := domain.GeneratePopulation(domain.PopulationParams{
		Size: 1,
		Archetypes: map[string]domain.ArchetypeSpec{"npc": {
			Share: 1, KeepCopies: 1, Behaviour: "casual", Traits: []domain.TraitSpec{{Name: "probe"}},
		}},
		Valuer:        valuer,
		TraitRegistry: reg,
		MaxMarginPct:  100,
	}, sets, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}

	var id domain.CardID
	for cardID := range sets[0].Cards {
		id = cardID
		break
	}
	if card, ok := env.Cards(id); !ok || card.ID != id {
		t.Errorf("Cards(%q) = %v, %v, want the card", id, card, ok)
	}
	if code, ok := env.CardSet(id); !ok || code != sets[0].Code {
		t.Errorf("CardSet(%q) = %q, %v, want %q", id, code, ok, sets[0].Code)
	}
	if date, ok := env.SetReleased(sets[0].Code); !ok || date != released {
		t.Errorf("SetReleased(%q) = %q, %v, want %q", sets[0].Code, date, ok, released)
	}
	if _, ok := env.SetReleased(sets[1].Code); ok {
		t.Errorf("SetReleased(%q) found a date for a set that has none", sets[1].Code)
	}
	if got := env.Valuer.Price(id); got != valuer.Price(id) {
		t.Errorf("Valuer.Price(%q) = %d, want %d", id, got, valuer.Price(id))
	}
}
