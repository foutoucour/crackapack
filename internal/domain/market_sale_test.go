package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// A listed card is reserved: it cannot be listed twice, nor given in a counter, until it is sold
// or unlisted.
func TestMarketList_reservesTheCard(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.give(t, card, 2)
	id := fx.list(t, card, 150, marketStart())
	if got := fx.market.Reserved(card); got != 1 {
		t.Errorf("Reserved = %d, want 1", got)
	}
	want := []domain.MarketListing{{ID: id, Card: card, Price: 150, At: marketStart()}}
	if got := fx.market.Listings(); !reflect.DeepEqual(got, want) {
		t.Errorf("Listings = %+v, want %+v", got, want)
	}
	if _, err := fx.market.List(card, 150, marketStart()); !errors.Is(err, domain.ErrAlreadyListed) {
		t.Errorf("List twice: error = %v, want ErrAlreadyListed", err)
	}
	if err := fx.market.Unlist(id); err != nil {
		t.Fatalf("Unlist: %v", err)
	}
	if got := fx.market.Reserved(card); got != 0 || len(fx.market.Listings()) != 0 {
		t.Errorf("after Unlist: reserved %d, listings %+v, want none", got, fx.market.Listings())
	}
	if err := fx.market.Unlist(id); !errors.Is(err, domain.ErrMarketNotFound) {
		t.Errorf("Unlist twice: error = %v, want ErrMarketNotFound", err)
	}
}

func TestMarketList_refusals(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	held, missing := fx.keys[0], fx.keys[1]
	fx.give(t, held, 1)
	tests := []struct {
		name  string
		card  domain.CardKey
		price domain.Coins
		want  error
	}{
		{name: "a card the player lacks", card: missing, price: 10, want: domain.ErrPlayerLacksCard},
		{name: "a price of nothing", card: held, price: 0, want: domain.ErrInvalidPrice},
	}
	for _, tt := range tests {
		if _, err := fx.market.List(tt.card, tt.price, marketStart()); !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
	}
	if len(fx.market.Listings()) != 0 {
		t.Errorf("Listings = %+v, want none", fx.market.Listings())
	}
}

// The NPCs that wish for the listed card, and the speculators that lack it, make an offer after
// their delay: the cards on their shelf the player lacks, as many as the card is worth once their
// margin is paid, or else coins, never more than the asking price nor than the card is worth.
func TestMarketList_offers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		price domain.Coins
		setup func(fx *marketFixture)
		npc   int
		delay time.Duration
		coins domain.Coins
		cards func(fx *marketFixture) []domain.TradeLine
	}{
		{
			name: "a speculator that lacks the card pays what it is worth less its margin", price: 500,
			npc: specNPC, delay: time.Hour, coins: 83, // 100 coins / 1.20
		},
		{
			name: "no more than the asking price", price: 50,
			npc: specNPC, delay: time.Hour, coins: 50,
		},
		{
			name: "a wisher gives a card the player lacks", price: 500,
			setup: func(fx *marketFixture) {
				fx.crowd.SetWishesForTest(fastNPC, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
				fx.crowd.ShelveForTest(fastNPC, fx.keys[3], 2)
			},
			npc: fastNPC, cards: func(fx *marketFixture) []domain.TradeLine { return lineOf(fx.keys[3], 1) },
		},
		{
			name: "a wisher whose shelf only holds what the player has pays coins", price: 500,
			setup: func(fx *marketFixture) {
				fx.crowd.SetWishesForTest(fastNPC, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
				fx.crowd.ShelveForTest(fastNPC, fx.keys[2], 1)
			},
			npc: fastNPC, coins: 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newMarketFixture(t)
			card := fx.keys[0]
			fx.give(t, card, 1)
			fx.give(t, fx.keys[2], 1)
			if tt.setup != nil {
				tt.setup(fx)
			}
			id := fx.list(t, card, tt.price, marketStart())
			fx.advance(t, marketStart().Add(tt.delay-time.Nanosecond))
			for _, a := range fx.market.Answers() {
				if a.NPC == tt.npc {
					t.Fatalf("NPC %d offered before its delay: %+v", tt.npc, a)
				}
			}
			fx.advance(t, marketStart().Add(tt.delay))
			got := fx.answerFrom(t, tt.npc)
			var cards []domain.TradeLine
			if tt.cards != nil {
				cards = tt.cards(fx)
			}
			if got.To != id || got.Kind != domain.AnswerOffer || got.Coins != tt.coins || !reflect.DeepEqual(got.Cards, cards) {
				t.Errorf("offer = %+v, want %d coins and cards %+v on listing %d", got, tt.coins, cards, id)
			}
		})
	}
}

// An NPC that neither wishes for the card nor speculates makes no offer.
func TestMarketList_noOfferFromAnNPCThatDoesNotWantTheCard(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.give(t, card, 1)
	if err := fx.npcs[specNPC].Collection().Add(card, 1); err != nil {
		t.Fatal(err)
	}
	fx.list(t, card, 100, marketStart())
	fx.advance(t, marketStart().Add(48*time.Hour))
	if got := fx.market.Answers(); len(got) != 0 {
		t.Errorf("Answers = %+v, want none: the speculator holds the card and nobody wishes for it", got)
	}
}

