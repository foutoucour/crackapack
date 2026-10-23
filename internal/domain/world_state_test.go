package domain_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// world is a Counter fed by a supply or a crowd, on a random stream whose state a test can copy,
// as a save does.
type world struct {
	src     *rand.PCG
	counter *domain.Counter
	supply  *domain.CounterSupply
	crowd   *domain.Crowd
}

func (w *world) tick(t *testing.T, ticks int) {
	t.Helper()
	for range ticks {
		w.counter.Tick()
		if w.supply != nil {
			if _, err := w.supply.Tick(); err != nil {
				t.Fatalf("supply Tick: %v", err)
			}
		}
		if w.crowd != nil {
			if _, err := w.crowd.Tick(); err != nil {
				t.Fatalf("crowd Tick: %v", err)
			}
		}
	}
}

// restoreFrom puts the stream and the Counter of w where those of other stand.
func (w *world) restoreFrom(t *testing.T, other *world) {
	t.Helper()
	state, err := other.src.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if err := w.src.UnmarshalBinary(state); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if err := w.counter.Restore(other.counter.State()); err != nil {
		t.Fatalf("Counter Restore: %v", err)
	}
}

// sameStock fails the test when two Counters hold different stocks of any card of the sets.
func sameStock(t *testing.T, what string, a, b *domain.Counter) {
	t.Helper()
	for _, cat := range crowdSets() {
		for id, card := range cat.Cards {
			for _, f := range card.Finishes {
				key := domain.CardKey{ID: id, Finish: f}
				if a.Stock(key) != b.Stock(key) {
					t.Fatalf("%s: stock of %v = %d restored, %d original", what, key, b.Stock(key), a.Stock(key))
				}
			}
		}
	}
}

func newWorldCounter(t *testing.T) *domain.Counter {
	t.Helper()
	counter, err := domain.NewCounter(crowdSets(), domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{
		SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 10, ShelfLife: domain.VanishAfterDays(1),
	})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	return counter
}

func newOpenerOn(rng *rand.Rand) *domain.Opener {
	return domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
}

func newSupplyWorld(t *testing.T, energyMode bool) *world {
	t.Helper()
	w := &world{src: rand.NewPCG(3, 3), counter: newWorldCounter(t)}
	rng := rand.New(w.src)
	var err error
	if energyMode {
		energy, eerr := domain.NewEnergy(domain.EnergyParams{RechargePerTick: 1, ReserveCap: 20, SavingsIntervalTicks: 1, SavingsCap: 20})
		if eerr != nil {
			t.Fatalf("NewEnergy: %v", eerr)
		}
		w.supply, err = domain.NewEnergyCounterSupply(w.counter, crowdSets(), newOpenerOn(rng), rng, energy, map[string]int{"RAV": 7, "CLS": 12})
	} else {
		w.supply, err = domain.NewFixedCounterSupply(w.counter, crowdSets(), newOpenerOn(rng), rng, 5)
	}
	if err != nil {
		t.Fatalf("supply: %v", err)
	}
	return w
}

// A supply restored from the state of another, with its stream and Counter, opens what the other
// opens from then on: the credit of the day, the energy and the set picked carry over.
func TestCounterSupplyRestore_continuesAsTheSupplyItCameFrom(t *testing.T) {
	t.Parallel()
	for _, energyMode := range []bool{false, true} {
		name := "fixed"
		if energyMode {
			name = "energy"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newSupplyWorld(t, energyMode)
			// Not a whole day, so that some credit or energy is under way.
			a.tick(t, domain.TicksPerDay+50)

			b := newSupplyWorld(t, energyMode)
			b.restoreFrom(t, a)
			if err := b.supply.Restore(a.supply.State()); err != nil {
				t.Fatalf("supply Restore: %v", err)
			}
			for step := range 4 {
				a.tick(t, 37)
				b.tick(t, 37)
				sameStock(t, fmt.Sprintf("step %d", step), a.counter, b.counter)
			}
		})
	}
}

func TestCounterSupplyRestore_refusesAStateThatCannotBe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		energyMode bool
		state      domain.SupplyState
	}{
		{name: "a negative credit", state: domain.SupplyState{Credit: -1}},
		{name: "a day of credit or more", state: domain.SupplyState{Credit: domain.TicksPerDay}},
		{name: "a set picked that is not one", energyMode: true, state: domain.SupplyState{Next: "NOPE"}},
		{name: "energy above the cap", energyMode: true, state: domain.SupplyState{Energy: domain.EnergyState{Reserve: 21}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := newSupplyWorld(t, tt.energyMode)
			if err := w.supply.Restore(tt.state); err == nil {
				t.Error("Restore went through")
			}
		})
	}
}

