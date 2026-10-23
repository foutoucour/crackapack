// Package playtest holds what tests of a play session, and of the clients built on it, share: a clock
// they move by hand and rules that make the arithmetic easy to follow.
package playtest

import (
	"fmt"
	"sync"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// Cost is the energy that Rules gives a booster of every set.
const Cost = 5

// Tick is the time that one tick of energy takes.
const Tick = domain.MinutesPerTick * time.Minute

// Rules returns the rules of a session on sets: a reserve of 10 that recharges by 1 each tick,
// savings of up to 200 that grow by 1 every 3 ticks once the reserve is full, and Cost energy for a
// booster of any set. Two boosters empty the reserve.
func Rules(sets ...domain.SetCatalogue) play.Rules {
	costs := make(map[string]int, len(sets))
	for _, cat := range sets {
		costs[cat.Code] = Cost
	}
	return play.Rules{
		Energy: domain.EnergyParams{RechargePerTick: 1, ReserveCap: 10, SavingsIntervalTicks: 3, SavingsCap: 200},
		Costs:  costs,
	}
}

// Clock is a clock that only moves when it is told to. It is safe for concurrent use.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock returns a Clock set to a fixed date.
func NewClock() *Clock {
	return &Clock{now: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)}
}

// Now reads the clock. Its method value is a play.Clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock by d, which may be negative.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// FlakyStore is a [play.Store] that holds nothing: a game loaded from it is a new one. Its saves go
// through, and count, until Fail makes them fail. It is safe for concurrent use.
type FlakyStore struct {
	mu    sync.Mutex
	err   error
	saved int
}

// Fail makes the next saves return err, and the saves go through again for a nil one.
func (s *FlakyStore) Fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// Saved is the number of saves that went through.
func (s *FlakyStore) Saved() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saved
}

// Load finds no save.
func (s *FlakyStore) Load() (play.Snapshot, bool, error) { return play.Snapshot{}, false, nil }

// Save counts the save, or returns the error that Fail set.
func (s *FlakyStore) Save(play.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.saved++
	return nil
}

// TinySet is a set of n commons whose booster is perBooster of them, drawn without repeats, so a
// test knows what a booster does to the binder: n of n gives the whole binder, and 1 of 2 half of
// it. The set's name is "Set " and its code.
func TinySet(code string, n, perBooster int) domain.SetCatalogue {
	cat := domain.SetCatalogue{Code: code, Name: "Set " + code, Cards: make(map[domain.CardID]domain.Card, n)}
	sheet := domain.Sheet{Name: "common", TotalWeight: n}
	for i := range n {
		id := domain.CardID(fmt.Sprintf("%s-%02d", code, i))
		cat.Cards[id] = domain.Card{ID: id, Name: string(id), Number: fmt.Sprint(i + 1), Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}}
		sheet.Cards = append(sheet.Cards, domain.SheetCard{ID: id, Weight: 1})
	}
	cat.Sheets = map[string]domain.Sheet{"common": sheet}
	cat.BoosterOptions = []domain.BoosterOption{{Contents: map[string]int{"common": perBooster}, Weight: 1}}
	return cat
}
