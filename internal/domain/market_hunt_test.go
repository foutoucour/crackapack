package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// The NPCs that hold the card answer the player, each after its delay, with what it asks for the
// card: its reference value plus its margin, in coins.
func TestMarketAskAround_holdersAnswerAfterTheirDelay(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(fastNPC, card, 1)
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	fx.crowd.ShelveForTest(specNPC, fx.keys[1], 1)
	start := marketStart()

	id := fx.ask(t, card, start)
	fx.advance(t, start)
	req := fx.market.Requests()
	if len(req) != 1 || req[0].ID != id || req[0].Card != card || req[0].Asked != 2 {
		t.Fatalf("Requests = %+v, want one for %v that asked 2 NPCs", req, card)
	}
	if got := fx.market.Pending(id); got != 1 {
		t.Errorf("Pending at once = %d, want 1: the slow NPC has not answered", got)
	}
	want := []domain.MarketAnswer{{ID: 2, To: id, Kind: domain.AnswerAsk, Card: card, NPC: fastNPC, Coins: 100, At: start, Expires: start.Add(marketExpiry)}}
	if got := fx.market.Answers(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Answers at once = %+v, want %+v", got, want)
	}

	fx.advance(t, start.Add(slowDelay-time.Nanosecond))
	if got := len(fx.market.Answers()); got != 1 {
		t.Fatalf("answers before the slow NPC's delay = %d, want 1", got)
	}
	fx.advance(t, start.Add(slowDelay))
	slow := fx.answerFrom(t, slowNPC)
	if slow.Coins != 110 || !slow.At.Equal(start.Add(slowDelay)) {
		t.Errorf("slow answer = %+v, want 110 coins at %v", slow, start.Add(slowDelay))
	}
	if got := fx.market.Pending(id); got != 0 {
		t.Errorf("Pending once both answered = %d, want 0", got)
	}
}

// An NPC that no longer holds the card when its answer is due does not answer.
func TestMarketAskAround_anNPCThatSoldTheCardMeanwhileDoesNotAnswer(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	id := fx.ask(t, card, marketStart())
	if err := fx.crowd.UnshelveForTest(slowNPC, card, 1); err != nil {
		t.Fatal(err)
	}
	fx.advance(t, marketStart().Add(slowDelay))
	if got := fx.market.Answers(); len(got) != 0 {
		t.Errorf("Answers = %+v, want none", got)
	}
	if got := fx.market.Pending(id); got != 0 {
		t.Errorf("Pending = %d, want 0", got)
	}
}

func TestMarketAskAround_refusals(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	if _, err := fx.market.AskAround(card, marketStart()); !errors.Is(err, domain.ErrNoHolder) {
		t.Errorf("AskAround of a card no NPC holds: error = %v, want ErrNoHolder", err)
	}
	fx.crowd.ShelveForTest(fastNPC, card, 1)
	fx.ask(t, card, marketStart())
	if _, err := fx.market.AskAround(card, marketStart()); !errors.Is(err, domain.ErrAlreadyAsked) {
		t.Errorf("AskAround twice: error = %v, want ErrAlreadyAsked", err)
	}
}

// A request whose answers have all lapsed is over: asking again starts a new one.
func TestMarketAskAround_againOnceTheAnswersLapsed(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(fastNPC, card, 1)
	first := fx.ask(t, card, marketStart())
	later := marketStart().Add(marketExpiry)
	fx.advance(t, later)
	if got := fx.lastResult(t); got.Outcome != domain.MarketExpired || got.Answer.NPC != fastNPC {
		t.Errorf("last result = %+v, want the fast NPC's answer expired", got)
	}
	second, err := fx.market.AskAround(card, later)
	if err != nil {
		t.Fatalf("AskAround again: %v", err)
	}
	if second == first || len(fx.market.Requests()) != 1 {
		t.Errorf("requests = %+v, want the new one %d only", fx.market.Requests(), second)
	}
}

