package sim_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// TestRun_everyEmbeddedArchetypeTrades runs each embedded archetype alone, with its own numbers,
// and requires an accepted trade. It proves that no shipped archetype is inert: a bad colour scope,
// an unreachable wish list or a margin no player offer can meet would leave it with no trade.
//
// A trade can start from either side, and an archetype need not suit both. A player that only
// accepts what NPCs offer reaches the archetypes that propose, and the trader policy, which
// proposes swaps for the cards NPCs shelve, reaches the ones that rarely propose. The playset
// hunter keeps four copies, so it has almost nothing on its shelf for the trader. Each archetype
// must trade with at least one of the two.
//
// Each archetype runs alone because a mixed crowd does not show it. The trader policy reaches the
// NPCs that hold a wanted card in ascending ID order and stops after a few proposals a session, so
// the archetypes with the low IDs and the most NPCs use the whole budget, see docs/todo.md.
//
// The subtests run one after another on purpose: a run is not cheap, and they must not pile up.
func TestRun_everyEmbeddedArchetypeTrades(t *testing.T) {
	loaded, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, name := range slices.Sorted(maps.Keys(loaded.NPCs.Archetypes)) {
		t.Run(name, func(t *testing.T) {
			cfg := traitArchetypeBase(10)
			archetype := loaded.NPCs.Archetypes[name]
			archetype.Share = 1
			cfg.NPCs.Archetypes[name] = archetype
			cfg.RunDays = 30

			var accepted int
			for _, inner := range []sim.PlayerPolicy{
				buyShelfPolicy{t: t, accepted: &accepted},
				sim.NewTraderPolicy(),
			} {
				runResultWithRegistry(t, cfg,
					probePolicy{inner: inner, t: t, accepted: &accepted},
					domain.DefaultTraits(),
				)
			}

			if accepted == 0 {
				t.Errorf("archetype %q made no accepted trade in %d days", name, cfg.RunDays)
			}
		})
	}
}
