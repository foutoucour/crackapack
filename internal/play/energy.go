package play

import (
	"errors"
	"fmt"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// tickLength is the time one tick of energy takes, see docs/core-features.md#energy.
const tickLength = domain.MinutesPerTick * time.Minute

var (
	// ErrNotEnoughEnergy is returned by Open when the booster is not a welcome one and the player's
	// energy cannot pay for it, and by OpenWithTopUp when the player has less than the least energy a
	// top-up takes. The error carries a *NotEnoughEnergyError.
	ErrNotEnoughEnergy = errors.New("play: not enough energy")
	// ErrNoTopUp is returned by OpenWithTopUp in a game that offers no coin top-up.
	ErrNoTopUp = errors.New("play: the game offers no coin top-up")
)

// Rules are the numbers of the game that come from its configuration.
type Rules struct {
	// Energy is how the reserve and the savings recharge.
	Energy domain.EnergyParams
	// Costs is the energy a booster costs, by set code. Every set needs one above zero that the
	// reserve and the savings can pay.
	Costs map[string]int
	// Fees is the coins a booster costs on top of its energy, by set code, see
	// docs/adr/0068-the-game-charges-a-sets-coin-fee-when-a-booster-is-opened.md. A set missing
	// from it has none. The coins are the player's, so a game without a Counter cannot pay a fee.
	Fees map[string]domain.Coins
	// TopUp lets the player pay coins for the energy a booster lacks, see
	// docs/adr/0069-the-player-asks-for-a-coin-top-up-and-the-game-plays-one-price.md. Nil offers
	// none.
	TopUp *TopUpRules
	// Milestones is what completing a part of a binder pays, see [MilestoneRules]. Nil gives the
	// game no milestones.
	Milestones *MilestoneRules
}

// TopUpRules is how the player pays coins for missing energy: the booster takes the energy the
// player has, and the rest is paid for at Price a point.
type TopUpRules struct {
	// Price is the coins one point of energy costs, above zero.
	Price domain.Coins
	// MinEnergy is the least energy the player must have to top up, at least 1: a booster is never
	// opened with coins alone.
	MinEnergy int
}

// TopUp is what opening a booster with coins for its missing energy would cost now.
type TopUp struct {
	// Units is the energy the player lacks, and Coins what it costs, the booster's fee not included.
	Units int
	Coins domain.Coins
	// Total is what the opening costs in coins in all: the top-up and the booster's fee.
	Total domain.Coins
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
	// TopUp is true when Need is the least energy a coin top-up takes, not the booster's cost.
	TopUp bool
}

// NotEnoughCoinsError says what an opening cost in coins, its fee and its top-up, and what the
// player had.
type NotEnoughCoinsError struct {
	Need domain.Coins
	Have domain.Coins
}

func (e *NotEnoughCoinsError) Error() string {
	return fmt.Sprintf("play: not enough coins: the booster costs %d coins and the player has %d", e.Need, e.Have)
}

// Is makes errors.Is(err, ErrNotEnoughCoins) true for it.
func (e *NotEnoughCoinsError) Is(target error) bool { return target == ErrNotEnoughCoins }

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
