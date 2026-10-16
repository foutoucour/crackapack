package play

import (
	"errors"
	"fmt"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// tickLength is the time one tick of energy takes, see docs/core-features.md#energy.
const tickLength = domain.MinutesPerTick * time.Minute

// ErrNotEnoughEnergy is returned by Open when the booster is not a welcome one and the player's
// energy cannot pay for it. The error carries a *NotEnoughEnergyError.
var ErrNotEnoughEnergy = errors.New("play: not enough energy")

// Rules are the numbers of the game that come from its configuration.
type Rules struct {
	// Energy is how the reserve and the savings recharge.
	Energy domain.EnergyParams
	// Costs is the energy a booster costs, by set code. Every set needs one above zero that the
	// reserve and the savings can pay.
	Costs map[string]int
}

// Clock tells the time. A session reads it to know how much energy has come back, so a test, or
// the real clock, is injected rather than read from the domain.
type Clock func() time.Time

// NotEnoughEnergyError says what an opening lacked and how long the player has to wait for it.
type NotEnoughEnergyError struct {
	Need int
	Have int
	// Wait is the time until the energy has come back to Need.
	Wait time.Duration
}

func (e *NotEnoughEnergyError) Error() string {
	return fmt.Sprintf("play: not enough energy: the booster costs %d, the player has %d, and %d more is %v away", e.Need, e.Have, e.Need-e.Have, e.Wait)
}

// Is makes errors.Is(err, ErrNotEnoughEnergy) true for it.
func (e *NotEnoughEnergyError) Is(target error) bool { return target == ErrNotEnoughEnergy }

// EnergyStatus is the player's energy at one moment.
type EnergyStatus struct {
	Reserve    int
	ReserveCap int
	Savings    int
	SavingsCap int
	// NextIn is the time until the energy next grows by a point, whether in the reserve or in the
	// savings. It is 0 when both are full.
	NextIn time.Duration
}

// Energy returns the player's energy, brought up to date.
func (g *Game) Energy() EnergyStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.sync()
	status := EnergyStatus{
		Reserve:    g.energy.Reserve(),
		ReserveCap: g.rules.Energy.ReserveCap,
		Savings:    g.energy.Savings(),
		SavingsCap: g.rules.Energy.SavingsCap,
	}
	// The total grows by a point at the next tick of the reserve or the next point of savings, so the
	// wait for one more is the wait for either. Past both caps there is nothing to wait for.
	status.NextIn = g.waitFor(status.Reserve+status.Savings+1, now)
	return status
}

// sync brings the energy up to date with the clock and returns the instant it read. The ticks that
// have gone by are applied, and the part of a tick that has not is kept for the next call. A clock
// that went back changes nothing, and the recharge resumes once the clock has caught up. The real
// clock cannot go back, since time.Now carries a monotonic reading. g.mu must be held.
func (g *Game) sync() time.Time {
	now := g.now()
	ticks := int(now.Sub(g.lastTick) / tickLength)
	if ticks <= 0 {
		return now
	}
	// Tick fails only for a negative count, and ticks is positive.
	_ = g.energy.Tick(ticks)
	g.lastTick = g.lastTick.Add(time.Duration(ticks) * tickLength)
	return now
}

// waitFor is the time, from now, until the energy can pay cost: 0 when it can already, or when it
// never will. g.mu must be held, and now must be the instant sync returned.
func (g *Game) waitFor(cost int, now time.Time) time.Duration {
	ticks, err := g.energy.TicksUntil(cost)
	if err != nil || ticks == 0 {
		return 0
	}
	return time.Duration(ticks)*tickLength - now.Sub(g.lastTick)
}
