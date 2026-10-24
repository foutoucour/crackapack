package main

import (
	"log/slog"
	"testing"

	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// A new game has the quests of the configuration: three daily and one weekly, drawn from what a
// player who owns nothing yet can do.
func TestStartGame_theDemoHasItsDailyAndWeeklyQuests(t *testing.T) {
	t.Parallel()
	a := demoGameArgs(t)
	a.now = playtest.NewClock().Now
	game, _, err := startGame(a, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}

	board, err := game.Quests()
	if err != nil {
		t.Fatalf("Quests: %v", err)
	}

	if len(board.Daily) != 3 || len(board.Weekly) != 1 {
		t.Errorf("board has %d daily and %d weekly quests, want 3 and 1", len(board.Daily), len(board.Weekly))
	}
}