func newCrowdWorld(t *testing.T) *world {
	t.Helper()
	w := &world{src: rand.NewPCG(5, 5), counter: newWorldCounter(t)}
	rng := rand.New(w.src)
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: 3, Archetypes: map[string]domain.ArchetypeSpec{
		"npc": {Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 7, KeepCopies: 2},
	}}, crowdSets(), rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	if w.crowd, err = domain.NewCrowd(npcs, crowdSets(), newOpenerOn(rng), w.counter, rng); err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	return w
}

// A crowd restored from the state of another keeps what each NPC holds and where its day stands,
// so that it sells the Counter what the other sells from then on.
func TestCrowdRestore_continuesAsTheCrowdItCameFrom(t *testing.T) {
	t.Parallel()
	a := newCrowdWorld(t)
	a.tick(t, 2*domain.TicksPerDay+11)
	state, err := a.crowd.State()
	if err != nil {
		t.Fatalf("State: %v", err)
	}

	b := newCrowdWorld(t)
	b.restoreFrom(t, a)
	if err := b.crowd.Restore(state); err != nil {
		t.Fatalf("crowd Restore: %v", err)
	}
	if b.crowd.CardsHeld() != a.crowd.CardsHeld() {
		t.Errorf("CardsHeld = %d restored, %d original", b.crowd.CardsHeld(), a.crowd.CardsHeld())
	}
	for step := range 3 {
		a.tick(t, domain.TicksPerDay/2)
		b.tick(t, domain.TicksPerDay/2)
		sameStock(t, fmt.Sprintf("step %d", step), a.counter, b.counter)
	}
}

func TestCrowdRestore_refusesAStateThatCannotBe(t *testing.T) {
	t.Parallel()
	card := domain.CardKey{ID: "rav-c-000", Finish: domain.FinishNonFoil}
	tests := []struct {
		name  string
		state domain.CrowdState
	}{
		{name: "another number of NPCs", state: domain.CrowdState{NPCs: make([]domain.NPCState, 2)}},
		{name: "a negative tick", state: domain.CrowdState{Tick: -1, NPCs: make([]domain.NPCState, 3)}},
		{name: "a day of credit", state: domain.CrowdState{NPCs: []domain.NPCState{{Credit: domain.TicksPerDay}, {}, {}}}},
		{name: "a card held no times", state: domain.CrowdState{NPCs: []domain.NPCState{{Cards: map[domain.CardKey]int{card: 0}}, {}, {}}}},
		{name: "a shelf copy from after the tick", state: domain.CrowdState{Tick: 5, NPCs: []domain.NPCState{{Shelf: map[domain.CardKey][]int{card: {6}}}, {}, {}}}},
		{name: "a shelf copy from before the start", state: domain.CrowdState{Tick: 5, NPCs: []domain.NPCState{{Shelf: map[domain.CardKey][]int{card: {-1}}}, {}, {}}}},
		{name: "shelf copies newest first", state: domain.CrowdState{Tick: 5, NPCs: []domain.NPCState{{Shelf: map[domain.CardKey][]int{card: {3, 2}}}, {}, {}}}},
		{name: "a shelf with no copy", state: domain.CrowdState{Tick: 5, NPCs: []domain.NPCState{{Shelf: map[domain.CardKey][]int{card: {}}}, {}, {}}}},
		{name: "a wish for no copy", state: domain.CrowdState{NPCs: []domain.NPCState{{Wishes: []domain.Wish{{ID: card.ID, Want: 0}}}, {}, {}}}},
		{name: "a wish twice", state: domain.CrowdState{NPCs: []domain.NPCState{{Wishes: []domain.Wish{{ID: card.ID, Want: 1}, {ID: card.ID, Want: 1}}}, {}, {}}}},
		{name: "a negative decline streak", state: domain.CrowdState{NPCs: []domain.NPCState{{DeclineStreak: -1}, {}, {}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := newCrowdWorld(t)
			if err := w.crowd.Restore(tt.state); err == nil {
				t.Error("Restore went through")
			}
		})
	}
}

// tradingWorld is a crowd that trades, whose NPCs mix the traits that keep a state of their own,
// with an Exchange and a player who holds two copies of every card, so that the NPCs wish for them.
type tradingWorld struct {
	world
	player   *domain.Collection
	exchange *domain.Exchange
}

func newTradingWorld(t *testing.T) *tradingWorld {
	t.Helper()
	w := &tradingWorld{world: world{src: rand.NewPCG(5, 5), counter: newWorldCounter(t)}, player: domain.NewCollection()}
	rng := rand.New(w.src)
	valuer := domain.NewValuer(domain.PriceSnapshot{}, 10)
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: 3, Valuer: valuer, MaxMarginPct: 1000, Archetypes: map[string]domain.ArchetypeSpec{
		"npc": {
			Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 4, KeepCopies: 1,
			Behaviour: "casual", WishSize: 4, MarginPct: 0, OfferChancePct: 20,
			Traits: []domain.TraitSpec{
				{Name: "ego", Params: map[string]int{"max_retries": 3, "endowment_pct_per_day": 5}},
				{Name: "gambler", Params: map[string]int{"chance_step_pct": 4, "max_offer_chance_pct": 30, "margin_extra_pct": 15, "margin_step_pct": 5}},
			},
		},
	}}, crowdSets(), rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	if w.crowd, err = domain.NewCrowd(npcs, crowdSets(), newOpenerOn(rng), w.counter, rng, domain.WithSurplusHold(2)); err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	for _, cat := range crowdSets() {
		for id, card := range cat.Cards {
			if !card.IsBasicLand {
				if err := w.player.Add(domain.CardKey{ID: id, Finish: domain.FinishNonFoil}, 2); err != nil {
					t.Fatalf("Add: %v", err)
				}
			}
		}
	}
	w.newExchange(t)
	return w
}

