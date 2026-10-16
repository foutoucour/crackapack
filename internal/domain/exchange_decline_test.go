package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// --- Test 1: NPC starts with no decline streak ---

func TestNPCStartsWithNoDeclineStreak(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
	if got := fx.npcs[0].DeclineStreak(); got != 0 {
		t.Errorf("DeclineStreak = %d, want 0 on a new NPC", got)
	}
}

// --- Test 2: Decline of an NPC offer raises the streak ---

func TestDeclineNPCOfferRaisesStreak(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

	// First decline: streak goes to 1.
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offer1 := fx.exchange.Offers()[0]
	if err := fx.exchange.Decline(offer1.ID); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 1 {
		t.Errorf("after first decline: DeclineStreak = %d, want 1", got)
	}

	// Second decline: NPC re-proposes (player still holds the duplicate) and streak goes to 2.
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offer2 := fx.exchange.Offers()[0]
	if err := fx.exchange.Decline(offer2.ID); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 2 {
		t.Errorf("after second decline: DeclineStreak = %d, want 2", got)
	}
}

// --- Test 3: Decline of an NPC counter raises the streak ---

func TestDeclineNPCCounterRaisesStreak(t *testing.T) {
	t.Parallel()
	// NPC with margin=10: player proposes short, NPC counters with topUp, player declines counter.
	fx := newExchangeFixture(t, 1, tradingSpec(10), domain.ExchangeParams{})
	fx.setPrice(fx.keys[0], 100)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 1)
	fx.shelve(0, fx.keys[1], 1)
	fx.exchange.StartDay(1)

	d, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
	if err != nil || d.Outcome != domain.OutcomeCountered {
		t.Fatalf("Propose = %+v, %v, want countered", d, err)
	}
	if d.Offer.Origin != domain.OriginNPCCounter {
		t.Fatalf("counter offer origin = %v, want OriginNPCCounter", d.Offer.Origin)
	}

	if err := fx.exchange.Decline(d.Offer.ID); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 1 {
		t.Errorf("DeclineStreak = %d, want 1 after declining the NPC counter", got)
	}
}

// --- Test 4: Decline of an unknown offer returns ErrOfferNotFound and leaves the streak ---

func TestDeclineUnknownOfferReturnsErrAndLeavesStreak(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

	err := fx.exchange.Decline(999)
	if !errors.Is(err, domain.ErrOfferNotFound) {
		t.Errorf("Decline(unknown) = %v, want ErrOfferNotFound", err)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 0 {
		t.Errorf("DeclineStreak = %d, want 0: unknown offer must not affect the streak", got)
	}
}

// --- Test 5: A player counter the NPC refuses raises the streak ---

func TestCounterRefusedByNPCRaisesStreak(t *testing.T) {
	t.Parallel()
	t.Run("ErrBelowMargin", func(t *testing.T) {
		t.Parallel()
		// NPC proposes: player gives keys[0](500), NPC gives 3×keys[1](100). Player counters
		// with keys[2](200) for the same NPC cards: 200 received for 300 given at 0 margin is
		// refused with ErrBelowMargin.
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		replacement := fx.keys[2]
		fx.setPrice(replacement, 200)
		fx.give(t, replacement, 1)

		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		npcOffer := fx.exchange.Offers()[0]

		d, err := fx.exchange.Counter(npcOffer.ID, lineOf(replacement, 1), 0)
		if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrBelowMargin) {
			t.Fatalf("Counter = %+v, %v, want refused with ErrBelowMargin", d, err)
		}
		if got := fx.npcs[0].DeclineStreak(); got != 1 {
			t.Errorf("DeclineStreak = %d, want 1: a refused counter on terms counts as a decline", got)
		}
	})

	t.Run("ErrCounterVetoed", func(t *testing.T) {
		t.Parallel()
		fx := vetoFixture(t)

		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		if len(fx.exchange.Offers()) != 1 {
			t.Fatalf("expected 1 NPC offer, got %d", len(fx.exchange.Offers()))
		}
		npcOffer := fx.exchange.Offers()[0]

		replacement := fx.keys[2]
		fx.setPrice(replacement, 500)
		fx.give(t, replacement, 1)

		d, err := fx.exchange.Counter(npcOffer.ID, lineOf(replacement, 1), 0)
		if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrCounterVetoed) {
			t.Fatalf("Counter = %+v, %v, want refused with ErrCounterVetoed", d, err)
		}
		if got := fx.npcs[0].DeclineStreak(); got != 1 {
			t.Errorf("DeclineStreak = %d, want 1: a vetoed counter counts as a decline", got)
		}
	})
}

