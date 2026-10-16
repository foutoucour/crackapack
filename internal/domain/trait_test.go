package domain_test

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// --- Criterion 1: no traits → returns base unwrapped ---

func TestNewBehaviourWithoutTraitsReturnsTheBase(t *testing.T) {
	t.Parallel()
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}
	// The returned type must be the base, not the traitBehaviour wrapper.
	// We verify this indirectly: the type should NOT implement VetoesCounter.
	type counterVetoer interface{ VetoesCounter(domain.Offer) bool }
	if _, ok := b.(counterVetoer); ok {
		t.Error("a behaviour with no traits must not be wrapped: got a traitBehaviour type")
	}
}

func TestGeneratePopulationAppliesTraitsFromTheSpec(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	var marginAdded domain.Percent = 10
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	spec := domain.ArchetypeSpec{
		Share:      1,
		KeepCopies: 1,
		Behaviour:  "casual",
		WishSize:   5,
		Traits:     []domain.TraitSpec{{Name: "margin", Params: map[string]int{"add": int(marginAdded)}}},
	}
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{
			Size:          1,
			Archetypes:    map[string]domain.ArchetypeSpec{"npc": spec},
			TraitRegistry: reg,
			MaxMarginPct:  1000,
		},
		[]domain.SetCatalogue{cat}, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}

	got := npcs[0].Behaviour.Margin(domain.Offer{})
	// base casual margin = 0, + 10 from trait = 10
	if got != marginAdded {
		t.Errorf("Margin = %d, want %d (base 0 + trait add %d)", got, marginAdded, marginAdded)
	}
}

// --- Criterion 2: optional interfaces ---

func TestTraitInterfacesAreOptional(t *testing.T) {
	t.Parallel()
	// A trait that only implements MarginAdder must not affect wishes or veto.
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	cat := domaintest.NewRavnicaCatalogue()
	scope := behaviourScope(cat)
	npc := behaviourNPC(t, cat)
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "margin", Params: map[string]int{"add": 5}}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	// Wishes are picked (margin trait doesn't score wishes)
	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 3, rand.New(rand.NewPCG(1, 1)))
	if len(wishes) == 0 {
		t.Error("expected some wishes from the base, got none")
	}
	// Veto does not apply (MarginTrait doesn't implement CounterVeto)
	type counterVetoer interface{ VetoesCounter(domain.Offer) bool }
	if cv, ok := b.(counterVetoer); ok {
		if cv.VetoesCounter(domain.Offer{}) {
			t.Error("a margin-only trait must not veto a counter")
		}
	}
}

func TestDeclineReactorFansOutToEveryTrait(t *testing.T) {
	t.Parallel()
	// Build two DeclineSpies and verify both receive the decline context.
	spy1 := &domaintest.DeclineSpy{}
	spy2 := &domaintest.DeclineSpy{}
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits: []domain.TraitSpec{
			{Name: "spy1"},
			{Name: "spy2"},
		},
		Registry: domain.TraitRegistry{
			"spy1": domaintest.NewDeclineSpy(spy1),
			"spy2": domaintest.NewDeclineSpy(spy2),
		},
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}
	type declineReactor interface{ ReactToDecline(domain.DeclineContext) }
	reactor, ok := b.(declineReactor)
	if !ok {
		t.Fatal("composed behaviour must implement ReactToDecline")
	}
	ctx := domain.DeclineContext{Offer: domain.Offer{ID: 42, NPC: 0}}
	reactor.ReactToDecline(ctx)

	if len(spy1.Calls) != 1 || spy1.Calls[0].Context.Offer.ID != ctx.Offer.ID {
		t.Errorf("spy1: got %v, want one call with offer ID %d", spy1.Calls, ctx.Offer.ID)
	}
	if len(spy2.Calls) != 1 || spy2.Calls[0].Context.Offer.ID != ctx.Offer.ID {
		t.Errorf("spy2: got %v, want one call with offer ID %d", spy2.Calls, ctx.Offer.ID)
	}
}

