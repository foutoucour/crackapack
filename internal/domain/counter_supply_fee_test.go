package domain_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// newFeeSupply builds an energy supply that pays a coin fee per booster from a treasury funded with
// treasury coins. The energy is ample, so only the coins limit what it opens.
func newFeeSupply(t *testing.T, treasury domain.Coins, fees map[string]domain.Coins) (*domain.CounterSupply, *domain.Counter, *domain.Journal) {
	t.Helper()
	sets := crowdSets()
	j := domain.NewJournal(domain.AccountPlayer, domain.AccountCounterTreasury)
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, j, domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 10, Treasury: true})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	if treasury > 0 {
		mustPost(t, j, domain.Entry{From: domain.AccountCounterFloat, To: domain.AccountCounterTreasury, Amount: treasury})
	}
	energy, err := domain.NewEnergy(domain.EnergyParams{RechargePerTick: 50, ReserveCap: 200, SavingsIntervalTicks: 1, SavingsCap: 200})
	if err != nil {
		t.Fatalf("NewEnergy: %v", err)
	}
	rng := rand.New(rand.NewPCG(1, 1))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	supply, err := domain.NewEnergyCounterSupply(counter, sets, opener, rng, energy, map[string]int{"RAV": 4, "CLS": 4}, domain.WithCoinFees(j, fees))
	if err != nil {
		t.Fatalf("NewEnergyCounterSupply: %v", err)
	}
	return supply, counter, j
}

func TestEnergyCounterSupplyPaysItsCoinFeeFromTheTreasury(t *testing.T) {
	t.Parallel()
	supply, counter, j := newFeeSupply(t, 1000, map[string]domain.Coins{"RAV": 100, "CLS": 100})

	opened := runSupplyTicks(t, supply, counter, 100)

	if opened.BoostersOpened != 10 {
		t.Errorf("opened %d boosters, want 10: the treasury pays for ten fees of 100", opened.BoostersOpened)
	}
	if opened.FeeCoins != 1000 {
		t.Errorf("fees paid = %d, want 1000", opened.FeeCoins)
	}
	if got := j.Balance(domain.AccountCounterFee); got != 1000 {
		t.Errorf("fee account = %d, want 1000: the fees leave the economy", got)
	}
	if got := j.Balance(domain.AccountCounterTreasury); got != 0 {
		t.Errorf("treasury = %d, want 0", got)
	}
}

func TestEnergyCounterSupplyOpensOnlyTheBoostersItCanAfford(t *testing.T) {
	t.Parallel()
	sets := crowdSets()
	supply, counter, _ := newFeeSupply(t, 0, map[string]domain.Coins{"RAV": 100, "CLS": 0})

	runSupplyTicks(t, supply, counter, 20)

	if got := stockOfSet(counter, sets[0]); got != 0 {
		t.Errorf("stock of the set that costs coins = %d, want 0: the treasury is empty", got)
	}
	if got := stockOfSet(counter, sets[1]); got == 0 {
		t.Error("the set that costs no coins was never opened")
	}
}