func (w *tradingWorld) newExchange(t *testing.T) {
	t.Helper()
	var err error
	w.exchange, err = domain.NewExchange(domain.ExchangeParams{
		ExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, MaxOffersPerDay: 20, CoinRate: 100,
	}, w.crowd, w.player, domain.NewJournal(), domain.NewValuer(domain.PriceSnapshot{}, 10))
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}
}

func (w *tradingWorld) state(t *testing.T) domain.CrowdState {
	t.Helper()
	s, err := w.crowd.State()
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	return s
}

// A crowd that trades, restored from the state of another, keeps each NPC's shelf, wishes and the
// moods its traits keep after a decline, so that it opens, sells, wishes and proposes as the other
// does from then on.
func TestCrowdRestore_continuesAsTheTradingCrowdItCameFrom(t *testing.T) {
	t.Parallel()
	a := newTradingWorld(t)
	a.tick(t, domain.TicksPerDay+30)
	a.exchange.StartDay(1)
	// The offer chance is below 100 so that the gambler has a chance to tilt.
	for range 20 {
		if err := a.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
	}
	offers := a.exchange.Offers()
	if len(offers) == 0 {
		t.Fatal("no NPC proposed, so no trait was told of a decline: the test proves nothing")
	}
	for _, o := range offers {
		if err := a.exchange.Decline(o.ID); err != nil {
			t.Fatalf("Decline: %v", err)
		}
	}
	a.tick(t, 40)
	state := a.state(t)

	b := newTradingWorld(t)
	b.restoreFrom(t, &a.world)
	if err := b.crowd.Restore(state); err != nil {
		t.Fatalf("crowd Restore: %v", err)
	}
	if got := b.state(t); !reflect.DeepEqual(got, state) {
		t.Fatalf("State after Restore = %+v, want %+v", got, state)
	}
	if b.crowd.CardsHeld() != a.crowd.CardsHeld() {
		t.Errorf("CardsHeld = %d restored, %d original", b.crowd.CardsHeld(), a.crowd.CardsHeld())
	}
	a.tick(t, domain.TicksPerDay+70)
	b.tick(t, domain.TicksPerDay+70)
	if got, want := b.state(t), a.state(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("a day and a half on, State = %+v, want %+v", got, want)
	}
	a.newExchange(t)
	b.newExchange(t)
	for _, w := range []*tradingWorld{a, b} {
		w.exchange.StartDay(1)
		if err := w.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
	}
	if got, want := b.exchange.Offers(), a.exchange.Offers(); !reflect.DeepEqual(got, want) {
		t.Errorf("offers after Restore = %+v, want %+v", got, want)
	}
}
