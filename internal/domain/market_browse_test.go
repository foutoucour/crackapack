package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ForSale lists what the NPCs that trade hold on their shelves, one entry per card, with each
// holder's asking price, the cheapest first: the card's reference value plus the NPC's margin.
func TestMarketForSale_listsEachShelfCardWithTheAskingPricesCheapestFirst(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	fx.crowd.ShelveForTest(slowNPC, fx.keys[0], 2)
	fx.crowd.ShelveForTest(fastNPC, fx.keys[0], 1)
	fx.crowd.ShelveForTest(specNPC, fx.keys[1], 1)

	want := []domain.MarketSale{
		{Card: fx.keys[0], Sellers: []domain.MarketSeller{{NPC: fastNPC, Price: 100, Qty: 1}, {NPC: slowNPC, Price: 110, Qty: 2}}},
		{Card: fx.keys[1], Sellers: []domain.MarketSeller{{NPC: specNPC, Price: 120, Qty: 1}}},
	}
	if got := fx.market.ForSale(); !reflect.DeepEqual(got, want) {
		t.Errorf("ForSale = %+v, want %+v", got, want)
	}
}

func TestMarketForSale_isEmptyWhenNoShelfHoldsACard(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	if got := fx.market.ForSale(); len(got) != 0 {
		t.Errorf("ForSale = %+v, want none", got)
	}
}

// The price of ForSale is the price an answer to a request carries, so Buy now and ask around cannot
// disagree.
func TestMarketForSale_pricesAsAnAnswerToARequestDoes(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	fx.prices[fx.keys[0].ID] = 1234
	fx.crowd.ShelveForTest(slowNPC, fx.keys[0], 1)
	fx.crowd.ShelveForTest(specNPC, fx.keys[0], 1)
	fx.ask(t, fx.keys[0], marketStart())
	fx.advance(t, marketStart().Add(slowDelay))

	sellers := fx.market.ForSale()[0].Sellers
	for _, s := range sellers {
		if got := fx.answerFrom(t, s.NPC).Coins; got != s.Price {
			t.Errorf("NPC %d: ForSale asks %d, its answer asks %d", s.NPC, s.Price, got)
		}
	}
}

func TestMarketBuyNow_paysTheAskingPriceAndTakesTheCardAtOnce(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	fx.crowd.ShelveForTest(fastNPC, card, 1)

	a, err := fx.market.BuyNow(slowNPC, card, marketStart())
	if err != nil {
		t.Fatalf("BuyNow: %v", err)
	}

	if a.NPC != slowNPC || a.Coins != 110 || a.Kind != domain.AnswerAsk {
		t.Errorf("answer = %+v, want the slow NPC's asking price of 110", a)
	}
	if got := fx.balance(); got != 890 {
		t.Errorf("balance = %d, want 890", got)
	}
	if got := fx.player.Quantity(card); got != 1 {
		t.Errorf("the player holds %d copies, want 1", got)
	}
	if got := fx.npcs[slowNPC].ShelfQuantity(card); got != 0 {
		t.Errorf("the NPC's shelf holds %d copies, want 0", got)
	}
	if got := fx.npcs[fastNPC].ShelfQuantity(card); got != 1 {
		t.Errorf("another NPC's shelf holds %d copies, want 1", got)
	}
	if r := fx.lastResult(t); r.Outcome != domain.MarketAccepted || r.Answer.NPC != slowNPC {
		t.Errorf("last result = %+v, want an accepted trade with the slow NPC", r)
	}
	if sum := fx.journal.SumAllBalances(); sum != 0 {
		t.Errorf("the journal sums to %d", sum)
	}
}

func TestMarketBuyNow_changesNothingWhenItCannotGoThrough(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		npc    int
		coins  domain.Coins
		shelve bool
		want   error
	}{
		{name: "the shelf is empty", npc: slowNPC, coins: 1000, want: domain.ErrNPCLacksCard},
		{name: "the NPC is unknown", npc: 99, coins: 1000, shelve: true, want: domain.ErrNPCLacksCard},
		{name: "the coins are short", npc: slowNPC, coins: 109, shelve: true, want: domain.ErrInsufficientBalance},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newMarketFixture(t)
			card := fx.keys[0]
			if tc.shelve {
				fx.crowd.ShelveForTest(slowNPC, card, 1)
			}
			if spent := fx.balance() - tc.coins; spent > 0 {
				if err := fx.journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounterTreasury, Amount: spent}); err != nil {
					t.Fatal(err)
				}
			}

			_, err := fx.market.BuyNow(tc.npc, card, marketStart())

			if !errors.Is(err, tc.want) {
				t.Fatalf("BuyNow error = %v, want %v", err, tc.want)
			}
			if got := fx.balance(); got != tc.coins {
				t.Errorf("balance = %d, want %d", got, tc.coins)
			}
			if got := fx.player.Quantity(card); got != 0 {
				t.Errorf("the player holds %d copies, want 0", got)
			}
			if got := fx.market.Results(); len(got) != 0 {
				t.Errorf("Results = %+v, want none", got)
			}
		})
	}
}