// --- Test 6: A counter refused for another reason (ErrPlayerLacksCard) leaves the streak ---

func TestCounterRefusedForPlayerErrorLeavesStreak(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	npcOffer := fx.exchange.Offers()[0]

	// keys[2] is not in the player's collection: the counter is refused for ErrPlayerLacksCard.
	d, err := fx.exchange.Counter(npcOffer.ID, lineOf(fx.keys[2], 1), 0)
	if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrPlayerLacksCard) {
		t.Fatalf("Counter = %+v, %v, want refused with ErrPlayerLacksCard", d, err)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 0 {
		t.Errorf("DeclineStreak = %d, want 0: a player-error refusal must not count", got)
	}
}

// --- Test 7: An accept resets the streak to 0, table-driven from a streak of 2 ---

// declineNPCOffers declines n consecutive NPC offers using ProposeFromNPCs to build up the streak.
func declineNPCOffers(t *testing.T, fx *exchangeFixture, n int) {
	t.Helper()
	for range n {
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		offers := fx.exchange.Offers()
		if len(offers) == 0 {
			t.Fatalf("no NPC offer to decline at iteration")
		}
		if err := fx.exchange.Decline(offers[0].ID); err != nil {
			t.Fatalf("Decline: %v", err)
		}
	}
}

func TestAcceptResetsStreakToZero(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		prepare func(t *testing.T) (npc *domain.NPC, doAccept func())
	}{
		{
			name: "Accept of an NPC offer",
			prepare: func(t *testing.T) (*domain.NPC, func()) {
				t.Helper()
				fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
				declineNPCOffers(t, fx, 2) // streak = 2
				if err := fx.exchange.ProposeFromNPCs(); err != nil {
					t.Fatalf("ProposeFromNPCs: %v", err)
				}
				offer := fx.exchange.Offers()[0]
				return fx.npcs[0], func() {
					d, err := fx.exchange.Accept(offer.ID)
					if err != nil || d.Outcome != domain.OutcomeAccepted {
						t.Fatalf("Accept = %+v, %v, want accepted", d, err)
					}
				}
			},
		},
		{
			name: "Accept of an NPC counter",
			prepare: func(t *testing.T) (*domain.NPC, func()) {
				t.Helper()
				// NPC with margin=10: player proposes short twice to build streak=2, then a third
				// time; the player funds the required top-up and accepts the counter.
				fx := newExchangeFixture(t, 1, tradingSpec(10), domain.ExchangeParams{})
				fx.setPrice(fx.keys[0], 100)
				fx.setPrice(fx.keys[1], 100)
				fx.give(t, fx.keys[0], 1)
				fx.shelve(0, fx.keys[1], 1)
				fx.exchange.StartDay(1)
				// Build streak to 2 by declining two NPC counters.
				for range 2 {
					d, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
					if err != nil || d.Outcome != domain.OutcomeCountered {
						t.Fatalf("Propose = %+v, %v, want countered", d, err)
					}
					if err := fx.exchange.Decline(d.Offer.ID); err != nil {
						t.Fatalf("Decline: %v", err)
					}
				}
				// Third proposal: fund the top-up and accept the counter.
				d, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
				if err != nil || d.Outcome != domain.OutcomeCountered {
					t.Fatalf("Propose for accept = %+v, %v, want countered", d, err)
				}
				fx.fund(t, d.Offer.TopUp)
				counterID := d.Offer.ID
				return fx.npcs[0], func() {
					accepted, err := fx.exchange.Accept(counterID)
					if err != nil || accepted.Outcome != domain.OutcomeAccepted {
						t.Fatalf("Accept counter = %+v, %v, want accepted", accepted, err)
					}
				}
			},
		},
		{
			name: "Counter accepted by the NPC",
			prepare: func(t *testing.T) (*domain.NPC, func()) {
				t.Helper()
				// NPC proposes (gives 3×keys[1]=300). Player counters with keys[2]=300: accepted.
				fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
				replacement := fx.keys[2]
				fx.setPrice(replacement, 300)
				fx.give(t, replacement, 1)
				declineNPCOffers(t, fx, 2) // streak = 2
				if err := fx.exchange.ProposeFromNPCs(); err != nil {
					t.Fatalf("ProposeFromNPCs: %v", err)
				}
				npcOffer := fx.exchange.Offers()[0]
				return fx.npcs[0], func() {
					d, err := fx.exchange.Counter(npcOffer.ID, lineOf(replacement, 1), 0)
					if err != nil || d.Outcome != domain.OutcomeAccepted {
						t.Fatalf("Counter = %+v, %v, want accepted", d, err)
					}
				}
			},
		},
		{
			name: "Propose accepted at once",
			prepare: func(t *testing.T) (*domain.NPC, func()) {
				t.Helper()
				// Player proposes giving keys[0](500) for 3×keys[1](300): player overpays at
				// zero margin, so the NPC accepts at once.
				fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
				declineNPCOffers(t, fx, 2) // streak = 2
				return fx.npcs[0], func() {
					d, err := fx.exchange.Propose(0,
						lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 3), 0)
					if err != nil || d.Outcome != domain.OutcomeAccepted {
						t.Fatalf("Propose = %+v, %v, want accepted at once", d, err)
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			npc, doAccept := tc.prepare(t)
			if got := npc.DeclineStreak(); got != 2 {
				t.Fatalf("pre-accept: DeclineStreak = %d, want 2", got)
			}
			doAccept()
			if got := npc.DeclineStreak(); got != 0 {
				t.Errorf("post-accept: DeclineStreak = %d, want 0", got)
			}
		})
	}
}