// Accepting an offer gives the listed card for the NPC's coins or cards at once, and ends the
// listing. The coins an NPC pays enter through the market's account, since NPCs hold none.
func TestMarketAccept_anOffer(t *testing.T) {
	t.Parallel()
	t.Run("coins", func(t *testing.T) {
		t.Parallel()
		fx := newMarketFixture(t)
		card := fx.keys[0]
		fx.give(t, card, 1)
		fx.list(t, card, 500, marketStart())
		fx.advance(t, marketStart().Add(time.Hour))
		answer := fx.answerFrom(t, specNPC)
		if _, err := fx.market.Accept(answer.ID, marketStart().Add(time.Hour)); err != nil {
			t.Fatalf("Accept: %v", err)
		}
		if fx.player.Quantity(card) != 0 || fx.npcs[specNPC].Collection().Copies(card.ID) != 1 {
			t.Errorf("player holds %d, NPC %d, want 0 and 1", fx.player.Quantity(card), fx.npcs[specNPC].Collection().Copies(card.ID))
		}
		if fx.balance() != 1083 || fx.journal.Balance(domain.AccountMarketSale) != -83 {
			t.Errorf("balance %d, market sale %d, want 1083 and -83", fx.balance(), fx.journal.Balance(domain.AccountMarketSale))
		}
		if len(fx.market.Listings()) != 0 || fx.market.Reserved(card) != 0 {
			t.Errorf("listings %+v, reserved %d, want none", fx.market.Listings(), fx.market.Reserved(card))
		}
	})
	t.Run("cards", func(t *testing.T) {
		t.Parallel()
		fx := newMarketFixture(t)
		card, theirs := fx.keys[0], fx.keys[3]
		fx.give(t, card, 1)
		fx.crowd.SetWishesForTest(fastNPC, []domain.Wish{{ID: card.ID, Want: 1}})
		fx.crowd.ShelveForTest(fastNPC, theirs, 1)
		fx.list(t, card, 500, marketStart())
		fx.advance(t, marketStart())
		if _, err := fx.market.Accept(fx.answerFrom(t, fastNPC).ID, marketStart()); err != nil {
			t.Fatalf("Accept: %v", err)
		}
		if fx.player.Quantity(card) != 0 || fx.player.Quantity(theirs) != 1 || fx.balance() != 1000 {
			t.Errorf("player holds %d of the listed card, %d of the NPC's and %d coins, want 0, 1 and 1000",
				fx.player.Quantity(card), fx.player.Quantity(theirs), fx.balance())
		}
	})
}

// A counter on an offer asks for coins, which the NPC pays when they are no more than the card is
// worth to it once its margin is paid.
func TestMarketCounter_onAnOffer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		coins domain.Coins
		want  domain.MarketOutcome
	}{
		{name: "what the card is worth less the margin", coins: 83, want: domain.MarketAccepted},
		{name: "more", coins: 84, want: domain.MarketRefused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newMarketFixture(t)
			card := fx.keys[0]
			fx.give(t, card, 1)
			fx.list(t, card, 60, marketStart())
			fx.advance(t, marketStart().Add(time.Hour))
			answer := fx.answerFrom(t, specNPC)
			if answer.Coins != 60 {
				t.Fatalf("offer = %+v, want the asking price of 60", answer)
			}
			if err := fx.market.Counter(answer.ID, tt.coins, nil, marketStart().Add(time.Hour)); err != nil {
				t.Fatalf("Counter: %v", err)
			}
			fx.advance(t, marketStart().Add(2*time.Hour))
			if got := fx.lastResult(t); got.Outcome != tt.want {
				t.Errorf("last result = %+v, want %v", got, tt.want)
			}
			wantBalance, wantHeld, wantListings := domain.Coins(1000), 1, 1
			if tt.want == domain.MarketAccepted {
				wantBalance, wantHeld, wantListings = 1000+tt.coins, 0, 0
			}
			if fx.balance() != wantBalance || fx.player.Quantity(card) != wantHeld || len(fx.market.Listings()) != wantListings {
				t.Errorf("balance %d, card %d, listings %d, want %d, %d and %d",
					fx.balance(), fx.player.Quantity(card), len(fx.market.Listings()), wantBalance, wantHeld, wantListings)
			}
		})
	}
}

func TestMarketCounter_anOfferTakesCoinsOnly(t *testing.T) {
	t.Parallel()
	fx := newMarketFixture(t)
	card := fx.keys[0]
	fx.give(t, card, 1)
	fx.give(t, fx.keys[1], 1)
	fx.list(t, card, 60, marketStart())
	fx.advance(t, marketStart().Add(time.Hour))
	answer := fx.answerFrom(t, specNPC)
	tests := []struct {
		coins domain.Coins
		cards []domain.TradeLine
		want  error
	}{
		{coins: 80, cards: lineOf(fx.keys[1], 1), want: domain.ErrCoinsOnly},
		{coins: 0, want: domain.ErrInvalidPrice},
	}
	for _, tt := range tests {
		if err := fx.market.Counter(answer.ID, tt.coins, tt.cards, marketStart().Add(time.Hour)); !errors.Is(err, tt.want) {
			t.Errorf("Counter(%d coins, %v): error = %v, want %v", tt.coins, tt.cards, err, tt.want)
		}
	}
}
