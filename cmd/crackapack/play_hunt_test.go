package main

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The demo lets a player hunt in a minute: hunting a few commons of the open set brings at once the
// offers of the NPCs that hold them, and tells which of them the Counter has.
func TestStartGame_theDemoHuntOffersAndFindsCardsAtTheCounter(t *testing.T) {
	t.Parallel()
	a := demoGameArgs(t)
	a.now = playtest.NewClock().Now
	game, _, err := startGame(a, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}

	const hunts = 10
	for i := range hunts {
		if err := game.Hunt(domain.CardID(fmt.Sprintf("dem-c-%03d", i)), domain.FinishNonFoil); err != nil {
			t.Fatalf("Hunt: %v", err)
		}
	}

	hunted, err := game.Hunted()
	if err != nil {
		t.Fatalf("Hunted: %v", err)
	}
	if len(hunted) != hunts {
		t.Fatalf("hunted %d cards, want %d", len(hunted), hunts)
	}
	counter, offers := countHunts(hunted)
	if counter == 0 || offers == 0 {
		t.Errorf("of %d hunted commons, %d are at the Counter and %d have an offer waiting, want some of each", hunts, counter, offers)
	}
}

func countHunts(hunted []play.Hunted) (atCounter, withOffers int) {
	for _, h := range hunted {
		if h.AtCounter {
			atCounter++
		}
		if h.Offers > 0 {
			withOffers++
		}
	}
	return atCounter, withOffers
}