// --- Criterion 3: margin is base + sum, order-independent ---

func TestMarginIsBasePlusSumOfTraitAdders(t *testing.T) {
	t.Parallel()
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		MarginPct: 5,
		Traits: []domain.TraitSpec{
			{Name: "margin", Params: map[string]int{"add": 3}},
		},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	got := b.Margin(domain.Offer{})
	if got != 8 {
		t.Errorf("Margin = %d, want 8 (base 5 + trait 3)", got)
	}
}

func TestMarginAdderOrderIndependence(t *testing.T) {
	t.Parallel()
	// Three adders with different values; all permutations must give the same total.
	reg := domain.TraitRegistry{
		"t3":  func(_ map[string]int) (domain.Trait, error) { return &fixedMarginTrait{add: 3}, nil },
		"t7":  func(_ map[string]int) (domain.Trait, error) { return &fixedMarginTrait{add: 7}, nil },
		"t11": func(_ map[string]int) (domain.Trait, error) { return &fixedMarginTrait{add: 11}, nil },
	}

	names := []string{"t3", "t7", "t11"}
	perms := permutations(names)
	var results []domain.Percent
	for _, perm := range perms {
		specs := make([]domain.TraitSpec, len(perm))
		for i, n := range perm {
			specs[i] = domain.TraitSpec{Name: n}
		}
		b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
			MarginPct:    0,
			Traits:       specs,
			Registry:     reg,
			MaxMarginPct: 1000,
		})
		if err != nil {
			t.Fatalf("NewBehaviour: %v", err)
		}
		results = append(results, b.Margin(domain.Offer{}))
	}
	for i, got := range results {
		if got != results[0] {
			t.Errorf("permutation %d gives Margin = %d, want %d", i, got, results[0])
		}
	}
}

func TestMarginCapNeverLowersBase(t *testing.T) {
	t.Parallel()
	// cap = 5, base = 10, add = 3 → should return 10 (cap can't lower base)
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		MarginPct: 10,
		Traits: []domain.TraitSpec{
			{Name: "margin", Params: map[string]int{"add": 3}},
		},
		Registry:     reg,
		MaxMarginPct: 5, // below base
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	got := b.Margin(domain.Offer{})
	if got != 10 {
		t.Errorf("Margin = %d, want 10 (cap %d is below base %d, base wins)", got, 5, 10)
	}
}

func TestMarginCapOfZeroLeavesTraitsNoRoom(t *testing.T) {
	t.Parallel()
	// An unset cap is 0. It must not read as "no cap", or a config that forgets the key would
	// switch the guard against stacked adders off.
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		MarginPct: 10,
		Traits: []domain.TraitSpec{
			{Name: "margin", Params: map[string]int{"add": 20}},
		},
		Registry: reg,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	got := b.Margin(domain.Offer{})
	if got != 10 {
		t.Errorf("Margin = %d, want 10 (a cap of 0 leaves no room for the trait's +20)", got)
	}
}

func TestMarginCapBoundsTraitAdders(t *testing.T) {
	t.Parallel()
	// cap = 15, base = 10, add = 20 → should return 15 (cap limits sum to 5)
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		MarginPct: 10,
		Traits: []domain.TraitSpec{
			{Name: "margin", Params: map[string]int{"add": 20}},
		},
		Registry:     reg,
		MaxMarginPct: 15,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	got := b.Margin(domain.Offer{})
	if got != 15 {
		t.Errorf("Margin = %d, want 15 (base 10 + min(20, room 5) = 15)", got)
	}
}

// --- Criterion 4: negative param rejected ---

func TestNewBehaviourRejectsNegativeTraitParam(t *testing.T) {
	t.Parallel()
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits: []domain.TraitSpec{
			{Name: "margin", Params: map[string]int{"add": -1}},
		},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err == nil {
		t.Error("negative param must be rejected")
	}
}

