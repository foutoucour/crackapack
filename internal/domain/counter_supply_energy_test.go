package domain_test

import (
	"maps"
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// newEnergySupply builds a Counter that starts empty and a supply that pays the booster costs
// from its own energy, on a random stream of its own.
func newEnergySupply(t *testing.T, params domain.EnergyParams, costs map[string]int, seed uint64) (*domain.CounterSupply, *domain.Counter) {
	t.Helper()
	sets := crowdSets()
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 10})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	energy, err := domain.NewEnergy(params)
	if err != nil {
		t.Fatalf("NewEnergy: %v", err)
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	supply, err := domain.NewEnergyCounterSupply(counter, sets, opener, rng, energy, costs)
	if err != nil {
		t.Fatalf("NewEnergyCounterSupply: %v", err)
	}
	return supply, counter
}

func runSupplyTicks(t *testing.T, supply *domain.CounterSupply, counter *domain.Counter, ticks int) domain.SupplyTick {
	t.Helper()
	var total domain.SupplyTick
	for range ticks {
		counter.Tick()
		tick, err := supply.Tick()
		if err != nil {
			t.Fatalf("supply Tick: %v", err)
		}
		total.BoostersOpened += tick.BoostersOpened
		total.CardsOpened += tick.CardsOpened
	}
	return total
}

// stockOfSet is the copies of every card of set, in both finishes, that the Counter holds.
func stockOfSet(c *domain.Counter, set domain.SetCatalogue) int {
	var total int
	for id := range set.Cards {
		total += c.Stock(domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
		total += c.Stock(domain.CardKey{ID: id, Finish: domain.FinishFoil})
	}
	return total
}

func TestEnergyCounterSupplyNeverSpendsMoreThanItHas(t *testing.T) {
	t.Parallel()
	const (
		cost     = 4
		recharge = 4
		reserve  = 40
		ticks    = 100
	)
	supply, counter := newEnergySupply(t,
		domain.EnergyParams{RechargePerTick: recharge, ReserveCap: reserve, SavingsIntervalTicks: 1},
		map[string]int{"RAV": cost, "CLS": cost}, 1)

	opened := runSupplyTicks(t, supply, counter, ticks)

	available := reserve + recharge*ticks
	if spent := opened.BoostersOpened * cost; spent > available {
		t.Errorf("spent %d energy, want at most the %d it had", spent, available)
	}
	if spent := opened.BoostersOpened * cost; spent < available-2*cost {
		t.Errorf("spent %d energy, want nearly all of the %d it had", spent, available)
	}
}

func TestEnergyCounterSupplyWaitsForTheEnergyOfAnExpensiveBooster(t *testing.T) {
	t.Parallel()
	const cost = 12
	supply, counter := newEnergySupply(t,
		domain.EnergyParams{RechargePerTick: 1, ReserveCap: cost, SavingsIntervalTicks: 1},
		map[string]int{"RAV": cost, "CLS": cost}, 1)

	if got := runSupplyTicks(t, supply, counter, 1).BoostersOpened; got != 1 {
		t.Fatalf("opened %d boosters on the first tick, want 1: the reserve starts full", got)
	}
	if got := runSupplyTicks(t, supply, counter, cost-1).BoostersOpened; got != 0 {
		t.Errorf("opened %d boosters before the energy was back, want 0", got)
	}
	if got := runSupplyTicks(t, supply, counter, 1).BoostersOpened; got != 1 {
		t.Errorf("opened %d boosters once the energy was back, want 1", got)
	}
}

func TestEnergyCounterSupplyOpensCheapSetsMoreOften(t *testing.T) {
	t.Parallel()
	sets := crowdSets()
	supply, counter := newEnergySupply(t,
		domain.EnergyParams{RechargePerTick: 4, ReserveCap: 100, SavingsIntervalTicks: 1, SavingsCap: 100},
		map[string]int{sets[0].Code: 12, sets[1].Code: 3}, 1)

	runSupplyTicks(t, supply, counter, 20*domain.TicksPerDay)

	dear, cheap := stockOfSet(counter, sets[0]), stockOfSet(counter, sets[1])
	if dear == 0 {
		t.Fatal("the dear set was never opened")
	}
	// An equal share of energy opens a set that costs a quarter as much four times as often.
	if ratio := float64(cheap) / float64(dear); ratio < 3 || ratio > 5 {
		t.Errorf("cheap set stock %d against dear set stock %d, ratio %.1f, want about 4", cheap, dear, ratio)
	}
}

func TestEnergyCounterSupplyIsReproducibleFromTheSeed(t *testing.T) {
	t.Parallel()
	params := domain.EnergyParams{RechargePerTick: 5, ReserveCap: 50, SavingsIntervalTicks: 1}
	costs := map[string]int{"RAV": 5, "CLS": 8}
	a, ca := newEnergySupply(t, params, costs, 7)
	b, cb := newEnergySupply(t, params, costs, 7)

	runSupplyTicks(t, a, ca, 2*domain.TicksPerDay)
	runSupplyTicks(t, b, cb, 2*domain.TicksPerDay)

	if !maps.Equal(ca.StockByRarity(), cb.StockByRarity()) {
		t.Errorf("stock %v and %v differ for the same seed", ca.StockByRarity(), cb.StockByRarity())
	}
}

func TestNewEnergyCounterSupplyRejectsBadInput(t *testing.T) {
	t.Parallel()
	sets := crowdSets()
	rng := rand.New(rand.NewPCG(1, 1))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	energy, err := domain.NewEnergy(domain.EnergyParams{RechargePerTick: 1, ReserveCap: 10, SavingsIntervalTicks: 1})
	if err != nil {
		t.Fatalf("NewEnergy: %v", err)
	}
	tests := []struct {
		name   string
		energy *domain.Energy
		costs  map[string]int
	}{
		{name: "no energy", energy: nil, costs: map[string]int{"RAV": 5, "CLS": 5}},
		{name: "a set without a cost", energy: energy, costs: map[string]int{"RAV": 5}},
		{name: "a zero cost", energy: energy, costs: map[string]int{"RAV": 5, "CLS": 0}},
		{name: "a negative cost", energy: energy, costs: map[string]int{"RAV": -1, "CLS": 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.NewEnergyCounterSupply(counter, sets, opener, rng, tt.energy, tt.costs); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}
