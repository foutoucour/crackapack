package domain_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func behaviourScope(cat domain.SetCatalogue) []domain.Card {
	var scope []domain.Card
	for _, card := range cat.Cards {
		if !card.IsBasicLand {
			scope = append(scope, card)
		}
	}
	slices.SortFunc(scope, func(a, b domain.Card) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return scope
}

func behaviourNPC(t *testing.T, cat domain.SetCatalogue) *domain.NPC {
	t.Helper()
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"npc": {Share: 1, KeepCopies: 1}}},
		[]domain.SetCatalogue{cat}, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	return npcs[0]
}

func behaviourValuer(cat domain.SetCatalogue) domain.Valuer {
	return domain.NewValuer(domaintest.NewPrices(cat, 3), 10)
}

func TestBehavioursPickDistinctWishesTheNPCLacks(t *testing.T) {
	t.Parallel()
	for _, name := range domain.BehaviourNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cat := domaintest.NewRavnicaCatalogue()
			npc := behaviourNPC(t, cat)
			scope := behaviourScope(cat)
			held := scope[0]
			if err := npc.Collection().Add(domain.CardKey{ID: held.ID, Finish: domain.FinishNonFoil}, 4); err != nil {
				t.Fatalf("Add: %v", err)
			}
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: behaviourValuer(cat)})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}

			wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(2, 2)))

			if len(wishes) == 0 || len(wishes) > 5 {
				t.Fatalf("got %d wishes, want between 1 and 5", len(wishes))
			}
			seen := map[domain.CardID]bool{}
			for _, w := range wishes {
				if seen[w.ID] {
					t.Errorf("card %q wished twice", w.ID)
				}
				seen[w.ID] = true
				if w.ID == held.ID {
					t.Errorf("wished %q, which the NPC already holds in full", w.ID)
				}
				if w.Want < 1 {
					t.Errorf("wish %q wants %d copies, want at least 1", w.ID, w.Want)
				}
			}
		})
	}
}

func TestBehavioursSkipWishesAlreadyOnTheList(t *testing.T) {
	t.Parallel()
	for _, name := range domain.BehaviourNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cat := domaintest.NewRavnicaCatalogue()
			npc := behaviourNPC(t, cat)
			scope := behaviourScope(cat)
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: behaviourValuer(cat)})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}
			wished := map[domain.CardID]struct{}{}
			for _, card := range scope {
				wished[card.ID] = struct{}{}
			}

			if got := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope, Wished: wished}, 5, rand.New(rand.NewPCG(2, 2))); len(got) != 0 {
				t.Errorf("picked %v with every card already wished, want none", got)
			}
			if got := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 0, rand.New(rand.NewPCG(2, 2))); len(got) != 0 {
				t.Errorf("picked %v for n = 0, want none", got)
			}
		})
	}
}

func TestBehavioursPickTheSameWishesFromTheSameSeed(t *testing.T) {
	t.Parallel()
	for _, name := range domain.BehaviourNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cat := domaintest.NewRavnicaCatalogue()
			npc := behaviourNPC(t, cat)
			scope := behaviourScope(cat)
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: behaviourValuer(cat)})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}

			first := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 4, rand.New(rand.NewPCG(9, 9)))
			second := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 4, rand.New(rand.NewPCG(9, 9)))

			if !slices.Equal(first, second) {
				t.Errorf("same seed gave %v then %v", first, second)
			}
		})
	}
}

func TestPlaysetHunterWishesCommonsAndUncommonsByFour(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	b, err := domain.NewBehaviour("playset_hunter", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 15, rand.New(rand.NewPCG(2, 2)))

	if len(wishes) != 15 {
		t.Fatalf("got %d wishes, want all 15 commons and uncommons", len(wishes))
	}
	for _, w := range wishes {
		card := cat.Cards[w.ID]
		if card.Rarity != domain.RarityCommon && card.Rarity != domain.RarityUncommon {
			t.Errorf("wished %q, a %s", w.ID, card.Rarity)
		}
		if w.Want != domain.PlaysetSize {
			t.Errorf("wish %q wants %d, want a playset", w.ID, w.Want)
		}
	}
}

func TestWhaleWishesOnlyRaresAndSpecials(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	b, err := domain.NewBehaviour("whale", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 5, rand.New(rand.NewPCG(2, 2)))

	if len(wishes) == 0 {
		t.Fatal("no wish: the catalogue has a rare")
	}
	for _, w := range wishes {
		if r := cat.Cards[w.ID].Rarity; r != domain.RarityRare && r != domain.RaritySpecial {
			t.Errorf("wished %q, a %s", w.ID, r)
		}
	}
}

func TestSpeculatorWishesTheMostValuableCardsFirst(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	valuer := behaviourValuer(cat)
	b, err := domain.NewBehaviour("speculator", domain.BehaviourParams{MarginPct: 15, Valuer: valuer})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 6, rand.New(rand.NewPCG(2, 2)))

	for i := 1; i < len(wishes); i++ {
		if valuer.Price(wishes[i-1].ID) < valuer.Price(wishes[i].ID) {
			t.Errorf("wish %d (%q, %d) is worth more than wish %d (%q, %d)", i, wishes[i].ID, valuer.Price(wishes[i].ID), i-1, wishes[i-1].ID, valuer.Price(wishes[i-1].ID))
		}
	}
}

