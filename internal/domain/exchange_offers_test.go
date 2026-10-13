package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// offerFixture is a fixture where NPC 0 wishes for keys[0] (worth 500), holds 3 copies of
// keys[1] (worth 100 each) on its shelf, and always proposes. The player holds two copies of
// keys[0], so one is a duplicate.
func offerFixture(t *testing.T, size int, margin domain.Percent, params domain.ExchangeParams) *exchangeFixture {
	t.Helper()
	spec := tradingSpec(margin)
	spec.OfferChancePct = 100
	fx := newExchangeFixture(t, size, spec, params)
	fx.setPrice(fx.keys[0], 500)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 2)
	for id := range size {
		fx.crowd.SetWishesForTest(id, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
		fx.shelve(id, fx.keys[1], 3)
	}
	fx.exchange.StartDay(1)
	return fx
}

func TestProposeFromNPCsMakesAnOfferForAMatchingDuplicate(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}

	offers := fx.exchange.Offers()
	if len(offers) != 1 {
		t.Fatalf("got %d offers, want 1", len(offers))
	}
	o := offers[0]
	if o.Origin != domain.OriginNPC || o.NPC != 0 || o.TopUp != 0 || o.CreatedDay != 1 || o.LastDay != 3 {
		t.Errorf("offer = %+v, want an NPC offer from NPC 0 with no top-up, made on day 1, open through day 3", o)
	}
	if want := lineOf(fx.keys[0], 1); !slices.Equal(o.PlayerGives, want) {
		t.Errorf("PlayerGives = %v, want %v: only the duplicate", o.PlayerGives, want)
	}
	if want := lineOf(fx.keys[1], 3); !slices.Equal(o.NPCGives, want) {
		t.Errorf("NPCGives = %v, want %v: three 100-cent cards for one 500-cent card", o.NPCGives, want)
	}
	if got := fx.exchange.TakeDayStats().NPCOffers; got != 1 {
		t.Errorf("NPCOffers = %d, want 1", got)
	}
}

func TestAcceptingAnNPCOfferTradesAndRefillsItsWishList(t *testing.T) {
	t.Parallel()
	spec := tradingSpec(0)
	spec.OfferChancePct = 100
	spec.WishSize = 3
	fx := newExchangeFixture(t, 1, spec, domain.ExchangeParams{})
	fx.setPrice(fx.keys[0], 500)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 2)
	fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
	fx.shelve(0, fx.keys[1], 3)
	fx.exchange.StartDay(1)
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offer := fx.exchange.Offers()[0]

	d, err := fx.exchange.Accept(offer.ID)

	if err != nil || d.Outcome != domain.OutcomeAccepted {
		t.Fatalf("Accept = %+v, %v, want accepted", d, err)
	}
	if got := fx.player.Quantity(fx.keys[1]); got != 3 {
		t.Errorf("player holds %d of the NPC's cards, want 3", got)
	}
	if got := fx.player.Quantity(fx.keys[0]); got != 1 {
		t.Errorf("player holds %d of the wished card, want 1 left", got)
	}
	if got := fx.npcs[0].Collection().Quantity(fx.keys[0]); got != 1 {
		t.Errorf("the NPC holds %d of its wished card, want 1", got)
	}
	wishes := fx.npcs[0].Wishes()
	if len(wishes) != 3 {
		t.Errorf("the NPC has %d wishes, want its list refilled to 3", len(wishes))
	}
	for _, w := range wishes {
		if w.ID == fx.keys[0].ID {
			t.Errorf("the NPC still wishes %q, which it now holds", w.ID)
		}
	}
	if len(fx.exchange.Offers()) != 0 {
		t.Error("the accepted offer is still open")
	}
}

func TestProposeFromNPCsMakesOneOpenOfferPerNPC(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

	for range 3 {
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
	}

	if got := len(fx.exchange.Offers()); got != 1 {
		t.Errorf("got %d open offers, want 1: an NPC waits for an answer", got)
	}
}

func TestProposeFromNPCsStopsAtTheDailyCap(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 3, 0, domain.ExchangeParams{MaxOffersPerDay: 2})
	fx.give(t, fx.keys[0], 2) // four copies, so three duplicates: one for each NPC

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	if got := len(fx.exchange.Offers()); got != 2 {
		t.Fatalf("day 1: got %d offers, want the cap of 2", got)
	}

	fx.exchange.StartDay(2)
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	if got := len(fx.exchange.Offers()); got != 3 {
		t.Errorf("day 2: got %d offers, want the third NPC to propose once the cap resets", got)
	}
}