// --- Test 8: A failed Accept (ErrInsufficientBalance) leaves the streak ---

func TestFailedAcceptLeavesStreak(t *testing.T) {
	t.Parallel()
	// NPC with margin=10: player proposes short, NPC counters with topUp=10. Player declines
	// once (streak=1) then proposes short again; the second counter stays open and the player
	// tries to accept with no coins.
	fx := newExchangeFixture(t, 1, tradingSpec(10), domain.ExchangeParams{})
	fx.setPrice(fx.keys[0], 100)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 1)
	fx.shelve(0, fx.keys[1], 1)
	fx.exchange.StartDay(1)

	// Build streak=1.
	d, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
	if err != nil || d.Outcome != domain.OutcomeCountered {
		t.Fatalf("first Propose = %+v, %v, want countered", d, err)
	}
	if err := fx.exchange.Decline(d.Offer.ID); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 1 {
		t.Fatalf("after decline: DeclineStreak = %d, want 1", got)
	}

	// Second proposal: player cannot pay the top-up.
	d2, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
	if err != nil || d2.Outcome != domain.OutcomeCountered {
		t.Fatalf("second Propose = %+v, %v, want countered", d2, err)
	}
	_, err = fx.exchange.Accept(d2.Offer.ID)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("Accept with no coins = %v, want ErrInsufficientBalance", err)
	}

	if got := fx.npcs[0].DeclineStreak(); got != 1 {
		t.Errorf("after failed accept: DeclineStreak = %d, want 1: a failed accept must not change the streak", got)
	}
}

// --- Test 9: Expiry leaves the streak unchanged ---

func TestExpiryLeavesStreak(t *testing.T) {
	t.Parallel()
	// ExpiryDays=3: offers on day 1 expire on StartDay(4).
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{ExpiryDays: 3})

	// Decline once to set streak=1.
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offer := fx.exchange.Offers()[0]
	if err := fx.exchange.Decline(offer.ID); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 1 {
		t.Fatalf("after decline: DeclineStreak = %d, want 1", got)
	}

	// Get a new offer on day 1 that will expire.
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	if got := len(fx.exchange.Offers()); got != 1 {
		t.Fatalf("after second ProposeFromNPCs: %d open offers, want 1", got)
	}

	// Advance past the expiry day.
	fx.exchange.TakeDayStats() // reset before observing
	fx.exchange.StartDay(4)

	stats := fx.exchange.TakeDayStats()
	if got := len(fx.exchange.Offers()); got != 0 {
		t.Errorf("after expiry: %d open offers, want 0", got)
	}
	if stats.Expired != 1 {
		t.Errorf("Expired = %d, want 1", stats.Expired)
	}
	if got := fx.npcs[0].DeclineStreak(); got != 1 {
		t.Errorf("after expiry: DeclineStreak = %d, want 1: expiry must not change the streak", got)
	}
}

