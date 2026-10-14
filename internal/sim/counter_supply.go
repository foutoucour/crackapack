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
// no mode and so no supply.
func newCounterSupply(cfg config.Config, openable []domain.SetCatalogue, counter *domain.Counter) (*domain.CounterSupply, error) {
	if cfg.CounterSupply.Mode != config.CounterSupplyFixed {
		return nil, nil
	}
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^counterStreamSalt))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	supply, err := domain.NewFixedCounterSupply(counter, openable, opener, rng, cfg.CounterSupply.BoostersPerDay)
	if err != nil {
		return nil, fmt.Errorf("sim: counter supply: %w", err)
	}
	return supply, nil
}