func TestProposeFromNPCsNeedsAMatchAndAChance(t *testing.T) {
	t.Parallel()
	t.Run("no duplicate above the keep", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.player.Remove(fx.keys[0], 1); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		if got := len(fx.exchange.Offers()); got != 0 {
			t.Errorf("got %d offers for a card the player holds once, want 0", got)
		}
	})
	t.Run("a zero chance never proposes", func(t *testing.T) {
		t.Parallel()
		spec := tradingSpec(0)
		fx := newExchangeFixture(t, 1, spec, domain.ExchangeParams{})
		fx.give(t, fx.keys[0], 2)
		fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
		fx.shelve(0, fx.keys[1], 3)
		fx.exchange.StartDay(1)
		for range 50 {
			if err := fx.exchange.ProposeFromNPCs(); err != nil {
				t.Fatalf("ProposeFromNPCs: %v", err)
			}
		}
		if got := len(fx.exchange.Offers()); got != 0 {
			t.Errorf("got %d offers with a 0%% chance, want 0", got)
		}
	})
	t.Run("an empty shelf has nothing to give", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.crowd.UnshelveForTest(0, fx.keys[1], 3); err != nil {
			t.Fatalf("UnshelveForTest: %v", err)
		}
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		if got := len(fx.exchange.Offers()); got != 0 {
			t.Errorf("got %d offers from an NPC with an empty shelf, want 0", got)
		}
	})
}

func TestOffersLapseAfterTheirLastDay(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{ExpiryDays: 3})
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	fx.exchange.TakeDayStats()

	fx.exchange.StartDay(3)
	if got := len(fx.exchange.Offers()); got != 1 {
		t.Fatalf("day 3: got %d offers, want the offer of day 1 still open: it lasts 3 days", got)
	}

	fx.exchange.StartDay(4)
	if got := len(fx.exchange.Offers()); got != 0 {
		t.Errorf("day 4: got %d offers, want the offer lapsed", got)
	}
	if got := fx.exchange.TakeDayStats().Expired; got != 1 {
		t.Errorf("Expired = %d, want 1", got)
	}
	history := fx.exchange.History()
	if len(history) == 0 || history[len(history)-1].Outcome != domain.OutcomeExpired {
		t.Errorf("history = %+v, want a last entry with the expired outcome", history)
	}
	if _, err := fx.exchange.Accept(1); !errors.Is(err, domain.ErrOfferNotFound) {
		t.Errorf("Accept of a lapsed offer = %v, want ErrOfferNotFound", err)
	}
}

func TestAcceptRefusesAndClosesAnOfferWhoseCardsAreGone(t *testing.T) {
	t.Parallel()
	t.Run("the NPC no longer holds what it gives", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		offer := fx.exchange.Offers()[0]
		if err := fx.crowd.UnshelveForTest(0, fx.keys[1], 1); err != nil {
			t.Fatalf("UnshelveForTest: %v", err)
		}
		before := fx.state(0)

		d, err := fx.exchange.Accept(offer.ID)

		if err != nil {
			t.Fatalf("Accept: %v, want no error for a stale offer", err)
		}
		if d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrNPCLacksCard) {
			t.Errorf("Decision = %+v, want a refusal with ErrNPCLacksCard", d)
		}
		fx.requireUnchanged(t, 0, before)
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the stale offer is still open")
		}
	})
	t.Run("the player sold what it offers", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		offer := fx.exchange.Offers()[0]
		if err := fx.player.Remove(fx.keys[0], 2); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		before := fx.state(0)

		d, err := fx.exchange.Accept(offer.ID)

		if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrPlayerLacksCard) {
			t.Errorf("Accept = %+v, %v, want a refusal with ErrPlayerLacksCard", d, err)
		}
		fx.requireUnchanged(t, 0, before)
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the stale offer is still open")
		}
	})
}

func TestCounterReplacesWhatThePlayerGives(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T, replacementPrice domain.USDCents) (*exchangeFixture, domain.Offer, domain.CardKey) {
		t.Helper()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		replacement := fx.keys[2]
		fx.setPrice(replacement, replacementPrice)
		fx.give(t, replacement, 1)
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		return fx, fx.exchange.Offers()[0], replacement
	}

	t.Run("a replacement worth enough is accepted", func(t *testing.T) {
		t.Parallel()
		fx, offer, replacement := setup(t, 300)

		d, err := fx.exchange.Counter(offer.ID, lineOf(replacement, 1), 0)

		if err != nil || d.Outcome != domain.OutcomeAccepted {
			t.Fatalf("Counter = %+v, %v, want accepted: 300 received for 300 given", d, err)
		}
		if got := fx.player.Quantity(fx.keys[1]); got != 3 {
			t.Errorf("player holds %d of the NPC's cards, want 3", got)
		}
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the countered offer is still open")
		}
	})
	t.Run("a replacement worth too little is refused, the NPC does not counter again", func(t *testing.T) {
		t.Parallel()
		fx, offer, replacement := setup(t, 200)
		before := fx.state(0)

		d, err := fx.exchange.Counter(offer.ID, lineOf(replacement, 1), 0)

		if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrBelowMargin) {
			t.Fatalf("Counter = %+v, %v, want a refusal with ErrBelowMargin", d, err)
		}
		fx.requireUnchanged(t, 0, before)
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the original offer is still open after the player countered it")
		}
	})
}

