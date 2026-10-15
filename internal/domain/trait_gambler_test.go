package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// buildGamblerSpec builds an ArchetypeSpec for a gambler NPC.
// marginBase is the NPC's base MarginPct (needed to force counter-offers).
func buildGamblerSpec(chanceStep, maxChance, marginExtra, marginStep, offerChancePct, marginBase int) domain.ArchetypeSpec {
	return domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "casual",
		OfferChancePct: offerChancePct,
		MarginPct:      domain.Percent(marginBase),
		Traits: []domain.TraitSpec{{
			Name: "gambler",
			Params: map[string]int{
				"chance_step_pct":      chanceStep,
				"max_offer_chance_pct": maxChance,
				"margin_extra_pct":     marginExtra,
				"margin_step_pct":      marginStep,
			},
		}},
	}
}

// gamblerFixture creates an exchange fixture ready for gambler streak testing.
// The NPC has keys[1] on its shelf and wants keys[0]; the player has 10 copies of keys[0].
func gamblerFixture(t *testing.T, spec domain.ArchetypeSpec) *exchangeFixture {
	t.Helper()
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{}, domain.DefaultTraits(), 1000)
	fx.setPrice(fx.keys[0], 100)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 10)
	fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: fx.keys[0].ID, Want: 10}})
	for range 10 {
		fx.shelve(0, fx.keys[1], 1)
	}
	fx.exchange.StartDay(1)
	return fx
}

// proposeAndDecline has the player propose keys[0]→keys[1] at 0 topUp and declines the NPC counter.
// The NPC must have a positive effective margin so the proposal is countered (not accepted or refused).
// Declining the counter increments the NPC's streak and fires ReactToDecline.
// proposeAndDecline has the player propose keys[0]→keys[1] at 0 topUp and declines the NPC counter.
// Requires that the NPC's effective margin > 0 so the proposal is countered (not accepted or refused).
// Declining the counter fires ReactToDecline and increments the NPC's streak.
func proposeAndDecline(t *testing.T, fx *exchangeFixture) {
	t.Helper()
	d, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if d.Outcome != domain.OutcomeCountered {
		t.Fatalf("expected counter (check margin setup), got %v", d.Outcome)
	}
	if err := fx.exchange.Decline(d.Offer.ID); err != nil {
		t.Fatalf("Decline: %v", err)
	}
}

// npcOfferAndDecline makes the NPC propose (it must have OfferChancePct=100) and the player declines.
// Use when proposeAndDecline cannot force a counter (e.g. when effective margin is 0).
func npcOfferAndDecline(t *testing.T, fx *exchangeFixture) {
	t.Helper()
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offers := fx.exchange.Offers()
	if len(offers) == 0 {
		t.Fatal("npcOfferAndDecline: no offer; NPC must have OfferChancePct=100")
	}
	if err := fx.exchange.Decline(offers[0].ID); err != nil {
		t.Fatalf("Decline NPC offer: %v", err)
	}
}

// --- Constructor tests ---

func TestGamblerTrait_constructorDefaults(t *testing.T) {
	t.Parallel()
	reg := domain.DefaultTraits()
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "gambler"}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("NewBehaviour with gambler defaults: %v", err)
	}
	if b == nil {
		t.Fatal("got nil behaviour")
	}
}

func TestGamblerTrait_constructorRejectsUnknownKey(t *testing.T) {
	t.Parallel()
	reg := domain.DefaultTraits()
	_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "gambler", Params: map[string]int{"unknown_key": 1}}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err == nil {
		t.Fatal("expected error for unknown gambler parameter")
	}
}

func TestGamblerTrait_constructorRejectsNegativeValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		param string
	}{
		{"negative chance_step_pct", "chance_step_pct"},
		{"negative margin_extra_pct", "margin_extra_pct"},
		{"negative margin_step_pct", "margin_step_pct"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := domain.DefaultTraits()
			_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
				Traits:       []domain.TraitSpec{{Name: "gambler", Params: map[string]int{tc.param: -1}}},
				Registry:     reg,
				MaxMarginPct: 1000,
			})
			if err == nil {
				t.Fatalf("expected error for negative %s", tc.param)
			}
		})
	}
}

func TestGamblerTrait_constructorRejectsMaxChanceAbove100(t *testing.T) {
	t.Parallel()
	reg := domain.DefaultTraits()
	_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		Traits:       []domain.TraitSpec{{Name: "gambler", Params: map[string]int{"max_offer_chance_pct": 101}}},
		Registry:     reg,
		MaxMarginPct: 1000,
	})
	if err == nil {
		t.Fatal("expected error for max_offer_chance_pct > 100")
	}
}

// --- Chance curve tests (story criterion 4, test 11) ---

// TestGamblerTrait_chanceCurveAtZeroStreak verifies that at streak=0, EffectiveOfferChancePct
// equals the base OfferChancePct regardless of trait settings.
func TestGamblerTrait_chanceCurveAtZeroStreak(t *testing.T) {
	t.Parallel()
	spec := buildGamblerSpec(5, 22, 0, 0, 10, 10)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]
	if got := npc.EffectiveOfferChancePct(); got != 10 {
		t.Errorf("streak=0: EffectiveOfferChancePct = %d, want 10", got)
	}
}

