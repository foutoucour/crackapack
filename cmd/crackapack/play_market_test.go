package main

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The demo lets a player use both sides of the market in a minute: asking around for a card of the
// open set gets an answer at once and another hours later, and the rare of a booster just opened,
// listed, draws an offer at once, which sells it.
func TestStartGame_theDemoMarketAnswersAtOnceAndLater(t *testing.T) {
	t.Parallel()
	a := demoGameArgs(t)
	clock := playtest.NewClock()
	a.now = clock.Now
	game, _, err := startGame(a, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}

	request := askUntilAFastAndASlowNPCHaveIt(t, game)
	clock.Advance(24 * time.Hour)
	v, err := game.Market()
	if err != nil {
		t.Fatalf("Market: %v", err)
	}
	if got := v.Requests[0]; got.ID != request.ID || got.Pending != 0 || len(got.Answers) <= len(request.Answers) {
		t.Fatalf("a day later, request = %+v, want the slow NPCs' answers in", got)
	}
	cheapest := cheapestAnswer(v.Requests[0].Answers)
	if _, err := game.AcceptAnswer(cheapest.ID); err != nil {
		t.Fatalf("AcceptAnswer(%+v), balance %d: %v", cheapest, game.Balance(), err)
	}

	opening, err := game.Open("DEM")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// The rare comes last, and is worth an offer.
	card := opening.Cards[len(opening.Cards)-1]
	if _, err := game.List(card.Card.ID, card.Finish, 1000); err != nil {
		t.Fatalf("List: %v", err)
	}
	if v, err = game.Market(); err != nil || len(v.Listings) != 1 || len(v.Listings[0].Answers) == 0 {
		t.Fatalf("listings = %+v, %v, want an offer at once", v.Listings, err)
	}
	before := game.Balance()
	offer := v.Listings[0].Answers[0]
	if _, err := game.AcceptAnswer(offer.ID); err != nil {
		t.Fatalf("AcceptAnswer(%+v): %v", offer, err)
	}
	if game.Balance() != before+offer.Coins {
		t.Errorf("balance = %d, want %d + the offer's %d", game.Balance(), before, offer.Coins)
	}
}

// askUntilAFastAndASlowNPCHaveIt asks around for the cards of DEM in turn until one gets an answer
// at once and leaves an NPC still to answer, and returns that request.
func askUntilAFastAndASlowNPCHaveIt(t *testing.T, game *play.Game) play.Request {
	t.Helper()
	for i := range 80 {
		id := domain.CardID(fmt.Sprintf("dem-c-%03d", i))
		rid, err := game.AskAround(id, domain.FinishNonFoil)
		if errors.Is(err, play.ErrNobodyHasIt) {
			continue
		}
		if err != nil {
			t.Fatalf("AskAround(%s): %v", id, err)
		}
		v, err := game.Market()
		if err != nil {
			t.Fatalf("Market: %v", err)
		}
		for _, r := range v.Requests {
			if r.ID == rid && len(r.Answers) > 0 && r.Pending > 0 {
				return r
			}
		}
		if err := game.Withdraw(rid); err != nil {
			t.Fatalf("Withdraw: %v", err)
		}
	}
	t.Fatal("no common of DEM is held by an NPC that answers at once and by one that answers later")
	return play.Request{}
}

func cheapestAnswer(answers []play.Answer) play.Answer {
	cheapest := answers[0]
	for _, a := range answers[1:] {
		if a.Coins < cheapest.Coins {
			cheapest = a
		}
	}
	return cheapest
}
