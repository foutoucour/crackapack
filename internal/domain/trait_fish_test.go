package domain_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// fishEnv builds a TraitEnv whose Valuer has known prices.
func fishEnv(prices map[domain.CardID]domain.USDCents) domain.TraitEnv {
	snap := make(domain.PriceSnapshot, len(prices))
	for id, p := range prices {
		snap[id] = p
	}
	return domain.TraitEnv{Valuer: domain.NewValuer(snap, 1)}
}

// buildFishTrait builds a fish trait directly from DefaultTraits.
func buildFishTrait(t *testing.T, params map[string]int, env domain.TraitEnv) domain.Trait {
	t.Helper()
	constructor, ok := domain.DefaultTraits()["fish"]
	if !ok {
		t.Fatal("fish not found in DefaultTraits")
	}
	tr, err := constructor(params, env)
	if err != nil {
		t.Fatalf("fish constructor: %v", err)
	}
	return tr
}

// --- Criterion 6: WishScore ordering ---

func TestFishWishScoreOrdering(t *testing.T) {
	t.Parallel()

	prices := map[domain.CardID]domain.USDCents{
		"cheap":      50,
		"expensive":  500,
		"cheap-rare": 50,
		"cheap-foil": 50,
	}
	env := fishEnv(prices)
	tr := buildFishTrait(t, map[string]int{
		"price_unit_cents": 100,
		"rarity_points":    5,
		"foil_points":      3,
	}, env)
	scorer, ok := tr.(domain.WishScorer)
	if !ok {
		t.Fatal("fish must implement WishScorer")
	}

	// Cards with controlled properties.
	cheap := domain.Card{ID: "cheap", Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}}
	expensive := domain.Card{ID: "expensive", Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}}
	// collectorRank(rare) = 2; score = 50/100 + 2*5 = 0 + 10 = 10; cheap common = 0.
	cheapRare := domain.Card{ID: "cheap-rare", Rarity: domain.RarityRare, Finishes: []domain.Finish{domain.FinishNonFoil}}
	// score = 50/100 + 3 foil = 0 + 3 = 3; cheap common = 0.
	cheapFoil := domain.Card{ID: "cheap-foil", Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil}}

	tests := []struct {
		name string
		a, b domain.Card
		msg  string
	}{
		{"pricier beats cheaper at equal rarity", expensive, cheap, "expensive(500) > cheap(50)"},
		{"rarer beats common at equal price", cheapRare, cheap, "rare(0+10) > common(0)"},
		{"foil beats non-foil at equal price and rarity", cheapFoil, cheap, "foil(0+3) > non-foil(0)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scoreA := scorer.WishScore(tt.a)
			scoreB := scorer.WishScore(tt.b)
			if scoreA <= scoreB {
				t.Errorf("%s: WishScore(%s)=%d must be > WishScore(%s)=%d",
					tt.msg, tt.a.ID, scoreA, tt.b.ID, scoreB)
			}
		})
	}
}

func TestFishUnsetParamsRankByPriceOnly(t *testing.T) {
	t.Parallel()
	prices := map[domain.CardID]domain.USDCents{
		"cheap":  50,
		"pricey": 500,
	}
	env := fishEnv(prices)
	// No rarity_points or foil_points set: only price matters.
	tr := buildFishTrait(t, map[string]int{}, env)
	scorer := tr.(domain.WishScorer)

	// cheap is mythic foil (would dominate if rarity/foil counted), pricey is common non-foil.
	// With no rarity_points/foil_points, only price_unit_cents (default 100) applies.
	// pricey: 500/100=5; cheap: 50/100=0 → pricey wins despite lower rarity.
	cheap := domain.Card{ID: "cheap", Rarity: domain.RarityMythic, Finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil}}
	pricey := domain.Card{ID: "pricey", Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}}
	if scorer.WishScore(pricey) <= scorer.WishScore(cheap) {
		t.Errorf("without rarity_points/foil_points, price-only ranking must apply: pricey(500) must beat cheap mythic foil(50)")
	}
}