func TestNewBehaviourRejectsDuplicateTrait(t *testing.T) {
	t.Parallel()
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "margin"}, {Name: "margin"}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err == nil {
		t.Error("duplicate trait name must be rejected")
	}
}

func TestNewBehaviourRejectsNegativeMaxMarginPct(t *testing.T) {
	t.Parallel()
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "margin"}},
		Registry:     reg,
		MaxMarginPct: -1,
	})
	if err == nil {
		t.Error("negative MaxMarginPct must be rejected")
	}
}

func TestNegativeAdderIsClamped(t *testing.T) {
	t.Parallel()
	// A trait returning a negative MarginAdd result must not lower the margin.
	reg := domain.TraitRegistry{
		"negative": func(_ map[string]int) (domain.Trait, error) {
			return &fixedMarginTrait{add: -5}, nil
		},
	}
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		MarginPct:    10,
		Traits:       []domain.TraitSpec{{Name: "negative"}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	got := b.Margin(domain.Offer{})
	if got != 10 {
		t.Errorf("Margin = %d, want 10: a negative adder result must be clamped at 0", got)
	}
}

// --- Criterion 5: wish ranking is order-independent ---

func TestWishRankingSumsTraitScores(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	scope := behaviourScope(cat)
	npc := behaviourNPC(t, cat)

	// Pick a card to be the top scorer.
	target := scope[0]
	scores := map[domain.CardID]int{target.ID: 100}
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "score", Constructor: domaintest.NewScoreTrait(scores)},
	)
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "score"}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 3, rand.New(rand.NewPCG(1, 1)))

	if len(wishes) == 0 {
		t.Fatal("no wishes returned")
	}
	if wishes[0].ID != target.ID {
		t.Errorf("first wish = %q, want the scored card %q", wishes[0].ID, target.ID)
	}
}

func TestWishRankingIsOrderIndependent(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	scope := behaviourScope(cat)
	npc := behaviourNPC(t, cat)
	target := scope[0]
	scores := map[domain.CardID]int{target.ID: 100}

	// Two registries: [score, score2] vs [score2, score] (score2 scores 0 everywhere)
	makeReg := func(first, second string) domain.TraitRegistry {
		return domain.TraitRegistry{
			"score":  domaintest.NewScoreTrait(scores),
			"score2": domaintest.NewScoreTrait(nil),
		}
	}

	traitOrders := [][]string{
		{"score", "score2"},
		{"score2", "score"},
	}
	var results [][]domain.Wish
	for _, order := range traitOrders {
		specs := make([]domain.TraitSpec, len(order))
		for i, n := range order {
			specs[i] = domain.TraitSpec{Name: n}
		}
		b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
			Traits:       specs,
			Registry:     makeReg(order[0], order[1]),
			MaxMarginPct: 1000,
		})
		if err != nil {
			t.Fatalf("NewBehaviour (%v): %v", order, err)
		}
		wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 3, rand.New(rand.NewPCG(1, 1)))
		results = append(results, wishes)
	}

	if !slices.EqualFunc(results[0], results[1], func(a, b domain.Wish) bool { return a.ID == b.ID }) {
		t.Errorf("wish order %v vs %v give different results:\n%v\n%v", traitOrders[0], traitOrders[1], results[0], results[1])
	}
}

func TestWishEqualScoresKeepBaseOrder(t *testing.T) {
	t.Parallel()
	// When all cards score 0, the order must match the base.
	cat := domaintest.NewRavnicaCatalogue()
	scope := behaviourScope(cat)
	npc := behaviourNPC(t, cat)
	rngSeed := rand.New(rand.NewPCG(1, 1))

	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "score", Constructor: domaintest.NewScoreTrait(nil)},
	)

	base, err := domain.NewBehaviour("casual", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("base NewBehaviour: %v", err)
	}
	composed, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "score"}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("composed NewBehaviour: %v", err)
	}

	// Same seed for both.
	baseWishes := base.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(1, 1)))
	_ = rngSeed
	composedWishes := composed.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(1, 1)))

	if !slices.EqualFunc(baseWishes, composedWishes, func(a, b domain.Wish) bool { return a.ID == b.ID }) {
		t.Errorf("equal scores: composed order %v differs from base order %v", composedIDs(composedWishes), composedIDs(baseWishes))
	}
}

