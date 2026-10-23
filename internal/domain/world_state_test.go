package domain_test

import (
	"fmt"
	"math/rand/v2"
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

// The state of a crowd that trades, with wishes and shelves, is not kept yet: State says so
// rather than saving half of it.
func TestCrowdState_refusesACrowdThatTrades(t *testing.T) {
	t.Parallel()
	w := &world{src: rand.NewPCG(5, 5), counter: newWorldCounter(t)}
	rng := rand.New(w.src)
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{
		"npc": {Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 2, KeepCopies: 1},
	}}, crowdSets(), rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	crowd, err := domain.NewCrowd(npcs, crowdSets(), newOpenerOn(rng), w.counter, rng, domain.WithSurplusHold(1))
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	if _, err := crowd.State(); err == nil {
		t.Error("State of a crowd that holds its surplus went through")
	}
}
