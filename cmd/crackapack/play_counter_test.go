package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// The demo lets a player trade at the Counter from the start: a rare of the open set is in stock,
// a mythic never was, a card just opened sells for coins and a rare can be bought.
func TestStartGame_theDemoCounterSellsAndBuys(t *testing.T) {
	t.Parallel()
	game, _, err := startGame(demoGameArgs(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}
	if got := game.Balance(); got != 200 {
		t.Fatalf("Balance = %d, want the welcome bonus of 200", got)
	}

	mythic, err := game.CounterQuote("dem-m-000", domain.FinishNonFoil)
	if err != nil {
		t.Fatalf("quote of a mythic: %v", err)
	}
	if mythic.InStock || !mythic.LastInStock.IsZero() || mythic.BuyPrice == 0 {
		t.Errorf("quote of a mythic = %+v, want a price and never in stock", mythic)
	}

	opening, err := game.Open("DEM")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	card := opening.Cards[0]
	paid, err := game.Sell(card.Card.ID, card.Finish, 1)
	if err != nil || paid <= 0 || game.Balance() != 200+paid {
		t.Fatalf("Sell = %d, %v, balance %d, want coins paid", paid, err, game.Balance())
	}

	for _, id := range []domain.CardID{"dem-r-003", "dem-r-004", "dem-r-005"} {
		q, err := game.CounterQuote(id, domain.FinishNonFoil)
		if err != nil {
			t.Fatalf("quote of %s: %v", id, err)
		}
		if !q.InStock || q.BuyPrice > game.Balance() {
			continue
		}
		before := game.Balance()
		p, err := game.Buy(id, domain.FinishNonFoil)
		if err != nil || p.Paid != q.BuyPrice || game.Balance() != before-q.BuyPrice {
			t.Fatalf("Buy(%s) = %+v, %v, balance %d, want %d paid", id, p, err, game.Balance(), q.BuyPrice)
		}
		return
	}
	t.Fatal("no demo rare was in stock at a price the player could pay")
}

// A purchase in the demo's locked set is refused until the set unlocks.
func TestStartGame_theDemoRefusesACardOfALockedSet(t *testing.T) {
	t.Parallel()
	game, _, err := startGame(demoGameArgs(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}
	if _, err := game.Buy("dsp-c-000", domain.FinishNonFoil); !errors.Is(err, play.ErrLocked) {
		t.Errorf("Buy of a card of the locked set error = %v, want ErrLocked", err)
	}
}

// demoGameArgs is demoPlayArgs for a test that starts the game without serving it.
func demoGameArgs(t *testing.T) playArgs {
	t.Helper()
	a := demoPlayArgs(t)
	a.stdout = io.Discard
	return a
}
