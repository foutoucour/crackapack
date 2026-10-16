// Package playtest holds what tests of a play session, and of the clients built on it, share: a clock
// they move by hand and rules that make the arithmetic easy to follow.
package playtest

import (
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
