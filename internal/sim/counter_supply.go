package sim

import (
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

// counterCoinFees returns the coin fee of each openable set for the Counter, see coinFees. Outside
// energy mode no set has a fee.
func counterCoinFees(cfg config.Config, openable []domain.SetCatalogue, avgValues []domain.USDCents) (map[string]domain.Coins, error) {
	if cfg.CounterSupply.Mode != config.CounterSupplyEnergy {
		return nil, nil
	}
	return coinFees(cfg.CounterSupply.CoinsPerEnergy, energyCap(cfg.Energy.CostTable), cfg.CoinRate, cfg.CounterSellPct, openable, avgValues)
}
