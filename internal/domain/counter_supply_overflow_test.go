package domain_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// Costs whose least common multiple does not fit in an int64 used to make the weights wrap, and
// the first pick then panicked. The constructor must refuse them instead.
func TestNewEnergyCounterSupplyRejectsCostsWhoseWeightsOverflow(t *testing.T) {
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
	costs := map[string]int{sets[0].Code: 4_000_000_007, sets[1].Code: 4_000_000_009}

	_, err = domain.NewEnergyCounterSupply(counter, sets, opener, rng, energy, costs)

	if err == nil || !strings.Contains(err.Error(), "costs") {
		t.Errorf("error = %v, want one naming the costs", err)
	}
}