func TestNoScorersMatchesBaseWithSameSeed(t *testing.T) {
	t.Parallel()
	// A scoring-less traitBehaviour must delegate PickWishes straight to base.
	cat := domaintest.NewRavnicaCatalogue()
	scope := behaviourScope(cat)
	npc := behaviourNPC(t, cat)

	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	base, err := domain.NewBehaviour("casual", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("base NewBehaviour: %v", err)
	}
	composed, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "margin", Params: map[string]int{"add": 5}}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("composed NewBehaviour: %v", err)
	}

	seed := uint64(7)
	baseWishes := base.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(seed, seed)))
	composedWishes := composed.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(seed, seed)))

	if !slices.EqualFunc(baseWishes, composedWishes, func(a, b domain.Wish) bool { return a.ID == b.ID }) {
		t.Errorf("margin-only trait changed wish order: base %v, composed %v",
			composedIDs(baseWishes), composedIDs(composedWishes))
	}
}

func TestWishBasesWantIsKept(t *testing.T) {
	t.Parallel()
	// The trait must not change the Want count that the base set.
	cat := domaintest.NewRavnicaCatalogue()
	scope := behaviourScope(cat)
	npc := behaviourNPC(t, cat)
	target := scope[0]
	scores := map[domain.CardID]int{target.ID: 100}
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "score", Constructor: domaintest.NewScoreTrait(scores)},
	)
	base, err := domain.NewBehaviour("casual", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("base NewBehaviour: %v", err)
	}
	composed, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "score"}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("composed NewBehaviour: %v", err)
	}
	baseWishes := base.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(1, 1)))
	composedWishes := composed.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(1, 1)))

	baseWant := make(map[domain.CardID]int, len(baseWishes))
	for _, w := range baseWishes {
		baseWant[w.ID] = w.Want
	}
	for _, w := range composedWishes {
		if want, ok := baseWant[w.ID]; ok && w.Want != want {
			t.Errorf("card %q: composed Want = %d, base Want = %d; trait must not change Want", w.ID, w.Want, want)
		}
	}
}

// --- Criterion 6: veto integration via Exchange ---

// vetoFixture creates an exchange fixture where the NPC has a VetoTrait, wishes for keys[0]
// (worth 500), holds three copies of keys[1] (worth 100 each) on the shelf, and always proposes.
// The player holds two copies of keys[0] so one is a duplicate.
func vetoFixture(t *testing.T) *exchangeFixture {
	t.Helper()
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "veto", Constructor: domaintest.NewVetoTrait()},
	)
	spec := domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "casual",
		OfferChancePct: 100,
		Traits:         []domain.TraitSpec{{Name: "veto"}},
	}
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{}, reg, 1000)
	fx.setPrice(fx.keys[0], 500)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 2)
	fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
	fx.shelve(0, fx.keys[1], 3)
	fx.exchange.StartDay(1)
	return fx
}