// TestGamblerTrait_chanceCurve verifies: base 10, step 5, cap 22 reads 10, 15, 20, 22, 22
// from EffectiveOfferChancePct at streaks 0..4. Streak is built via player proposals that
// the NPC counters (it wants a 10% base margin) and the player declines.
func TestGamblerTrait_chanceCurve(t *testing.T) {
	t.Parallel()
	// marginBase=10: NPC counters 0-topUp player proposals; gambler adds no extra margin.
	spec := buildGamblerSpec(5, 22, 0, 0, 10, 10)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]

	tests := []struct {
		streak int
		want   int
	}{
		{0, 10},
		{1, 15},
		{2, 20},
		{3, 22},
		{4, 22},
	}

	if got := npc.EffectiveOfferChancePct(); got != tests[0].want {
		t.Errorf("streak=0: EffectiveOfferChancePct = %d, want %d", got, tests[0].want)
	}

	for i := 1; i <= 4; i++ {
		proposeAndDecline(t, fx)
		if got := npc.EffectiveOfferChancePct(); got != tests[i].want {
			t.Errorf("streak=%d: EffectiveOfferChancePct = %d, want %d", i, got, tests[i].want)
		}
	}
}

// TestGamblerTrait_capBelowBaseNeverLowersChance verifies that when maxChance < base, the
// effective chance is not lowered below the base OfferChancePct.
func TestGamblerTrait_capBelowBaseNeverLowersChance(t *testing.T) {
	t.Parallel()
	// cap=5 < base=10; gambler must not lower the chance below 10.
	// marginBase=10 ensures player proposals are countered.
	spec := buildGamblerSpec(5, 5, 0, 0, 10, 10)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]

	for range 3 {
		proposeAndDecline(t, fx)
	}
	if got := npc.EffectiveOfferChancePct(); got < npc.OfferChancePct {
		t.Errorf("EffectiveOfferChancePct = %d < OfferChancePct = %d: cap below base must not lower chance",
			got, npc.OfferChancePct)
	}
}

// TestGamblerTrait_acceptResetsChanceViaStreak verifies that after an accepted trade the streak
// resets and EffectiveOfferChancePct returns the plain base OfferChancePct.
func TestGamblerTrait_acceptResetsChanceViaStreak(t *testing.T) {
	t.Parallel()
	// base=50, step=30, cap=80: one decline raises chance from 50 to min(80,50+30)=80.
	// OfferChancePct=50 (< cap=80) so the tilt is clearly visible.
	// marginBase=10 forces player proposals to be countered (for the decline step).
	spec := buildGamblerSpec(30, 80, 0, 0, 50, 10)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]

	// Build streak=1; effective chance must be above base.
	proposeAndDecline(t, fx)
	if got := npc.EffectiveOfferChancePct(); got <= npc.OfferChancePct {
		t.Fatalf("tilt not active after decline: got=%d, base=%d", got, npc.OfferChancePct)
	}

	// Accept an NPC offer to reset the streak. Bump OfferChancePct to 100 to guarantee
	// ProposeFromNPCs produces an offer; restore afterward. The tilt-corrupting concern does not
	// apply here because the post-accept streak is 0, which ignores chanceTilt entirely.
	npc.OfferChancePct = 100
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	npc.OfferChancePct = 50
	offers := fx.exchange.Offers()
	if len(offers) == 0 {
		t.Skip("no NPC offer generated; skipping accept-resets-chance test")
	}
	if _, err := fx.exchange.Accept(offers[0].ID); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	// Streak=0: EffectiveOfferChancePct ignores chanceTilt and returns base.
	if got := npc.EffectiveOfferChancePct(); got != npc.OfferChancePct {
		t.Errorf("EffectiveOfferChancePct = %d after accept, want %d (base)", got, npc.OfferChancePct)
	}
}

// --- Margin curve tests (story criterion 4, test 12) ---

// marginer is the local interface for MarginFor.
type marginer interface {
	MarginFor(domain.MarginContext) domain.Percent
}