func TestFishPickWishesTopNOrder(t *testing.T) {
	t.Parallel()
	// Build a composed speculator+fish behaviour and check that PickWishes returns the rare
	// (highest price) as the first wish.
	//
	// NewRavnicaCatalogue cards sorted alphabetically (non-basic, non-foil):
	//   [0-10] rav-c-000..rav-c-010  commons    (11 cards)
	//   [11]   rav-r-000              rare       (1 card)
	//   [12-15] rav-u-000..rav-u-003  uncommons  (4 cards)
	cat := domaintest.NewRavnicaCatalogue()
	scope := behaviourScope(cat)

	// Assign a high price to the rare card so fish scores it highest.
	snap := domaintest.NewPrices(cat, 1)
	snap["rav-r-000"] = 10000 // far above baseline, dominates score
	valuer := domain.NewValuer(snap, 1)
	env := domain.TraitEnv{Valuer: valuer}

	b, err := domain.NewBehaviour("speculator", domain.BehaviourParams{
		Valuer: valuer,
		Traits: []domain.TraitSpec{{Name: "fish", Params: map[string]int{
			"price_unit_cents": 100,
		}}},
		Registry:     domain.DefaultTraits(),
		Env:          env,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	npc := behaviourNPC(t, cat)
	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 3, rand.New(rand.NewPCG(1, 1)))
	if len(wishes) == 0 {
		t.Fatal("expected some wishes, got none")
	}
	// The first wish must be the rare: score = 10000/100 = 100, far ahead of commons.
	if wishes[0].ID != "rav-r-000" {
		t.Errorf("first wish = %q, want rav-r-000 (highest score)", wishes[0].ID)
	}
}

// --- Criterion 7: constructor errors ---

func TestFishConstructorErrors(t *testing.T) {
	t.Parallel()
	env := fishEnv(nil)

	tests := []struct {
		name    string
		params  map[string]int
		wantErr string
	}{
		{
			name:    "negative value",
			params:  map[string]int{"rarity_points": -1},
			wantErr: "rarity_points",
		},
		{
			name:    "unknown parameter",
			params:  map[string]int{"magic_bonus": 1},
			wantErr: "magic_bonus",
		},
		{
			name:    "price_unit_cents zero",
			params:  map[string]int{"price_unit_cents": 0},
			wantErr: "price_unit_cents",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			constructor := domain.DefaultTraits()["fish"]
			_, err := constructor(tt.params, env)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// --- Criterion 8: fish asks for the dearest duplicate first ---

// TestFishAsksForDearestDuplicateFirst verifies that the speculator base behaviour sorts the NPC's
// wish list by price descending before proposing, so the most expensive duplicate is requested
// even when the wish list was populated cheap-first.
func TestFishAsksForDearestDuplicateFirst(t *testing.T) {
	t.Parallel()
	spec := domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "speculator",
		WishSize:       2,
		MarginPct:      0,
		OfferChancePct: 100,
		Traits: []domain.TraitSpec{{Name: "fish", Params: map[string]int{
			"price_unit_cents": 100,
		}}},
	}
	// nil registry → DefaultTraits(), which includes "fish".
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{MaxCardsPerSide: 1}, domain.DefaultTraits(), 1000)

	// [0] rav-c-000 (cheap), [1] rav-c-001 (expensive), [11] rav-r-000 (NPC gives this).
	cheapCard := fx.keys[0]
	expensiveCard := fx.keys[1]
	npcGiveCard := fx.keys[11] // rare; NPC puts this on its shelf

	const cheapPrice domain.USDCents = 100
	const expPrice domain.USDCents = 500
	fx.setPrice(cheapCard, cheapPrice)
	fx.setPrice(expensiveCard, expPrice)
	fx.setPrice(npcGiveCard, expPrice) // budget: NPC can give expPrice worth of cards

	// Player holds 2 copies of each → both are duplicates (DuplicateKeep default 1).
	fx.give(t, cheapCard, 2)
	fx.give(t, expensiveCard, 2)

	// Set the NPC's wish list with cheap first, to prove the speculator re-orders by price.
	fx.crowd.SetWishesForTest(0, []domain.Wish{
		{ID: cheapCard.ID, Want: 1},
		{ID: expensiveCard.ID, Want: 1},
	})

	// NPC shelf: one copy of npcGiveCard (what the NPC gives to the player).
	fx.shelve(0, npcGiveCard, 1)

	fx.exchange.StartDay(1)
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}

	offers := fx.exchange.Offers()
	if len(offers) == 0 {
		t.Fatal("expected at least one NPC offer")
	}
	offer := offers[0]
	if len(offer.PlayerGives) == 0 {
		t.Fatal("offer has no PlayerGives")
	}
	// Speculator sorts ctx.Wanted by price descending; with MaxCardsPerSide=1, it picks expensive.
	if offer.PlayerGives[0].Key.ID != expensiveCard.ID {
		t.Errorf("PlayerGives[0] = %q, want %q (speculator base sorts by price desc)",
			offer.PlayerGives[0].Key.ID, expensiveCard.ID)
	}
}

// --- Criterion 9: fish offers the card the player lacks first ---

// TestFishOffersCardPlayerLacksFirst verifies inherited proposeSwap behaviour: shelf cards the
// player lacks come before those it already holds. Ordering NPCGives by price would need a new
// trait role and is intentionally out of scope (see story T-006 comment).
func TestFishOffersCardPlayerLacksFirst(t *testing.T) {
	t.Parallel()
	spec := domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "speculator",
		WishSize:       2,
		MarginPct:      0,
		OfferChancePct: 100,
		Traits: []domain.TraitSpec{{Name: "fish", Params: map[string]int{
			"price_unit_cents": 100,
		}}},
	}
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{MaxCardsPerSide: 1}, domain.DefaultTraits(), 1000)

	// [0] rav-c-000 (player holds one copy — not a duplicate),
	// [1] rav-c-001 (player lacks it entirely),
	// [12] rav-u-000 (what the NPC wishes for; player holds 2 copies → 1 duplicate tradeable).
	held := fx.keys[0]
	lacking := fx.keys[1]
	wishCard := fx.keys[12]

	const price domain.USDCents = 200
	fx.setPrice(held, price)
	fx.setPrice(lacking, price)
	fx.setPrice(wishCard, price)

	// Player holds wishCard in duplicate (NPC can ask for it) and held (not a duplicate).
	fx.give(t, wishCard, 2)
	fx.give(t, held, 1)

	// NPC wishes for wishCard; its shelf has both held and lacking at equal prices.
	fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: wishCard.ID, Want: 1}})
	fx.shelve(0, held, 1)
	fx.shelve(0, lacking, 1)

	fx.exchange.StartDay(1)
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}

	offers := fx.exchange.Offers()
	if len(offers) == 0 {
		t.Fatal("expected at least one NPC offer")
	}
	offer := offers[0]
	if len(offer.NPCGives) == 0 {
		t.Fatal("offer has no NPCGives")
	}
	// proposeSwap sorts shelf by playerHolds ascending (0 = lacks first).
	// With MaxCardsPerSide=1, the NPC gives the card the player lacks.
	if offer.NPCGives[0].Key.ID != lacking.ID {
		t.Errorf("NPCGives[0] = %q, want %q (player lacks it; inherited proposeSwap behaviour)",
			offer.NPCGives[0].Key.ID, lacking.ID)
	}
}