func TestTargetedCollectorWishesRarerCardsFirst(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	b, err := domain.NewBehaviour("targeted_collector", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 3, rand.New(rand.NewPCG(2, 2)))

	if len(wishes) != 3 {
		t.Fatalf("got %d wishes, want 3", len(wishes))
	}
	if r := cat.Cards[wishes[0].ID].Rarity; r != domain.RarityRare {
		t.Errorf("first wish is a %s, want the rare", r)
	}
	if r := cat.Cards[wishes[1].ID].Rarity; r != domain.RarityUncommon {
		t.Errorf("second wish is a %s, want an uncommon", r)
	}
}

func TestNewBehaviourRejectsAnUnknownNameAndANegativeMargin(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewBehaviour("gambler", domain.BehaviourParams{}); err == nil {
		t.Error("unknown behaviour accepted")
	}
	if _, err := domain.NewBehaviour("casual", domain.BehaviourParams{MarginPct: -1}); err == nil {
		t.Error("negative margin accepted: an NPC must never give more value than it gets")
	}
	if domain.KnownBehaviour("gambler") || !domain.KnownBehaviour("casual") {
		t.Error("KnownBehaviour disagrees with the registry")
	}
}

func TestBehaviourMarginIsTheConfiguredPercentage(t *testing.T) {
	t.Parallel()
	b, err := domain.NewBehaviour("speculator", domain.BehaviourParams{MarginPct: 15})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}
	if got := b.Margin(domain.Offer{}); got != 15 {
		t.Errorf("Margin = %d, want 15", got)
	}
}

func TestProposeBuildsAnOfferTheNPCWouldAccept(t *testing.T) {
	t.Parallel()
	wanted := domain.CardKey{ID: "wanted", Finish: domain.FinishNonFoil}
	cheap := domain.CardKey{ID: "cheap", Finish: domain.FinishNonFoil}
	pricey := domain.CardKey{ID: "pricey", Finish: domain.FinishNonFoil}
	valuer := domain.NewValuer(domain.PriceSnapshot{"wanted": 1000, "cheap": 400, "pricey": 700}, 10)
	ctx := domain.ProposalContext{
		Wanted:       []domain.TradeLine{{Key: wanted, Qty: 1}},
		Shelf:        []domain.TradeLine{{Key: cheap, Qty: 2}, {Key: pricey, Qty: 1}},
		PlayerCopies: func(id domain.CardID) int { return map[domain.CardID]int{"pricey": 1}[id] },
		MaxCards:     10,
	}

	tests := []struct {
		name     string
		margin   domain.Percent
		maxCards int
		wantGive []domain.TradeLine
		wantOK   bool
	}{
		{"fits at zero margin, the cards the player lacks first", 0, 10, []domain.TradeLine{{Key: cheap, Qty: 2}}, true},
		{"a margin shrinks what the NPC gives", 50, 10, []domain.TradeLine{{Key: cheap, Qty: 1}}, true},
		{"the card cap bounds the give side", 0, 1, []domain.TradeLine{{Key: cheap, Qty: 1}}, true},
		{"nothing fits a huge margin", 400, 10, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b, err := domain.NewBehaviour("casual", domain.BehaviourParams{MarginPct: tt.margin, Valuer: valuer})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}
			c := ctx
			c.MaxCards = tt.maxCards

			want, give, ok := b.Propose(c)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !slices.Equal(give, tt.wantGive) {
				t.Errorf("give = %v, want %v", give, tt.wantGive)
			}
			if ok && !slices.Equal(want, ctx.Wanted) {
				t.Errorf("want = %v, want the wanted lines %v", want, ctx.Wanted)
			}
		})
	}
}

func TestSpeculatorAsksForTheMostValuableDuplicateFirst(t *testing.T) {
	t.Parallel()
	low := domain.CardKey{ID: "low", Finish: domain.FinishNonFoil}
	high := domain.CardKey{ID: "high", Finish: domain.FinishNonFoil}
	shelf := domain.CardKey{ID: "shelf", Finish: domain.FinishNonFoil}
	valuer := domain.NewValuer(domain.PriceSnapshot{"low": 100, "high": 900, "shelf": 50}, 10)
	ctx := domain.ProposalContext{
		Wanted:       []domain.TradeLine{{Key: low, Qty: 1}, {Key: high, Qty: 1}},
		Shelf:        []domain.TradeLine{{Key: shelf, Qty: 3}},
		PlayerCopies: func(domain.CardID) int { return 0 },
		MaxCards:     1,
	}
	for name, wantFirst := range map[string]domain.CardKey{"casual": low, "speculator": high} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: valuer})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}

			want, _, ok := b.Propose(ctx)

			if !ok || len(want) != 1 || want[0].Key != wantFirst {
				t.Errorf("want = %v (ok %v), want the single line %v", want, ok, wantFirst)
			}
		})
	}
}