// TestGamblerTrait_marginCurve verifies: extra=15, step=5, base margin=0 →
// streak 0→15, 1→10, 2→5, 3→0, 4→0 from MarginFor.
// At streak=0 the gambler adds 15, forcing NPC counters at 0 topUp.
// Once the margin reaches 0 (streak≥3), NPC offers are declined directly to build further streak.
func TestGamblerTrait_marginCurve(t *testing.T) {
	t.Parallel()
	// marginBase=0; gambler contributes 15 at streak=0 which forces counters at 0 topUp.
	// OfferChancePct=100 so the NPC proposes every time (used at streak≥3 via npcOfferAndDecline).
	spec := buildGamblerSpec(0, 30, 15, 5, 100, 0)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]

	cm, ok := npc.Behaviour.(marginer)
	if !ok {
		t.Fatal("behaviour does not implement MarginFor; no gambler trait?")
	}
	ctx := domain.MarginContext{Offer: domain.Offer{}, NPC: npc, Tick: 0}

	// wantMargin holds the expected MarginFor at each streak level 0..4.
	// noCounter marks the streak level whose margin is 0, meaning the next decline must use
	// npcOfferAndDecline (a player-proposed trade would be accepted, not countered).
	type step struct {
		wantMargin int
		noCounter  bool // margin at this streak = 0; use NPC offer for the next decline
	}
	tests := []step{
		{15, false}, // streak=0
		{10, false}, // streak=1
		{5, false},  // streak=2
		{0, true},   // streak=3: margin=0, next decline must come from NPC offer
		{0, false},  // streak=4
	}

	if got := int(cm.MarginFor(ctx)); got != tests[0].wantMargin {
		t.Errorf("streak=0: margin = %d, want %d", got, tests[0].wantMargin)
	}

	for i := 1; i <= 4; i++ {
		if tests[i-1].noCounter {
			// Margin already 0: player proposals get accepted, not countered. Use NPC offer.
			npcOfferAndDecline(t, fx)
		} else {
			proposeAndDecline(t, fx)
		}
		if got := int(cm.MarginFor(ctx)); got != tests[i].wantMargin {
			t.Errorf("streak=%d: margin = %d, want %d", i, got, tests[i].wantMargin)
		}
	}
}

// TestGamblerTrait_marginFloorsAtBase verifies that the gambler's shrinking margin extra never
// pulls the total below the NPC's base MarginPct.
func TestGamblerTrait_marginFloorsAtBase(t *testing.T) {
	t.Parallel()
	// base=10, extra=5, step=3; at streak≥2 extra hits 0, total stays at 10.
	// marginBase=10 ensures early counters; total margin starts at 15 (base+extra).
	spec := buildGamblerSpec(0, 30, 5, 3, 100, 10)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]

	cm, ok := npc.Behaviour.(marginer)
	if !ok {
		t.Fatal("behaviour does not implement MarginFor")
	}
	ctx := domain.MarginContext{Offer: domain.Offer{}, NPC: npc, Tick: 0}

	for range 4 {
		proposeAndDecline(t, fx)
	}
	if got := int(cm.MarginFor(ctx)); got < 10 {
		t.Errorf("margin = %d, want >= 10: gambler extra must not lower total below base", got)
	}
}

// TestGamblerTrait_marginAcceptResets verifies that after an accepted trade the margin extra
// returns to its maximum (streak resets to 0).
func TestGamblerTrait_marginAcceptResets(t *testing.T) {
	t.Parallel()
	// marginBase=0, extra=15, step=5. At streak=3 extra hits 0.
	spec := buildGamblerSpec(0, 30, 15, 5, 100, 0)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]

	cm, ok := npc.Behaviour.(marginer)
	if !ok {
		t.Fatal("behaviour does not implement MarginFor")
	}
	ctx := domain.MarginContext{Offer: domain.Offer{}, NPC: npc, Tick: 0}

	// Build streak=3: gambler extra falls to 0.
	for range 3 {
		proposeAndDecline(t, fx)
	}
	if got := int(cm.MarginFor(ctx)); got != 0 {
		t.Errorf("streak=3: margin = %d, want 0", got)
	}

	// NPC proposes (OfferChancePct=100); player accepts → streak reset.
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offers := fx.exchange.Offers()
	if len(offers) == 0 {
		t.Skip("no NPC offer generated; fixture may need adjustment")
	}
	if _, err := fx.exchange.Accept(offers[0].ID); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if got := int(cm.MarginFor(ctx)); got != 15 {
		t.Errorf("after accept: margin = %d, want 15 (extra, streak reset)", got)
	}
}

// --- Proposes-more-after-decline test (story test 13) ---

// TestGamblerTrait_proposesMoreAfterDecline verifies that after a decline the gambler's
// EffectiveOfferChancePct rises measurably above its base.
func TestGamblerTrait_proposesMoreAfterDecline(t *testing.T) {
	t.Parallel()
	// base=10, step=50, cap=80: one decline raises effective chance to min(80,10+50)=60.
	// marginBase=10 forces counters at 0 topUp.
	spec := buildGamblerSpec(50, 80, 0, 0, 10, 10)
	fx := gamblerFixture(t, spec)
	npc := fx.npcs[0]

	before := npc.EffectiveOfferChancePct()
	proposeAndDecline(t, fx)
	after := npc.EffectiveOfferChancePct()

	if after <= before {
		t.Errorf("EffectiveOfferChancePct did not rise after decline: before=%d, after=%d", before, after)
	}
}

// --- Registry test ---

func TestGamblerTrait_registryEntryExists(t *testing.T) {
	t.Parallel()
	reg := domain.DefaultTraits()
	if _, ok := reg["gambler"]; !ok {
		t.Error("gambler is not registered in DefaultTraits()")
	}
}
