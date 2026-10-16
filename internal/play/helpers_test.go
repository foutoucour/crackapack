package play_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// newGameWithClock starts a session on sets with the standard test rules and a clock the test moves.
func newGameWithClock(t *testing.T, sets ...domain.SetCatalogue) (*play.Game, *playtest.Clock) {
	t.Helper()
	clock := playtest.NewClock()
	g, err := play.NewGame(sets, nil, newOpener(), playtest.Rules(sets...), clock.Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	return g, clock
}

// newPooledGame starts a session on set whose boosters draw cards from pool.
func newPooledGame(t *testing.T, set, pool domain.SetCatalogue) *play.Game {
	t.Helper()
	g, err := play.NewGame([]domain.SetCatalogue{set}, []domain.SetCatalogue{pool}, newOpener(), playtest.Rules(set), playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	return g
}
