package sim

import (
	"fmt"
	"math/rand/v2"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// npcStreamSalt separates the crowd's random stream from the player's. Both derive from
// cfg.Seed, so a seed still reproduces the whole run, but the crowd's draws never shift the
// player's: adding NPCs to a config leaves what the player opens unchanged.
const npcStreamSalt uint64 = 0x9e3779b97f4a7c15

// newCrowd builds the NPC crowd described by cfg.NPCs on its own random stream. It returns nil
// when the population is zero, which is a run without NPCs.
//
// The trade settings of the archetypes and the surplus hold apply only when trading is true: the
// config enables trades, or the player uses the market. A run without either behaves as it did
// before they existed.
//
// registry is the trait registry to inject; nil means domain.DefaultTraits.
func newCrowd(cfg config.Config, openable []domain.SetCatalogue, counter *domain.Counter, valuer domain.Valuer, registry domain.TraitRegistry, trading bool) (*domain.Crowd, error) {
	if cfg.NPCs.Population == 0 {
		return nil, nil
	}
	archetypes := cfg.NPCs.ArchetypeSpecs(trading)

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^npcStreamSalt))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{
		Size:          cfg.NPCs.Population,
		Archetypes:    archetypes,
		Valuer:        valuer,
		TraitRegistry: registry,
		MaxMarginPct:  cfg.NPCs.MaxMarginPct,
	}, openable, rng)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	var opts []domain.CrowdOption
	if trading {
		opts = append(opts, domain.WithSurplusHold(cfg.NPCs.SurplusHoldDays))
	}
	crowd, err := domain.NewCrowd(npcs, openable, opener, counter, rng, opts...)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	return crowd, nil
}
