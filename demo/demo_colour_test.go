package demo_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/domain"
)

// allWishesBehaviour picks every card in scope as a wish, up to n, so the scope is fully
// exercised. It is defined here for the colour test and does not mutate ctx.Scope.
type allWishesBehaviour struct{}

func (allWishesBehaviour) PickWishes(ctx domain.WishContext, n int, _ *rand.Rand) []domain.Wish {
	var wishes []domain.Wish
	for _, card := range ctx.Scope {
		if _, ok := ctx.Wished[card.ID]; ok {
			continue
		}
		wishes = append(wishes, domain.Wish{ID: card.ID, Want: 1})
		if len(wishes) >= n {
			break
		}
	}
	return wishes
}

func (allWishesBehaviour) Margin(domain.Offer) domain.Percent { return 0 }

func (allWishesBehaviour) Propose(domain.ProposalContext) (want, give []domain.TradeLine, ok bool) {
	return nil, nil, false
}

// TestBuild_demoCataloguesCarryColoursForAllFiveColours verifies that the demo dataset has cards
// in each of the five Magic colours, at least one gold card and at least one non-basic colourless
// card. These properties are needed for a colour-filtered crowd to have a non-trivial scope.
func TestBuild_demoCataloguesCarryColoursForAllFiveColours(t *testing.T) {
	t.Parallel()
	ds := demo.Build()

	dem, err := domain.LoadSetCatalogue(ds.Files, "DEM.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue DEM: %v", err)
	}
	dsp, err := domain.LoadSetCatalogue(ds.Files, "DSP.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue DSP: %v", err)
	}

	allCards := make([]domain.Card, 0, len(dem.Cards)+len(dsp.Cards))
	for _, c := range dem.Cards {
		allCards = append(allCards, c)
	}
	for _, c := range dsp.Cards {
		allCards = append(allCards, c)
	}

	// All five mono colours must appear in at least one card.
	seen := make(map[string]bool)
	for _, c := range allCards {
		for _, col := range c.Colors {
			seen[col] = true
		}
	}
	for _, want := range domain.ValidColors() {
		if !seen[want] {
			t.Errorf("colour %q is absent from all demo cards; want each of W U B R G present", want)
		}
	}

	// At least one gold card (multi-colour).
	hasGold := false
	for _, c := range allCards {
		if len(c.Colors) > 1 {
			hasGold = true
			break
		}
	}
	if !hasGold {
		t.Error("no gold card found in demo catalogues; want at least one multi-colour card")
	}

	// At least one non-basic colourless card.
	hasColourless := false
	for _, c := range allCards {
		if !c.IsBasicLand && len(c.Colors) == 0 {
			hasColourless = true
			break
		}
	}
	if !hasColourless {
		t.Error("no non-basic colourless card found in demo catalogues; want at least one")
	}
}

// TestBuild_colouredCrowdWishesOnlyForInScopeCards builds NPCs restricted to black and verifies
// that every wish they make is for a card whose colours are a subset of black. It uses the demo
// catalogues so that any catalogue change that breaks colour data is caught here.
func TestBuild_colouredCrowdWishesOnlyForInScopeCards(t *testing.T) {
	t.Parallel()
	ds := demo.Build()
	cats, _, _ := loadDemoInputs(t, ds)

	rng := rand.New(rand.NewPCG(42, 42))
	spec := domain.ArchetypeSpec{
		Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1,
		Colors:   []string{"B"},
		WishSize: 20,
	}
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 5, Archetypes: map[string]domain.ArchetypeSpec{"black": spec}},
		cats, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	for _, npc := range npcs {
		npc.Behaviour = allWishesBehaviour{}
	}

	counter, err := domain.NewCounter(cats, domain.PriceSnapshot{}, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{
		domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
	}, rng)
	_, err = domain.NewCrowd(npcs, cats, opener, counter, rng)
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}

	// Build a lookup of all cards across all catalogues.
	allCards := make(map[domain.CardID]domain.Card)
	for _, cat := range cats {
		for id, card := range cat.Cards {
			allCards[id] = card
		}
	}

	// Every wish must be for a card that is black-only or colourless.
	for _, npc := range npcs {
		for _, wish := range npc.Wishes() {
			card, ok := allCards[wish.ID]
			if !ok {
				t.Errorf("NPC %d wishes for %q which is not in the catalogues", npc.ID, wish.ID)
				continue
			}
			// Colourless cards are in scope for everyone.
			if len(card.Colors) == 0 {
				continue
			}
			// All colours of the card must be within the NPC's colours (B only).
			for _, col := range card.Colors {
				if col != "B" {
					t.Errorf("NPC %d (colors [B]) wishes for %q which has colour %q; "+
						"want only B and colourless cards in scope", npc.ID, wish.ID, col)
				}
			}
		}
	}
}