// A refusal changes nothing: asking again for a card whose request is over, when nobody has it any
// more, keeps the old request.
func TestMarketAskAround_aRefusalKeepsTheRequestThatIsOver(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(fastNPC, card, 1)
	first := fx.ask(t, card, marketStart())
	later := marketStart().Add(marketExpiry)
	fx.advance(t, later)
	if err := fx.crowd.UnshelveForTest(fastNPC, card, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.market.AskAround(card, later); !errors.Is(err, domain.ErrNoHolder) {
		t.Fatalf("error = %v, want ErrNoHolder", err)
	}
	if got := fx.market.Requests(); len(got) != 1 || got[0].ID != first {
		t.Errorf("requests = %+v, want the old request %d kept", got, first)
	}
}

// Accepting an asking price pays it and moves the card at once, and ends the request.
func TestMarketAccept_anAskingPrice(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(fastNPC, card, 1)
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	fx.ask(t, card, marketStart())
	fx.advance(t, marketStart())

	answer := fx.answerFrom(t, fastNPC)
	if _, err := fx.market.Accept(answer.ID, marketStart()); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if fx.player.Quantity(card) != 1 || fx.npcs[fastNPC].ShelfQuantity(card) != 0 {
		t.Errorf("player holds %d, NPC shelf %d, want 1 and 0", fx.player.Quantity(card), fx.npcs[fastNPC].ShelfQuantity(card))
	}
	if fx.balance() != 900 || fx.journal.Balance(domain.AccountTradeTopUp) != 100 {
		t.Errorf("balance %d, trade top-up %d, want 900 and 100", fx.balance(), fx.journal.Balance(domain.AccountTradeTopUp))
	}
	if len(fx.market.Requests()) != 0 || len(fx.market.Answers()) != 0 {
		t.Errorf("requests %+v, answers %+v, want none: the player has the card", fx.market.Requests(), fx.market.Answers())
	}
	fx.advance(t, marketStart().Add(slowDelay))
	if got := fx.market.Answers(); len(got) != 0 {
		t.Errorf("the slow NPC answered a request that is over: %+v", got)
	}
	if got := fx.lastResult(t); got.Outcome != domain.MarketAccepted || got.Answer.ID != answer.ID {
		t.Errorf("last result = %+v, want the answer accepted", got)
	}
}

func TestMarketAccept_refusals(t *testing.T) {
	t.Parallel()
	t.Run("not enough coins leaves the answer open", func(t *testing.T) {
		t.Parallel()
		fx := newMarketFixture(t)
		card := fx.keys[0]
		fx.prices[card.ID] = 5000
		fx.crowd.ShelveForTest(fastNPC, card, 1)
		fx.ask(t, card, marketStart())
		fx.advance(t, marketStart())
		answer := fx.answerFrom(t, fastNPC)
		if _, err := fx.market.Accept(answer.ID, marketStart()); !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Errorf("error = %v, want ErrInsufficientBalance", err)
		}
		if fx.balance() != 1000 || fx.player.Quantity(card) != 0 || len(fx.market.Answers()) != 1 {
			t.Errorf("balance %d, card %d, answers %d, want nothing changed", fx.balance(), fx.player.Quantity(card), len(fx.market.Answers()))
		}
	})
	t.Run("an NPC that no longer holds the card closes the answer", func(t *testing.T) {
		t.Parallel()
		fx := newMarketFixture(t)
		card := fx.keys[0]
		fx.crowd.ShelveForTest(fastNPC, card, 1)
		fx.ask(t, card, marketStart())
		fx.advance(t, marketStart())
		answer := fx.answerFrom(t, fastNPC)
		if err := fx.crowd.UnshelveForTest(fastNPC, card, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := fx.market.Accept(answer.ID, marketStart()); !errors.Is(err, domain.ErrNPCLacksCard) {
			t.Errorf("error = %v, want ErrNPCLacksCard", err)
		}
		if fx.balance() != 1000 || len(fx.market.Answers()) != 0 {
			t.Errorf("balance %d, answers %+v, want 1000 and none", fx.balance(), fx.market.Answers())
		}
		if got := fx.lastResult(t); got.Outcome != domain.MarketFailed {
			t.Errorf("last result = %+v, want failed", got)
		}
	})
	t.Run("an unknown answer", func(t *testing.T) {
		t.Parallel()
		fx := newMarketFixture(t)
		if _, err := fx.market.Accept(99, marketStart()); !errors.Is(err, domain.ErrMarketNotFound) {
			t.Errorf("error = %v, want ErrMarketNotFound", err)
		}
	})
}

// The NPC replies to a counter once, after its delay, by the acceptance rule: it accepts what is
// worth its card plus its margin, and refuses less, which is a decline of its asking price.
func TestMarketCounter_onAnAskingPrice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		coins     domain.Coins
		cards     int // copies of fx.keys[1], at $1, that the player adds
		traits    []domain.TraitSpec
		want      domain.MarketOutcome
		wantPaid  domain.Coins
		wantCards int
	}{
		{name: "coins that pay the margin", coins: 110, want: domain.MarketAccepted, wantPaid: 110},
		{name: "a card and coins that pay the margin", coins: 10, cards: 1, want: domain.MarketAccepted, wantPaid: 10, wantCards: 1},
		{name: "coins below the margin", coins: 109, want: domain.MarketRefused},
		{name: "a trait that vetoes every counter", coins: 500, traits: []domain.TraitSpec{{Name: "ego"}}, want: domain.MarketRefused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newMarketFixture(t, tt.traits...)
			card, extra := fx.keys[0], fx.keys[1]
			fx.give(t, extra, 1)
			fx.crowd.ShelveForTest(slowNPC, card, 1)
			start := marketStart()
			fx.ask(t, card, start)
			fx.advance(t, start.Add(slowDelay))
			answer := fx.answerFrom(t, slowNPC)
			var give []domain.TradeLine
			if tt.cards > 0 {
				give = []domain.TradeLine{{Key: extra, Qty: tt.cards}}
			}
			counteredAt := start.Add(slowDelay + time.Hour)
			if err := fx.market.Counter(answer.ID, tt.coins, give, counteredAt); err != nil {
				t.Fatalf("Counter: %v", err)
			}
			if got := fx.market.Reserved(extra); got != tt.cards {
				t.Errorf("Reserved while waiting = %d, want %d", got, tt.cards)
			}
			fx.advance(t, counteredAt.Add(slowDelay-time.Nanosecond))
			if got := fx.answerFrom(t, slowNPC); got.Counter == nil || !got.Counter.ReplyAt.Equal(counteredAt.Add(slowDelay)) {
				t.Fatalf("answer before the reply = %+v, want the counter waiting until %v", got, counteredAt.Add(slowDelay))
			}
			fx.advance(t, counteredAt.Add(slowDelay))

			if got := fx.lastResult(t); got.Outcome != tt.want || got.Answer.ID != answer.ID {
				t.Errorf("last result = %+v, want outcome %v", got, tt.want)
			}
			if fx.market.Reserved(extra) != 0 || len(fx.market.Answers()) != 0 {
				t.Errorf("reserved %d, answers %+v, want none once the NPC replied", fx.market.Reserved(extra), fx.market.Answers())
			}
			wantStreak, wantCard := 1, 0
			if tt.want == domain.MarketAccepted {
				wantStreak, wantCard = 0, 1
			}
			if got := fx.npcs[slowNPC].DeclineStreak(); got != wantStreak {
				t.Errorf("decline streak = %d, want %d", got, wantStreak)
			}
			if fx.player.Quantity(card) != wantCard || fx.balance() != 1000-tt.wantPaid || fx.player.Quantity(extra) != 1-tt.wantCards {
				t.Errorf("player holds %d of the card, %d of the extra and %d coins, want %d, %d and %d",
					fx.player.Quantity(card), fx.player.Quantity(extra), fx.balance(), wantCard, 1-tt.wantCards, 1000-tt.wantPaid)
			}
		})
	}
}

