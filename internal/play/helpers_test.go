package play_test

import (
	"math/rand/v2"
	"sync"
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

// memStore is a [play.Store] that keeps the latest save in memory. It is safe for concurrent use.
type memStore struct {
	mu    sync.Mutex
	snap  play.Snapshot
	found bool
}

func (s *memStore) Load() (play.Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap, s.found, nil
}

func (s *memStore) Save(snap play.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap, s.found = snap, true
	return nil
}

// last is the latest save, which the test fails without.
func (s *memStore) last(t *testing.T) play.Snapshot {
	t.Helper()
	snap, found, _ := s.Load()
	if !found {
		t.Fatal("nothing was saved")
	}
	return snap
}

// copied is a store that holds what s holds, for a second game to load.
func (s *memStore) copied() *memStore {
	snap, found, _ := s.Load()
	return &memStore{snap: snap, found: found}
}

// newOpenerOn is an opener of the classic layout that draws from stream.
func newOpenerOn(stream *play.Stream) *domain.Opener {
	return domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rand.New(stream))
}