// --- Test 10: The reactor is notified with DeclineSpy ---

func TestDeclineReactorIsNotifiedWithContext(t *testing.T) {
	t.Parallel()
	// spySpec is the archetype spec shared by sub-tests that create their own spy+registry.
	spySpec := domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "casual",
		OfferChancePct: 100,
		Traits:         []domain.TraitSpec{{Name: "spy"}},
	}

	t.Run("Decline of an NPC offer", func(t *testing.T) {
		t.Parallel()
		// Each sub-test uses its own spy to stay independent.
		spyLocal := &domaintest.DeclineSpy{}
		regLocal := domaintest.TraitRegistry(
			domaintest.TraitEntry{Name: "spy", Constructor: domaintest.NewDeclineSpy(spyLocal)},
		)
		specLocal := domain.ArchetypeSpec{
			Share:          1,
			KeepCopies:     1,
			Behaviour:      "casual",
			OfferChancePct: 100,
			Traits:         []domain.TraitSpec{{Name: "spy"}},
		}
		fxLocal := newExchangeFixtureWithRegistry(t, 1, specLocal, domain.ExchangeParams{}, regLocal, 1000)
		fxLocal.setPrice(fxLocal.keys[0], 500)
		fxLocal.setPrice(fxLocal.keys[1], 100)
		fxLocal.give(t, fxLocal.keys[0], 2)
		fxLocal.crowd.SetWishesForTest(0, []domain.Wish{{ID: fxLocal.keys[0].ID, Want: 1}})
		fxLocal.shelve(0, fxLocal.keys[1], 3)
		fxLocal.exchange.StartDay(1)

		if err := fxLocal.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		npcOffer := fxLocal.exchange.Offers()[0]
		if err := fxLocal.exchange.Decline(npcOffer.ID); err != nil {
			t.Fatalf("Decline: %v", err)
		}

		if len(spyLocal.Calls) != 1 {
			t.Fatalf("spy.Calls len = %d, want 1", len(spyLocal.Calls))
		}
		call := spyLocal.Calls[0]
		if call.Context.NPC == nil || call.Context.NPC.ID != 0 {
			t.Errorf("call.Context.NPC.ID = %v, want 0", call.Context.NPC)
		}
		if call.Context.Offer.ID != npcOffer.ID {
			t.Errorf("call.Context.Offer.ID = %d, want %d", call.Context.Offer.ID, npcOffer.ID)
		}
		if call.Context.Offer.Origin != domain.OriginNPC {
			t.Errorf("call.Context.Offer.Origin = %v, want OriginNPC", call.Context.Offer.Origin)
		}
		if call.Streak != 1 {
			t.Errorf("call.Streak = %d, want 1", call.Streak)
		}
	})

	t.Run("Decline of an NPC counter", func(t *testing.T) {
		t.Parallel()
		spyLocal := &domaintest.DeclineSpy{}
		regLocal := domaintest.TraitRegistry(
			domaintest.TraitEntry{Name: "spy", Constructor: domaintest.NewDeclineSpy(spyLocal)},
		)
		specLocal := domain.ArchetypeSpec{
			Share:      1,
			KeepCopies: 1,
			Behaviour:  "casual",
			MarginPct:  10,
			Traits:     []domain.TraitSpec{{Name: "spy"}},
		}
		fxLocal := newExchangeFixtureWithRegistry(t, 1, specLocal, domain.ExchangeParams{}, regLocal, 1000)
		fxLocal.setPrice(fxLocal.keys[0], 100)
		fxLocal.setPrice(fxLocal.keys[1], 100)
		fxLocal.give(t, fxLocal.keys[0], 1)
		fxLocal.shelve(0, fxLocal.keys[1], 1)
		fxLocal.exchange.StartDay(1)

		d, err := fxLocal.exchange.Propose(0, lineOf(fxLocal.keys[0], 1), lineOf(fxLocal.keys[1], 1), 0)
		if err != nil || d.Outcome != domain.OutcomeCountered {
			t.Fatalf("Propose = %+v, %v, want countered", d, err)
		}
		if err := fxLocal.exchange.Decline(d.Offer.ID); err != nil {
			t.Fatalf("Decline: %v", err)
		}

		if len(spyLocal.Calls) != 1 {
			t.Fatalf("spy.Calls len = %d, want 1", len(spyLocal.Calls))
		}
		if spyLocal.Calls[0].Context.Offer.Origin != domain.OriginNPCCounter {
			t.Errorf("call.Offer.Origin = %v, want OriginNPCCounter", spyLocal.Calls[0].Context.Offer.Origin)
		}
	})

	t.Run("refused counter gives the original NPC offer", func(t *testing.T) {
		t.Parallel()
		spyLocal := &domaintest.DeclineSpy{}
		regLocal := domaintest.TraitRegistry(
			domaintest.TraitEntry{Name: "spy", Constructor: domaintest.NewDeclineSpy(spyLocal)},
		)
		specLocal := domain.ArchetypeSpec{
			Share:          1,
			KeepCopies:     1,
			Behaviour:      "casual",
			OfferChancePct: 100,
			Traits:         []domain.TraitSpec{{Name: "spy"}},
		}
		fxLocal := newExchangeFixtureWithRegistry(t, 1, specLocal, domain.ExchangeParams{}, regLocal, 1000)
		fxLocal.setPrice(fxLocal.keys[0], 500)
		fxLocal.setPrice(fxLocal.keys[1], 100)
		fxLocal.give(t, fxLocal.keys[0], 2)
		fxLocal.crowd.SetWishesForTest(0, []domain.Wish{{ID: fxLocal.keys[0].ID, Want: 1}})
		fxLocal.shelve(0, fxLocal.keys[1], 3)
		// keys[2] is used as replacement at price 200 < NPC gives (3×100=300): ErrBelowMargin.
		replacement := fxLocal.keys[2]
		fxLocal.setPrice(replacement, 200)
		fxLocal.give(t, replacement, 1)
		fxLocal.exchange.StartDay(1)

		if err := fxLocal.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		originalOffer := fxLocal.exchange.Offers()[0]

		d, err := fxLocal.exchange.Counter(originalOffer.ID, lineOf(replacement, 1), 0)
		if err != nil || !errors.Is(d.Reason, domain.ErrBelowMargin) {
			t.Fatalf("Counter = %+v, %v, want ErrBelowMargin", d, err)
		}

		if len(spyLocal.Calls) != 1 {
			t.Fatalf("spy.Calls len = %d, want 1", len(spyLocal.Calls))
		}
		call := spyLocal.Calls[0]
		// The context carry the original NPC offer (OriginNPC), not the counter.
		if call.Context.Offer.ID != originalOffer.ID {
			t.Errorf("call.Offer.ID = %d, want original offer ID %d", call.Context.Offer.ID, originalOffer.ID)
		}
		if call.Context.Offer.Origin != domain.OriginNPC {
			t.Errorf("call.Offer.Origin = %v, want OriginNPC (original offer)", call.Context.Offer.Origin)
		}
	})

	t.Run("accept notifies nobody", func(t *testing.T) {
		t.Parallel()
		spyAccept := &domaintest.DeclineSpy{}
		regAccept := domaintest.TraitRegistry(
			domaintest.TraitEntry{Name: "spy", Constructor: domaintest.NewDeclineSpy(spyAccept)},
		)
		fxAccept := newExchangeFixtureWithRegistry(t, 1, spySpec, domain.ExchangeParams{}, regAccept, 1000)
		fxAccept.setPrice(fxAccept.keys[0], 500)
		fxAccept.setPrice(fxAccept.keys[1], 100)
		fxAccept.give(t, fxAccept.keys[0], 2)
		fxAccept.crowd.SetWishesForTest(0, []domain.Wish{{ID: fxAccept.keys[0].ID, Want: 1}})
		fxAccept.shelve(0, fxAccept.keys[1], 3)
		fxAccept.exchange.StartDay(1)

		if err := fxAccept.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		o := fxAccept.exchange.Offers()[0]
		if _, err := fxAccept.exchange.Accept(o.ID); err != nil {
			t.Fatalf("Accept: %v", err)
		}
		if len(spyAccept.Calls) != 0 {
			t.Errorf("spy.Calls len = %d, want 0: accept must not notify the decline reactor", len(spyAccept.Calls))
		}
	})

	t.Run("expiry notifies nobody", func(t *testing.T) {
		t.Parallel()
		spyExpiry := &domaintest.DeclineSpy{}
		regExpiry := domaintest.TraitRegistry(
			domaintest.TraitEntry{Name: "spy", Constructor: domaintest.NewDeclineSpy(spyExpiry)},
		)
		fxExpiry := newExchangeFixtureWithRegistry(t, 1, spySpec, domain.ExchangeParams{ExpiryDays: 3}, regExpiry, 1000)
		fxExpiry.setPrice(fxExpiry.keys[0], 500)
		fxExpiry.setPrice(fxExpiry.keys[1], 100)
		fxExpiry.give(t, fxExpiry.keys[0], 2)
		fxExpiry.crowd.SetWishesForTest(0, []domain.Wish{{ID: fxExpiry.keys[0].ID, Want: 1}})
		fxExpiry.shelve(0, fxExpiry.keys[1], 3)
		fxExpiry.exchange.StartDay(1)

		if err := fxExpiry.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		fxExpiry.exchange.StartDay(4) // expires the offer

		if len(spyExpiry.Calls) != 0 {
			t.Errorf("spy.Calls len = %d, want 0: expiry must not notify the decline reactor", len(spyExpiry.Calls))
		}
	})

	t.Run("counter refused for player error notifies nobody", func(t *testing.T) {
		t.Parallel()
		spyPlayerErr := &domaintest.DeclineSpy{}
		regPlayerErr := domaintest.TraitRegistry(
			domaintest.TraitEntry{Name: "spy", Constructor: domaintest.NewDeclineSpy(spyPlayerErr)},
		)
		fxPlayerErr := newExchangeFixtureWithRegistry(t, 1, spySpec, domain.ExchangeParams{}, regPlayerErr, 1000)
		fxPlayerErr.setPrice(fxPlayerErr.keys[0], 500)
		fxPlayerErr.setPrice(fxPlayerErr.keys[1], 100)
		fxPlayerErr.give(t, fxPlayerErr.keys[0], 2)
		fxPlayerErr.crowd.SetWishesForTest(0, []domain.Wish{{ID: fxPlayerErr.keys[0].ID, Want: 1}})
		fxPlayerErr.shelve(0, fxPlayerErr.keys[1], 3)
		fxPlayerErr.exchange.StartDay(1)

		if err := fxPlayerErr.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		npcOffer := fxPlayerErr.exchange.Offers()[0]

		// Player does not hold keys[2]: ErrPlayerLacksCard.
		d, err := fxPlayerErr.exchange.Counter(npcOffer.ID, lineOf(fxPlayerErr.keys[2], 1), 0)
		if err != nil || !errors.Is(d.Reason, domain.ErrPlayerLacksCard) {
			t.Fatalf("Counter = %+v, %v, want ErrPlayerLacksCard", d, err)
		}
		if len(spyPlayerErr.Calls) != 0 {
			t.Errorf("spy.Calls len = %d, want 0: a player-error refusal must not notify", len(spyPlayerErr.Calls))
		}
	})
}