func TestCounterRefusedWhenTraitVetoes(t *testing.T) {
	t.Parallel()
	fx := vetoFixture(t)

	// NPC proposes (OriginNPC).
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offers := fx.exchange.Offers()
	if len(offers) != 1 || offers[0].Origin != domain.OriginNPC {
		t.Fatalf("expected one OriginNPC offer, got %v", offers)
	}
	npcOffer := offers[0]

	// Player counters the NPC's own proposal; the trait must veto this.
	replacement := fx.keys[2]
	fx.setPrice(replacement, 500)
	fx.give(t, replacement, 1)

	before := fx.state(0) // capture after giving the replacement card

	d, err := fx.exchange.Counter(npcOffer.ID, lineOf(replacement, 1), 0)
	if err != nil {
		t.Fatalf("Counter: %v", err)
	}

	if d.Outcome != domain.OutcomeRefused {
		t.Errorf("Outcome = %v, want refused (veto)", d.Outcome)
	}
	if !errors.Is(d.Reason, domain.ErrCounterVetoed) {
		t.Errorf("Reason = %v, want ErrCounterVetoed", d.Reason)
	}
	// State is unchanged (nothing moved, no offer stored, countered stays 0).
	fx.requireUnchanged(t, 0, before)
	if len(fx.exchange.Offers()) != 0 {
		t.Error("vetoed counter must not store an open offer")
	}
	stats := fx.exchange.TakeDayStats()
	if stats.Countered != 0 {
		t.Errorf("Countered = %d, want 0: a vetoed counter is a refusal, not a counter", stats.Countered)
	}
}

func TestVetoAlsoAppliesWhenTopUpCoversMargin(t *testing.T) {
	t.Parallel()
	fx := vetoFixture(t)
	fx.fund(t, 1000) // player has plenty of coins

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offers := fx.exchange.Offers()
	if len(offers) != 1 {
		t.Fatalf("expected 1 NPC offer, got %d", len(offers))
	}
	npcOffer := offers[0]

	// Counter with lots of coins; veto must still refuse.
	replacement := fx.keys[2]
	fx.setPrice(replacement, 500)
	fx.give(t, replacement, 1)
	before := fx.state(0) // capture after giving the replacement card

	d, err := fx.exchange.Counter(npcOffer.ID, lineOf(replacement, 1), 500)
	if err != nil {
		t.Fatalf("Counter: %v", err)
	}
	if d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrCounterVetoed) {
		t.Errorf("Outcome = %v, Reason = %v; want refused/ErrCounterVetoed even with sufficient top-up", d.Outcome, d.Reason)
	}
	fx.requireUnchanged(t, 0, before)
}

func TestPlayerProposeIsNotVetoed(t *testing.T) {
	t.Parallel()
	// Even with a VetoTrait, Exchange.Propose (canCounter=true) must not trigger the veto.
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "veto", Constructor: domaintest.NewVetoTrait()},
	)
	spec := domain.ArchetypeSpec{
		Share:      1,
		KeepCopies: 1,
		Behaviour:  "casual",
		MarginPct:  0,
		Traits:     []domain.TraitSpec{{Name: "veto"}},
	}
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{}, reg, 1000)
	mine, theirs := fx.keys[0], fx.keys[1]
	fx.setPrice(mine, 100)
	fx.setPrice(theirs, 100)
	fx.shelve(0, theirs, 1)
	fx.give(t, mine, 1)
	fx.exchange.StartDay(1)

	d, err := fx.exchange.Propose(0, lineOf(mine, 1), lineOf(theirs, 1), 0)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	// At zero margin, a fair swap should be accepted—NOT vetoed.
	if d.Outcome != domain.OutcomeAccepted {
		t.Errorf("Outcome = %v (reason %v), want accepted: Exchange.Propose must not be vetoed", d.Outcome, d.Reason)
	}
}

