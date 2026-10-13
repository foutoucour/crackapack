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
func newCrowd(cfg config.Config, openable []domain.SetCatalogue, counter *domain.Counter) (*domain.Crowd, error) {
	if cfg.NPCs.Population == 0 {
		return nil, nil
	}
	archetypes := make(map[string]domain.ArchetypeSpec, len(cfg.NPCs.Archetypes))
	for name, a := range cfg.NPCs.Archetypes {
		archetypes[name] = domain.ArchetypeSpec{
			Share:             a.Share,
			MinBoostersPerDay: a.MinBoostersPerDay,
			MaxBoostersPerDay: a.MaxBoostersPerDay,
			KeepCopies:        a.KeepCopies,
			FocusSet:          a.FocusSet,
		}
	}

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^npcStreamSalt))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: cfg.NPCs.Population, Archetypes: archetypes}, openable, rng)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	crowd, err := domain.NewCrowd(npcs, openable, opener, counter, rng)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	return crowd, nil
}
