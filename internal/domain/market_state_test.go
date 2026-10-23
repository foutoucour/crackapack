package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// midNegotiation is a market with a request answered by one NPC and waiting for another, a listing
// waiting for the speculator's offer, and a counter waiting for the slow NPC's reply.
func midNegotiation(t *testing.T) *marketFixture {
	t.Helper()
	fx := newMarketFixture(t)
	wanted, other, listed := fx.keys[0], fx.keys[1], fx.keys[2]
	fx.give(t, listed, 1)
	fx.give(t, fx.keys[3], 2)
	fx.crowd.ShelveForTest(fastNPC, wanted, 1)
	fx.crowd.ShelveForTest(slowNPC, wanted, 1)
	fx.crowd.ShelveForTest(slowNPC, other, 1)
	return fx
}

func (fx *marketFixture) negotiate(t *testing.T) {
	t.Helper()
	start := marketStart()
	fx.ask(t, fx.keys[1], start)
	fx.advance(t, start.Add(slowDelay))
	if err := fx.market.Counter(fx.answerFrom(t, slowNPC).ID, 20, lineOf(fx.keys[3], 1), start.Add(slowDelay)); err != nil {
		t.Fatalf("Counter: %v", err)
	}
	fx.ask(t, fx.keys[0], start.Add(slowDelay))
	fx.list(t, fx.keys[2], 70, start.Add(slowDelay))
	fx.advance(t, start.Add(slowDelay+time.Minute))
}

type marketSeen struct {
	Requests []domain.MarketRequest
	Listings []domain.MarketListing
	Answers  []domain.MarketAnswer
	Results  []domain.MarketResult
	Player   map[domain.CardKey]int
	Balance  domain.Coins
}

func (fx *marketFixture) seen() marketSeen {
	return marketSeen{fx.market.Requests(), fx.market.Listings(), fx.market.Answers(), fx.market.Results(), fx.player.Snapshot(), fx.balance()}
}

// A market restored from the state of another, in the middle of a negotiation, goes on as the other
// does: the same answers come at the same instants and the counter gets the same reply.
func TestMarketRestore_continuesAsTheMarketItCameFrom(t *testing.T) {
	t.Parallel()
	a := midNegotiation(t)
	a.negotiate(t)
	b := midNegotiation(t)
	if err := b.market.Restore(a.market.State()); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got, want := b.seen(), a.seen(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after Restore: %+v, want %+v", got, want)
	}
	if b.market.Reserved(a.keys[3]) != 1 || b.market.Reserved(a.keys[2]) != 1 {
		t.Errorf("reserved %d and %d, want the counter's card and the listed one", b.market.Reserved(a.keys[3]), b.market.Reserved(a.keys[2]))
	}
	for _, step := range []time.Duration{time.Hour, slowDelay, 2 * slowDelay, 4 * 24 * time.Hour} {
		at := marketStart().Add(step)
		a.advance(t, at)
		b.advance(t, at)
		if got, want := b.seen(), a.seen(); !reflect.DeepEqual(got, want) {
			t.Fatalf("at +%v: %+v, want %+v", step, got, want)
		}
	}
	if len(a.market.Results()) < 3 {
		t.Errorf("results = %+v, want the reply, the offer's and the answers' lapses", a.market.Results())
	}
}

func TestMarketRestore_refusesAStateThatCannotBe(t *testing.T) {
	t.Parallel()
	at := marketStart()
	card := func(fx *marketFixture) domain.CardKey { return fx.keys[2] }
	tests := []struct {
		name  string
		state func(fx *marketFixture) domain.MarketState
	}{
		{name: "an answer to nothing", state: func(fx *marketFixture) domain.MarketState {
			return domain.MarketState{NextID: 3, Answers: []domain.MarketAnswer{{ID: 2, To: 1, Kind: domain.AnswerAsk, Card: card(fx), Coins: 5, At: at, Expires: at}}}
		}},
		{name: "an answer from an NPC the crowd lacks", state: func(fx *marketFixture) domain.MarketState {
			return domain.MarketState{
				NextID:   3,
				Requests: []domain.MarketRequest{{ID: 1, Card: card(fx), At: at, Asked: 1}},
				Answers:  []domain.MarketAnswer{{ID: 2, To: 1, Kind: domain.AnswerAsk, Card: card(fx), NPC: 9, Coins: 5, At: at, Expires: at}},
			}
		}},
		{name: "a listing of a card the player lacks", state: func(fx *marketFixture) domain.MarketState {
			return domain.MarketState{NextID: 2, Listings: []domain.MarketListing{{ID: 1, Card: fx.keys[5], Price: 5, At: at}}}
		}},
		{name: "a card listed twice", state: func(fx *marketFixture) domain.MarketState {
			return domain.MarketState{NextID: 3, Listings: []domain.MarketListing{{ID: 1, Card: card(fx), Price: 5, At: at}, {ID: 2, Card: card(fx), Price: 5, At: at}}}
		}},
		{name: "an ID not below the next one", state: func(fx *marketFixture) domain.MarketState {
			return domain.MarketState{NextID: 1, Listings: []domain.MarketListing{{ID: 1, Card: card(fx), Price: 5, At: at}}}
		}},
		{name: "a pending answer from an NPC the crowd lacks", state: func(fx *marketFixture) domain.MarketState {
			return domain.MarketState{
				NextID:   2,
				Listings: []domain.MarketListing{{ID: 1, Card: card(fx), Price: 5, At: at}},
				Pending:  []domain.MarketPending{{To: 1, NPC: -1, Due: at}},
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := midNegotiation(t)
			if err := fx.market.Restore(tt.state(fx)); err == nil {
				t.Error("Restore went through")
			}
			if len(fx.market.Listings()) != 0 || len(fx.market.Answers()) != 0 {
				t.Error("a refused Restore changed the market")
			}
		})
	}
}

// When the NPCs are new ones, what the old NPCs said and were waiting to say is dropped, and what
// the player asked and listed stays.
func TestMarketForgetNPCs(t *testing.T) {
	t.Parallel()
	fx := midNegotiation(t)
	fx.negotiate(t)
	fx.market.ForgetNPCs()
	if len(fx.market.Answers()) != 0 || fx.market.Pending(fx.market.Listings()[0].ID) != 0 {
		t.Errorf("answers %+v, want none and nothing pending", fx.market.Answers())
	}
	if len(fx.market.Requests()) != 2 || len(fx.market.Listings()) != 1 {
		t.Errorf("requests %+v, listings %+v, want both requests and the listing kept", fx.market.Requests(), fx.market.Listings())
	}
	if fx.market.Reserved(fx.keys[3]) != 0 {
		t.Errorf("the card of a counter that will get no reply is still reserved")
	}
}
