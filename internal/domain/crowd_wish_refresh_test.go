package domain_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// countingBehaviour is a TradeBehaviour that finds nothing to wish for and counts how often it is
// asked, to show when the crowd asks.
type countingBehaviour struct{ calls *int }

func (b countingBehaviour) PickWishes(domain.WishContext, int, *rand.Rand) []domain.Wish {
	*b.calls++
	return nil
}

func (countingBehaviour) Margin(domain.Offer) domain.Percent { return 0 }

func (countingBehaviour) Propose(domain.ProposalContext) (want, give []domain.TradeLine, ok bool) {
	return nil, nil, false
}

func TestCrowdAsksForWishesOnlyWhenAWishWasFulfilled(t *testing.T) {
	t.Parallel()
	sets := crowdSets()
	rng := rand.New(rand.NewPCG(1, 1))
	spec := fixedSpec(10, 1)
	spec.WishSize = 3
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: 2, Archetypes: map[string]domain.ArchetypeSpec{"npc": spec}}, sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	var calls int
	for _, npc := range npcs {
		npc.Behaviour = countingBehaviour{calls: &calls}
	}
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 10})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	crowd, err := domain.NewCrowd(npcs, sets, opener, counter, rng, domain.WithSurplusHold(2))
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}

	runDays(t, crowd, 3)

	if want := len(npcs); calls != want {
		t.Errorf("PickWishes was called %d times over 3 days, want %d (the first fill of each NPC): "+
			"an NPC with nothing left to wish for must not rescan its scope after every booster", calls, want)
	}
}