func TestNPCCountersAtMostOnceWithTrait(t *testing.T) {
	t.Parallel()
	// Verify ADR-0012 still holds with traits: NPC counters once via the Propose path.
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
	)
	spec := domain.ArchetypeSpec{
		Share:      1,
		KeepCopies: 1,
		Behaviour:  "casual",
		MarginPct:  20,
		Traits:     []domain.TraitSpec{{Name: "margin", Params: map[string]int{"add": 5}}},
	}
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{}, reg, 1000)
	mine, theirs := fx.keys[0], fx.keys[1]
	fx.setPrice(mine, 100)
	fx.setPrice(theirs, 100)
	fx.shelve(0, theirs, 1)
	fx.give(t, mine, 1)
	fx.exchange.StartDay(1)

	// Player proposes; NPC counters once (topUp = 0 < needed).
	d, err := fx.exchange.Propose(0, lineOf(mine, 1), lineOf(theirs, 1), 0)
	if err != nil || d.Outcome != domain.OutcomeCountered {
		t.Fatalf("Propose: %+v %v", d, err)
	}
	// Accepting the NPC's counter: should accept now.
	fx.fund(t, d.Offer.TopUp)
	accepted, err := fx.exchange.Accept(d.Offer.ID)
	if err != nil || accepted.Outcome != domain.OutcomeAccepted {
		t.Fatalf("Accept: %+v %v", accepted, err)
	}
}

// --- Criterion 7: unknown trait name ---

func TestNewBehaviourRejectsUnknownTraitName(t *testing.T) {
	t.Parallel()
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "known", Constructor: domaintest.NewMarginTrait()},
	)
	_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "unknown"}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err == nil {
		t.Fatal("unknown trait name must be rejected")
	}
	if !containsSubstring(err.Error(), "known") {
		t.Errorf("error %q does not list known trait names", err)
	}
}

// --- Helpers ---

// newExchangeFixtureWithRegistry creates an exchange fixture where the NPCs use the given trait registry.
func newExchangeFixtureWithRegistry(t *testing.T, size int, spec domain.ArchetypeSpec, params domain.ExchangeParams, reg domain.TraitRegistry, maxMarginPct domain.Percent) *exchangeFixture {
	t.Helper()
	sets := crowdSets()
	prices := domain.PriceSnapshot{}
	for i, cat := range sets {
		for id, price := range domaintest.NewPrices(cat, uint64(i+1)) {
			prices[id] = price
		}
	}
	valuer := domain.NewValuer(prices, exchangeFallbackPrice)
	rng := rand.New(rand.NewPCG(1, 1))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{
		Size:          size,
		Archetypes:    map[string]domain.ArchetypeSpec{"npc": spec},
		Valuer:        valuer,
		TraitRegistry: reg,
		MaxMarginPct:  maxMarginPct,
	}, sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	journal := domain.NewJournal()
	counter, err := domain.NewCounter(sets, prices, journal, domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: exchangeFallbackPrice})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	crowd, err := domain.NewCrowd(npcs, sets, opener, counter, rng, domain.WithSurplusHold(2))
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	player := domain.NewCollection()
	exchange, err := domain.NewExchange(withExchangeDefaults(params), crowd, player, journal, valuer)
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}

	fx := &exchangeFixture{exchange: exchange, crowd: crowd, npcs: npcs, player: player, journal: journal, prices: prices}
	for id, card := range sets[0].Cards {
		if !card.IsBasicLand {
			fx.keys = append(fx.keys, domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
		}
	}
	slices.SortFunc(fx.keys, func(a, b domain.CardKey) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return fx
}

// fixedMarginTrait is a minimal MarginAdder that always returns the same add.
type fixedMarginTrait struct{ add domain.Percent }

func (f *fixedMarginTrait) Name() string                            { return "fixed" }
func (f *fixedMarginTrait) MarginAdd(_ domain.Offer) domain.Percent { return f.add }

func composedIDs(wishes []domain.Wish) []domain.CardID {
	ids := make([]domain.CardID, len(wishes))
	for i, w := range wishes {
		ids[i] = w.ID
	}
	return ids
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// permutations returns all permutations of s.
func permutations(s []string) [][]string {
	if len(s) == 0 {
		return nil
	}
	if len(s) == 1 {
		return [][]string{{s[0]}}
	}
	var result [][]string
	for i, v := range s {
		rest := make([]string, 0, len(s)-1)
		rest = append(rest, s[:i]...)
		rest = append(rest, s[i+1:]...)
		for _, p := range permutations(rest) {
			result = append(result, append([]string{v}, p...))
		}
	}
	return result
}