// --- Test 11: Per-NPC streaks with a shared spy ---

func TestPerNPCStreaksWithSharedSpy(t *testing.T) {
	t.Parallel()
	// Two NPCs of one archetype share one behaviour and therefore one spy.
	spy := &domaintest.DeclineSpy{}
	reg := domaintest.TraitRegistry(
		domaintest.TraitEntry{Name: "spy", Constructor: domaintest.NewDeclineSpy(spy)},
	)
	spec := domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "casual",
		OfferChancePct: 100,
		WishSize:       5,
		Traits:         []domain.TraitSpec{{Name: "spy"}},
	}
	fx := newExchangeFixtureWithRegistry(t, 2, spec, domain.ExchangeParams{}, reg, 1000)
	fx.setPrice(fx.keys[0], 500)
	fx.setPrice(fx.keys[1], 100)
	// Three copies → two duplicates so both NPCs can propose simultaneously.
	fx.give(t, fx.keys[0], 3)
	for id := range 2 {
		fx.crowd.SetWishesForTest(id, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
		fx.shelve(id, fx.keys[1], 3)
	}
	fx.exchange.StartDay(1)

	// Both NPCs propose: collect each NPC's offer by NPC ID.
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offers := fx.exchange.Offers()
	if len(offers) < 2 {
		t.Fatalf("expected 2 NPC offers, got %d", len(offers))
	}
	var npc0Offer, npc1Offer domain.Offer
	for _, o := range offers {
		switch o.NPC {
		case 0:
			npc0Offer = o
		case 1:
			npc1Offer = o
		}
	}
	if npc0Offer.ID == 0 || npc1Offer.ID == 0 {
		t.Fatalf("did not get one offer per NPC: %v", offers)
	}

	// NPC 0 declines once → spy.Calls[0].Streak = 1.
	if err := fx.exchange.Decline(npc0Offer.ID); err != nil {
		t.Fatalf("Decline NPC 0: %v", err)
	}

	// NPC 0 proposes a second time (player still has 3 copies; NPC 1's offer reserves one
	// duplicate, leaving one for NPC 0 to re-propose).
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs (second): %v", err)
	}
	var npc0Offer2 domain.Offer
	for _, o := range fx.exchange.Offers() {
		if o.NPC == 0 {
			npc0Offer2 = o
		}
	}
	if npc0Offer2.ID == 0 {
		t.Fatalf("NPC 0 did not re-propose; open offers: %v", fx.exchange.Offers())
	}

	// NPC 0 declines a second time → spy.Calls[1].Streak = 2.
	if err := fx.exchange.Decline(npc0Offer2.ID); err != nil {
		t.Fatalf("Decline NPC 0 (second): %v", err)
	}

	// NPC 1 declines once → spy.Calls[2].Streak = 1.
	if err := fx.exchange.Decline(npc1Offer.ID); err != nil {
		t.Fatalf("Decline NPC 1: %v", err)
	}

	// Verify the shared spy recorded the correct streaks in order.
	if len(spy.Calls) != 3 {
		t.Fatalf("spy.Calls len = %d, want 3; calls = %+v", len(spy.Calls), spy.Calls)
	}
	wantStreaks := []int{1, 2, 1}
	for i, call := range spy.Calls {
		if call.Streak != wantStreaks[i] {
			t.Errorf("spy.Calls[%d].Streak = %d, want %d", i, call.Streak, wantStreaks[i])
		}
	}

	// Per-NPC streak state is independent: NPC 0 has streak 2, NPC 1 has streak 1.
	if got := fx.npcs[0].DeclineStreak(); got != 2 {
		t.Errorf("NPC 0 DeclineStreak = %d, want 2", got)
	}
	if got := fx.npcs[1].DeclineStreak(); got != 1 {
		t.Errorf("NPC 1 DeclineStreak = %d, want 1", got)
	}
}