func TestMarketCounter_refusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		coins domain.Coins
		cards func(fx *marketFixture) []domain.TradeLine
		want  error
	}{
		{name: "nothing offered", want: domain.ErrNothingOffered},
		{name: "more coins than the player has", coins: 1001, want: domain.ErrInsufficientBalance},
		{name: "a card the player lacks", cards: func(fx *marketFixture) []domain.TradeLine { return lineOf(fx.keys[2], 1) }, want: domain.ErrPlayerLacksCard},
		{name: "a card the player listed", cards: func(fx *marketFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) }, want: domain.ErrReserved},
		{name: "a negative top-up", coins: -1, want: domain.ErrInvalidLine},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newMarketFixture(t)
			card := fx.keys[0]
			fx.give(t, fx.keys[1], 1)
			fx.list(t, fx.keys[1], 100, marketStart())
			fx.crowd.ShelveForTest(fastNPC, card, 1)
			fx.ask(t, card, marketStart())
			fx.advance(t, marketStart())
			answer := fx.answerFrom(t, fastNPC)
			var cards []domain.TradeLine
			if tt.cards != nil {
				cards = tt.cards(fx)
			}
			if err := fx.market.Counter(answer.ID, tt.coins, cards, marketStart()); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
			if got := fx.answerFrom(t, fastNPC); got.Counter != nil {
				t.Errorf("answer = %+v, want it open with no counter", got)
			}
		})
	}
}