// An offer below the asking price is a counter without the ask-around step: the NPC replies once,
// after its delay, by the acceptance rule.
func TestMarketOffer_isRefusedBelowTheMarginAfterTheNPCsDelay(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	start := marketStart()

	if _, err := fx.market.Offer(slowNPC, card, 105, start); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	fx.advance(t, start.Add(slowDelay-time.Nanosecond))
	if got := fx.market.Results(); len(got) != 0 {
		t.Fatalf("Results before the NPC's delay = %+v, want none", got)
	}
	a := fx.answerFrom(t, slowNPC)
	if a.Counter == nil || a.Counter.Coins != 105 || a.Coins != 110 {
		t.Errorf("answer = %+v, want the asking price of 110 countered with 105", a)
	}

	fx.advance(t, start.Add(slowDelay))

	if r := fx.lastResult(t); r.Outcome != domain.MarketRefused {
		t.Errorf("outcome = %v, want refused", r.Outcome)
	}
	if got := fx.market.Requests(); len(got) != 0 {
		t.Errorf("Requests = %+v, want the direct request gone with its answer", got)
	}
	if got := fx.balance(); got != 1000 {
		t.Errorf("balance = %d, want 1000", got)
	}
}

// A gambler that is declined lowers its margin, so the offer it refused as the price stood is
// accepted once the reply comes: the same rule as a counter.
func TestMarketOffer_isAcceptedWhenTheNPCsMarginFellBeforeItReplied(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t, domain.TraitSpec{Name: "gambler"})
	card, other := fx.keys[0], fx.keys[1:3]
	start := marketStart()
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	for _, k := range other {
		fx.crowd.ShelveForTest(slowNPC, k, 1)
		fx.ask(t, k, start.Add(-slowDelay))
	}
	fx.advance(t, start)
	if _, err := fx.market.Offer(slowNPC, card, 120, start); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	for _, a := range fx.market.Answers() {
		if a.Counter == nil {
			if err := fx.market.Decline(a.ID); err != nil {
				t.Fatalf("Decline: %v", err)
			}
		}
	}

	fx.advance(t, start.Add(slowDelay))

	if r := fx.lastResult(t); r.Outcome != domain.MarketAccepted || r.Answer.Counter == nil || r.Answer.Counter.Coins != 120 {
		t.Fatalf("last result = %+v, want the offer of 120 accepted", r)
	}
	if got := fx.balance(); got != 880 {
		t.Errorf("balance = %d, want 880", got)
	}
	if got := fx.player.Quantity(card); got != 1 {
		t.Errorf("the player holds %d copies, want 1", got)
	}
}

func TestMarketOffer_isRefusedAtOnceWhenItMakesNoSense(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		coins domain.Coins
		held  bool
		want  error
	}{
		{name: "no coin", coins: 0, held: true, want: domain.ErrInvalidPrice},
		{name: "the asking price or more", coins: 110, held: true, want: domain.ErrOfferNotBelowAsk},
		{name: "more than the player has", coins: 105, held: true, want: domain.ErrInsufficientBalance},
		{name: "the NPC lacks the card", coins: 105, want: domain.ErrNPCLacksCard},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newMarketFixture(t)
			card := fx.keys[0]
			if tc.held {
				fx.crowd.ShelveForTest(slowNPC, card, 1)
			}
			if tc.want == domain.ErrInsufficientBalance {
				if err := fx.journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountCounterTreasury, Amount: fx.balance() - 100}); err != nil {
					t.Fatal(err)
				}
			}

			_, err := fx.market.Offer(slowNPC, card, tc.coins, marketStart())

			if !errors.Is(err, tc.want) {
				t.Errorf("Offer error = %v, want %v", err, tc.want)
			}
			if got := fx.market.Requests(); len(got) != 0 {
				t.Errorf("Requests = %+v, want none", got)
			}
		})
	}
}

// The player waits on a card once: an offer and a request for the same card exclude each other.
func TestMarketOffer_andAskAroundExcludeEachOtherWhileTheyWait(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	fx.crowd.ShelveForTest(specNPC, card, 1)
	if _, err := fx.market.Offer(slowNPC, card, 105, marketStart()); err != nil {
		t.Fatalf("Offer: %v", err)
	}

	if _, err := fx.market.AskAround(card, marketStart()); !errors.Is(err, domain.ErrAlreadyAsked) {
		t.Errorf("AskAround while an offer waits: %v, want ErrAlreadyAsked", err)
	}
	if _, err := fx.market.Offer(specNPC, card, 110, marketStart()); !errors.Is(err, domain.ErrAlreadyAsked) {
		t.Errorf("Offer while an offer waits: %v, want ErrAlreadyAsked", err)
	}
}

func TestMarketOffer_survivesASaveAndALoad(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	if _, err := fx.market.Offer(slowNPC, card, 105, marketStart()); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	state := fx.market.State()

	if err := fx.market.Restore(state); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if got := fx.market.State(); !reflect.DeepEqual(got, state) {
		t.Errorf("State after Restore = %+v, want %+v", got, state)
	}
	fx.advance(t, marketStart().Add(slowDelay))
	if got := fx.market.Requests(); len(got) != 0 {
		t.Errorf("Requests after the reply = %+v, want the direct request gone", got)
	}
}

// NextAnswer says when the next NPC answers a request, so that the page can tell how long to wait.
func TestMarketNextAnswer_isTheEarliestAnswerStillToCome(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	fx.crowd.ShelveForTest(specNPC, card, 1)
	id := fx.ask(t, card, marketStart())

	got, ok := fx.market.NextAnswer(id)
	if want := marketStart().Add(time.Hour); !ok || !got.Equal(want) {
		t.Errorf("NextAnswer = %v, %v, want %v", got, ok, want)
	}

	fx.advance(t, marketStart().Add(slowDelay))
	if got, ok := fx.market.NextAnswer(id); ok {
		t.Errorf("NextAnswer once all answered = %v, want none", got)
	}
}
