package domain

import (
	"errors"
	"fmt"
	"math/rand/v2"
)

// SupplyTick is what a CounterSupply opened in one tick.
type SupplyTick struct {
	BoostersOpened int
	CardsOpened    int
}

// CounterSupply opens boosters on behalf of the Counter, which then behaves as a special player:
// the cards it draws go on its shelf, subject to its ShelfLife, and it sells them like any other
// stock. It draws on a random stream of its own, so adding it to a run never moves what the
// player or the NPCs draw.
type CounterSupply struct {
	counter *Counter
	sets    []SetCatalogue
	opener  *Opener
	rng     *rand.Rand

	// perDay is the boosters opened each day. credit accumulates it a tick at a time, so that a
	// day's openings match perDay exactly, as for an NPC.
	perDay int
	credit int
}

// NewFixedCounterSupply returns a supply that opens boostersPerDay boosters a day, spread over the
// day and with no energy, from sets chosen uniformly at random. rng is the stream the supply
// draws set choices from, and should be the one opener draws from too.
//
// It returns an error when counter, opener or rng is nil, when sets is empty, or when
// boostersPerDay is not positive.
func NewFixedCounterSupply(counter *Counter, sets []SetCatalogue, opener *Opener, rng *rand.Rand, boostersPerDay int) (*CounterSupply, error) {
	switch {
	case counter == nil:
		return nil, errors.New("counter supply: counter must not be nil")
	case opener == nil:
		return nil, errors.New("counter supply: opener must not be nil")
	case rng == nil:
		return nil, errors.New("counter supply: rng must not be nil")
	case len(sets) == 0:
		return nil, errors.New("counter supply: at least one set is required")
	case boostersPerDay <= 0:
		return nil, fmt.Errorf("counter supply: boosters per day must be > 0, got %d", boostersPerDay)
	}
	return &CounterSupply{counter: counter, sets: sets, opener: opener, rng: rng, perDay: boostersPerDay}, nil
}

// Tick advances the supply by one tick and opens the boosters it has come due for.
func (s *CounterSupply) Tick() (SupplyTick, error) {
	var total SupplyTick
	s.credit += s.perDay
	for s.credit >= TicksPerDay {
		s.credit -= TicksPerDay
		cards, err := s.openBooster()
		if err != nil {
			return total, err
		}
		total.BoostersOpened++
		total.CardsOpened += cards
	}
	return total, nil
}

// openBooster opens one booster of a set chosen uniformly and shelves every card it holds.
func (s *CounterSupply) openBooster() (int, error) {
	cat := s.sets[s.rng.IntN(len(s.sets))]
	booster, err := s.opener.Open(BoosterLayoutClassic, cat)
	if err != nil {
		return 0, fmt.Errorf("counter supply: open booster of %q: %w", cat.Code, err)
	}
	for _, key := range booster.Cards {
		if err := s.counter.AcceptSurplus(key, 1); err != nil {
			return 0, fmt.Errorf("counter supply: shelve %q: %w", key.ID, err)
		}
	}
	return len(booster.Cards), nil
}