// --- Test 12: Streak counts even without a decline-reacting trait ---

func TestStreakCountsWithoutDeclineReactor(t *testing.T) {
	t.Parallel()
	t.Run("margin trait only, no decline reactor", func(t *testing.T) {
		t.Parallel()
		reg := domaintest.TraitRegistry(
			domaintest.TraitEntry{Name: "margin", Constructor: domaintest.NewMarginTrait()},
		)
		spec := domain.ArchetypeSpec{
			Share:          1,
			KeepCopies:     1,
			Behaviour:      "casual",
			OfferChancePct: 100,
			Traits:         []domain.TraitSpec{{Name: "margin", Params: map[string]int{"add": 5}}},
		}
		fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{}, reg, 1000)
		fx.setPrice(fx.keys[0], 500)
		fx.setPrice(fx.keys[1], 100)
		fx.give(t, fx.keys[0], 2)
		fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
		fx.shelve(0, fx.keys[1], 3)
		fx.exchange.StartDay(1)

		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		o := fx.exchange.Offers()[0]
		if err := fx.exchange.Decline(o.ID); err != nil {
			t.Fatalf("Decline: %v", err)
		}
		if got := fx.npcs[0].DeclineStreak(); got != 1 {
			t.Errorf("DeclineStreak = %d, want 1 (margin-only trait, no reactor)", got)
		}
	})

	t.Run("base behaviour only, no traits", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		o := fx.exchange.Offers()[0]
		if err := fx.exchange.Decline(o.ID); err != nil {
			t.Fatalf("Decline: %v", err)
		}
		if got := fx.npcs[0].DeclineStreak(); got != 1 {
			t.Errorf("DeclineStreak = %d, want 1 (base behaviour only, no traits)", got)
		}
	})
}
