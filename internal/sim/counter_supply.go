package sim

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// counterStreamSalt separates the Counter's random stream from the player's and the crowd's. All
// derive from cfg.Seed, so a seed still reproduces the whole run, but the Counter's draws never
// shift anyone else's.
const counterStreamSalt uint64 = 0xbf58476d1ce4e5b9

// newCounterSupply builds the supply described by cfg.CounterSupply on its own random stream. It
// returns nil unless a supply mode is set, which is a Counter that opens nothing. A zero Config has
// no mode and so no supply. In energy mode the Counter pays the same energy cost per set as the
// player, from boosterCosts.
func newCounterSupply(cfg config.Config, openable []domain.SetCatalogue, counter *domain.Counter, boosterCosts map[string]int, journal *domain.Journal, fees map[string]domain.Coins) (*domain.CounterSupply, error) {
	mode := cfg.CounterSupply.Mode
	if mode != config.CounterSupplyFixed && mode != config.CounterSupplyEnergy {
		return nil, nil
	}
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^counterStreamSalt))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)

	var (
		supply *domain.CounterSupply
		err    error
	)
	if mode == config.CounterSupplyFixed {
		supply, err = domain.NewFixedCounterSupply(counter, openable, opener, rng, cfg.CounterSupply.BoostersPerDay)
	} else {
		e := cfg.CounterSupply.Energy
		var energy *domain.Energy
		energy, err = domain.NewEnergy(domain.EnergyParams{
			RechargePerTick: e.RechargePerTick, ReserveCap: e.ReserveCap,
			SavingsIntervalTicks: e.SavingsIntervalTicks, SavingsCap: e.SavingsCap,
		})
		if err != nil {
			return nil, fmt.Errorf("sim: counter supply: energy: %w", err)
		}
		supply, err = domain.NewEnergyCounterSupply(counter, openable, opener, rng, energy, boosterCosts, domain.WithCoinFees(journal, fees))
	}
	if err != nil {
		return nil, fmt.Errorf("sim: counter supply: %w", err)
	}
	return supply, nil
}

// counterCoinFees returns the coin fee of one booster of each openable set, keyed by set code. A
// booster worth more than the energy cap costs its cash-out value, what the Counter would pay the
// player for its cards, minus what the cap is worth at coins_per_energy. With coins_per_energy at
// 0, or outside energy mode, no set has a fee.
//
// The cap is the dearest energy cost of the cost table, the cost a booster has once it passes the
// last tier.
func counterCoinFees(cfg config.Config, openable []domain.SetCatalogue, avgValues []domain.USDCents) (map[string]domain.Coins, error) {
	p := cfg.CounterSupply.CoinsPerEnergy
	if p <= 0 || cfg.CounterSupply.Mode != config.CounterSupplyEnergy {
		return nil, nil
	}
	var energyCap int
	for _, tier := range cfg.Energy.CostTable {
		energyCap = max(energyCap, tier.Cost)
	}
	capValue := domain.Coins(energyCap) * p
	fees := make(map[string]domain.Coins, len(openable))
	for i, cat := range openable {
		value, err := domain.PriceToCoins(avgValues[i], cfg.CoinRate, cfg.CounterSellPct)
		if err != nil && !errors.Is(err, domain.ErrSubCoin) {
			return nil, fmt.Errorf("sim: coin fee of %q: %w", cat.Code, err)
		}
		if value > capValue {
			fees[cat.Code] = value - capValue
		}
	}
	return fees, nil
}