func TestCounterRejectsUnknownAndNPCCounterOffers(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, tradingSpec(10), domain.ExchangeParams{})
	fx.setPrice(fx.keys[0], 100)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 1)
	fx.shelve(0, fx.keys[1], 1)
	counter, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
	if err != nil || counter.Outcome != domain.OutcomeCountered {
		t.Fatalf("setup: %+v, %v", counter, err)
	}

	if _, err := fx.exchange.Counter(counter.Offer.ID, lineOf(fx.keys[0], 1), 0); !errors.Is(err, domain.ErrNotCounterable) {
		t.Errorf("Counter of an NPC counter-offer = %v, want ErrNotCounterable: an NPC counters once", err)
	}
	if _, err := fx.exchange.Counter(999, lineOf(fx.keys[0], 1), 0); !errors.Is(err, domain.ErrOfferNotFound) {
		t.Errorf("Counter of an unknown offer = %v, want ErrOfferNotFound", err)
	}
}

func TestHoldersOfFollowsTheShelves(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 3, tradingSpec(0), domain.ExchangeParams{})
	fx.shelve(0, fx.keys[3], 1)
	fx.shelve(2, fx.keys[3], 2)

	if got, want := fx.exchange.HoldersOf(fx.keys[3]), []int{0, 2}; !slices.Equal(got, want) {
		t.Errorf("HoldersOf = %v, want %v", got, want)
	}
	if got, want := fx.exchange.Shelf(2), lineOf(fx.keys[3], 2); !slices.Equal(got, want) {
		t.Errorf("Shelf(2) = %v, want %v", got, want)
	}
	if fx.exchange.NPCCount() != 3 {
		t.Errorf("NPCCount = %d, want 3", fx.exchange.NPCCount())
	}
}

func TestProposeFromNPCsNeverOffersTheSameDuplicateTwice(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 2, 0, domain.ExchangeParams{})

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}

	offers := fx.exchange.Offers()
	if len(offers) != 1 {
		t.Fatalf("got %d offers for one duplicate, want 1: two NPCs must not both ask for the same copy", len(offers))
	}
	if d, err := fx.exchange.Accept(offers[0].ID); err != nil || d.Outcome != domain.OutcomeAccepted {
		t.Fatalf("Accept = %+v, %v, want accepted", d, err)
	}
	if got := fx.player.Quantity(fx.keys[0]); got != 1 {
		t.Errorf("player holds %d copies of the card, want its last copy kept", got)
	}
}

func TestProposeFromNPCsOffersEachDuplicateOnce(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 2, 0, domain.ExchangeParams{})
	fx.give(t, fx.keys[0], 1) // three copies, so two duplicates

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}

	offers := fx.exchange.Offers()
	if len(offers) != 2 {
		t.Fatalf("got %d offers for two duplicates, want 2", len(offers))
	}
	for _, o := range offers {
		if d, err := fx.exchange.Accept(o.ID); err != nil || d.Outcome != domain.OutcomeAccepted {
			t.Fatalf("Accept(%d) = %+v, %v, want accepted", o.ID, d, err)
		}
	}
	if got := fx.player.Quantity(fx.keys[0]); got != 1 {
		t.Errorf("player holds %d copies after both trades, want its last copy kept", got)
	}
}

func TestProposeFromNPCsSharesTheDailyCapAcrossTheCrowd(t *testing.T) {
	t.Parallel()
	const npcCount, days, dailyCap = 40, 300, 5
	fx := offerFixture(t, npcCount, 0, domain.ExchangeParams{ExpiryDays: 1, MaxOffersPerDay: dailyCap})
	fx.give(t, fx.keys[0], 2*dailyCap) // enough duplicates for every offer of a day

	offersTo := make([]int, npcCount)
	for day := 1; day <= days; day++ {
		fx.exchange.StartDay(day)
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		for _, o := range fx.exchange.Offers() {
			offersTo[o.NPC]++
		}
	}

	var low, high int
	for id, n := range offersTo {
		if id < npcCount/2 {
			low += n
		} else {
			high += n
		}
	}
	total := low + high
	if total == 0 {
		t.Fatal("no offers at all")
	}
	if low*100 < total*35 || high*100 < total*35 {
		t.Errorf("offers went %d to the first 20 NPCs and %d to the last 20: the cap must not favour low IDs", low, high)
	}
}