// An answer that waits for the NPC's reply is neither accepted, declined nor countered again, and
// it gets its reply even past the instant it would have lapsed.
func TestMarket_anAnswerWaitingForAReply(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	fx.ask(t, card, marketStart())
	fx.advance(t, marketStart().Add(slowDelay))
	answer := fx.answerFrom(t, slowNPC)
	countered := answer.Expires.Add(-time.Hour)
	if err := fx.market.Counter(answer.ID, 50, nil, countered); err != nil {
		t.Fatalf("Counter: %v", err)
	}
	if _, err := fx.market.Accept(answer.ID, countered); !errors.Is(err, domain.ErrAwaitingReply) {
		t.Errorf("Accept: error = %v, want ErrAwaitingReply", err)
	}
	if err := fx.market.Decline(answer.ID); !errors.Is(err, domain.ErrAwaitingReply) {
		t.Errorf("Decline: error = %v, want ErrAwaitingReply", err)
	}
	if err := fx.market.Counter(answer.ID, 60, nil, countered); !errors.Is(err, domain.ErrAwaitingReply) {
		t.Errorf("Counter again: error = %v, want ErrAwaitingReply", err)
	}
	fx.advance(t, countered.Add(slowDelay))
	if got := fx.lastResult(t); got.Outcome != domain.MarketRefused || !got.At.Equal(countered.Add(slowDelay)) {
		t.Errorf("last result = %+v, want the counter refused at %v and no lapse", got, countered.Add(slowDelay))
	}
}

// Declining an answer closes it, is told to the NPC as a decline, and moves nothing. An answer
// left alone lapses, which is no decline.
func TestMarketDecline_andExpiry(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(fastNPC, card, 1)
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	fx.ask(t, card, marketStart())
	fx.advance(t, marketStart().Add(slowDelay))

	if err := fx.market.Decline(fx.answerFrom(t, fastNPC).ID); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if got := fx.npcs[fastNPC].DeclineStreak(); got != 1 {
		t.Errorf("decline streak of the fast NPC = %d, want 1", got)
	}
	fx.advance(t, marketStart().Add(slowDelay+marketExpiry))
	if got := fx.lastResult(t); got.Outcome != domain.MarketExpired || got.Answer.NPC != slowNPC {
		t.Errorf("last result = %+v, want the slow NPC's answer expired", got)
	}
	if got := fx.npcs[slowNPC].DeclineStreak(); got != 0 {
		t.Errorf("decline streak of the slow NPC = %d, want 0: a lapse is no decline", got)
	}
	if fx.balance() != 1000 || fx.player.Quantity(card) != 0 {
		t.Errorf("balance %d, card %d, want nothing moved", fx.balance(), fx.player.Quantity(card))
	}
}

func TestMarketWithdraw(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.crowd.ShelveForTest(fastNPC, card, 1)
	fx.crowd.ShelveForTest(slowNPC, card, 1)
	id := fx.ask(t, card, marketStart())
	fx.advance(t, marketStart())
	if err := fx.market.Withdraw(id); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	fx.advance(t, marketStart().Add(slowDelay))
	if len(fx.market.Requests()) != 0 || len(fx.market.Answers()) != 0 {
		t.Errorf("requests %+v, answers %+v, want none", fx.market.Requests(), fx.market.Answers())
	}
	if err := fx.market.Withdraw(id); !errors.Is(err, domain.ErrMarketNotFound) {
		t.Errorf("Withdraw twice: error = %v, want ErrMarketNotFound", err)
	}
}
