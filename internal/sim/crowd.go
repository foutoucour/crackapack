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
// The trade settings of the archetypes and the surplus hold apply only when trades are enabled, so
// a run without trades behaves as it did before they existed.
func newCrowd(cfg config.Config, openable []domain.SetCatalogue, counter *domain.Counter, valuer domain.Valuer) (*domain.Crowd, error) {
	if cfg.NPCs.Population == 0 {
		return nil, nil
	}
	archetypes := make(map[string]domain.ArchetypeSpec, len(cfg.NPCs.Archetypes))
	for name, a := range cfg.NPCs.Archetypes {
		spec := domain.ArchetypeSpec{
			Share:             a.Share,
			MinBoostersPerDay: a.MinBoostersPerDay,
			MaxBoostersPerDay: a.MaxBoostersPerDay,
			KeepCopies:        a.KeepCopies,
			FocusSet:          a.FocusSet,
		}
		if cfg.Trades.Enabled {
			spec.Behaviour = a.Behaviour
			spec.WishSize = a.WishSize
			spec.MarginPct = a.MarginPct
			spec.OfferChancePct = a.OfferChancePct
			spec.Colors = a.Colors
			spec.Traits = traitSpecs(a)
		}
		archetypes[name] = spec
	}

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^npcStreamSalt))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: cfg.NPCs.Population, Archetypes: archetypes, Valuer: valuer}, openable, rng)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	var opts []domain.CrowdOption
	if cfg.Trades.Enabled {
		opts = append(opts, domain.WithSurplusHold(cfg.NPCs.SurplusHoldDays))
	}
	crowd, err := domain.NewCrowd(npcs, openable, opener, counter, rng, opts...)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	return crowd, nil
}

// traitSpecs turns the trait names of an archetype, with their numbers, into the specs the domain
// builds its behaviour from.
func traitSpecs(a config.ArchetypeConfig) []domain.TraitSpec {
	specs := make([]domain.TraitSpec, 0, len(a.Traits))
	for _, name := range a.Traits {
		specs = append(specs, domain.TraitSpec{Name: name, Params: a.TraitParams[name]})
	}
	return specs
}
