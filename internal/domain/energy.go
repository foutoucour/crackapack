package domain

import "fmt"

// EnergyParams holds the recharge parameters for the energy system.
// The composition root fills this from config.EnergyConfig; internal/domain
// never imports internal/config.
type EnergyParams struct {
	RechargePerTick      int // energy added to the reserve per tick; must be > 0
	ReserveCap           int // maximum reserve energy; must be > 0
	SavingsIntervalTicks int // ticks between savings recharge events; must be > 0
	SavingsCap           int // maximum savings energy; must be >= 0
}

// InsufficientEnergyError is returned by Spend when reserve+savings < amount.
// State is left unchanged when this error is returned.
type InsufficientEnergyError struct {
	Have int // total available energy at the time of the call
	Need int // amount requested
}

func (e *InsufficientEnergyError) Error() string {
	return fmt.Sprintf("energy: insufficient: need %d, have %d", e.Need, e.Have)
}

// InvalidSpendAmountError is returned by Spend when amount <= 0.
// State is left unchanged when this error is returned.
type InvalidSpendAmountError struct {
	Amount int
}

func (e *InvalidSpendAmountError) Error() string {
	return fmt.Sprintf("energy: spend amount must be > 0, got %d", e.Amount)
}

// Energy tracks a player's reserve and savings energy pools.
//
// Starting state: reserve is full (EnergyParams.ReserveCap), savings is 0,
// and the internal savings-interval counter is 0.
//
// Recharge (via Tick):
//   - While reserve < ReserveCap: +RechargePerTick per tick, capped at ReserveCap.
//     The savings-interval counter does NOT advance during this phase.
//   - While reserve == ReserveCap: the savings-interval counter increments each tick.
//     Every SavingsIntervalTicks ticks the counter reaches its target, savings gains +1
//     (capped at SavingsCap), and the counter resets to 0.
//
// Savings counter and spend-refill cycles: when Spend drops the reserve below cap the
// savings-interval counter retains its current value. It resumes advancing once the
// reserve is full again. This prevents losing partial progress from brief spending dips.
// For example, if the counter is at 2 out of 3 and the player spends one energy, the
// counter stays at 2 while the reserve refills, then reaches 3 on the next tick.
//
// Spending: reserve is consumed first, then savings. Returns *InsufficientEnergyError
// (state unchanged) when total is insufficient. Returns *InvalidSpendAmountError (state
// unchanged) when amount <= 0.
//
// Energy is never purchasable with coins: no such method exists on this type.
// This is intentional — energy sets the rhythm of play sessions and must not be
// bypassed through the coin economy. See docs/core-features.md §Energy.
type Energy struct {
	params      EnergyParams
	reserve     int
	savings     int
	savingsTick int // ticks elapsed toward the next savings recharge while reserve is full
}

// TicksToAfford returns how many ticks an empty Energy, with neither reserve nor savings, needs
// before it can pay cost. It is the longest wait a player faces for one purchase of that cost.
//
// Cost is paid from the reserve first, so the reserve must reach min(cost, ReserveCap). A cost
// above ReserveCap also needs savings, which only grow while the reserve is full, one point per
// SavingsIntervalTicks. It returns an error when cost is not positive or exceeds
// ReserveCap + SavingsCap, since such a cost can never be paid.
func (p EnergyParams) TicksToAfford(cost int) (int, error) {
	if cost <= 0 {
		return 0, &InvalidSpendAmountError{Amount: cost}
	}
	if cost > p.ReserveCap+p.SavingsCap {
		return 0, fmt.Errorf("energy: cost %d exceeds reserve cap %d plus savings cap %d", cost, p.ReserveCap, p.SavingsCap)
	}
	fromReserve := min(cost, p.ReserveCap)
	ticks := (fromReserve + p.RechargePerTick - 1) / p.RechargePerTick
	if cost > p.ReserveCap {
		ticks += (cost - p.ReserveCap) * p.SavingsIntervalTicks
	}
	return ticks, nil
}

// NewEnergy returns an Energy with a full reserve and empty savings.
// Returns an error naming the offending field when any parameter is invalid.
func NewEnergy(p EnergyParams) (*Energy, error) {
	if p.RechargePerTick <= 0 {
		return nil, fmt.Errorf("energy: RechargePerTick must be > 0, got %d", p.RechargePerTick)
	}
	if p.ReserveCap <= 0 {
		return nil, fmt.Errorf("energy: ReserveCap must be > 0, got %d", p.ReserveCap)
	}
	if p.SavingsIntervalTicks <= 0 {
		return nil, fmt.Errorf("energy: SavingsIntervalTicks must be > 0, got %d", p.SavingsIntervalTicks)
	}
	if p.SavingsCap < 0 {
		return nil, fmt.Errorf("energy: SavingsCap must be >= 0, got %d", p.SavingsCap)
	}
	return &Energy{
		params:  p,
		reserve: p.ReserveCap,
	}, nil
}

// Tick advances the energy state by n ticks. n == 0 is a valid no-op, needed when a
// catch-up simulation has zero elapsed ticks. Returns an error for n < 0 without
// modifying state.
func (e *Energy) Tick(n int) error {
	if n < 0 {
		return fmt.Errorf("energy: Tick count must be >= 0, got %d", n)
	}
	for range n {
		e.applyTick()
	}
	return nil
}

// applyTick applies one tick of recharge logic.
func (e *Energy) applyTick() {
	if e.reserve < e.params.ReserveCap {
		e.reserve += e.params.RechargePerTick
		if e.reserve > e.params.ReserveCap {
			e.reserve = e.params.ReserveCap
		}
		// The savings-interval counter does not advance while the reserve is recharging.
		return
	}
	// Reserve is full: advance the savings-interval counter.
	e.savingsTick++
	if e.savingsTick >= e.params.SavingsIntervalTicks {
		if e.savings < e.params.SavingsCap {
			e.savings++
		}
		e.savingsTick = 0
	}
}

// Spend deducts amount from the reserve first, then from savings.
// Returns *InvalidSpendAmountError when amount <= 0.
// Returns *InsufficientEnergyError when reserve+savings < amount; state is unchanged.
func (e *Energy) Spend(amount int) error {
	if amount <= 0 {
		return &InvalidSpendAmountError{Amount: amount}
	}
	total := e.reserve + e.savings
	if total < amount {
		return &InsufficientEnergyError{Have: total, Need: amount}
	}
	if e.reserve >= amount {
		e.reserve -= amount
		return nil
	}
	remainder := amount - e.reserve
	e.reserve = 0
	e.savings -= remainder
	return nil
}

// Reserve returns the current reserve energy level.
func (e *Energy) Reserve() int { return e.reserve }

// Savings returns the current savings energy level.
func (e *Energy) Savings() int { return e.savings }
