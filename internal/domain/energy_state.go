package domain

import "fmt"

// EnergyState is what an Energy holds at one moment, so that a save can keep it and Restore can
// bring it back.
type EnergyState struct {
	Reserve int
	Savings int
	// SavingsTick is the ticks that have gone by toward the next point of savings.
	SavingsTick int
}

// State returns the energy as it is.
func (e *Energy) State() EnergyState {
	return EnergyState{Reserve: e.reserve, Savings: e.savings, SavingsTick: e.savingsTick}
}

// Restore sets the energy to s. It returns an error, and changes nothing, when s does not fit the
// energy's parameters: a level below zero or above its cap, or a savings tick that is not within
// the interval.
func (e *Energy) Restore(s EnergyState) error {
	switch {
	case s.Reserve < 0 || s.Reserve > e.params.ReserveCap:
		return fmt.Errorf("energy: reserve %d is outside 0..%d", s.Reserve, e.params.ReserveCap)
	case s.Savings < 0 || s.Savings > e.params.SavingsCap:
		return fmt.Errorf("energy: savings %d is outside 0..%d", s.Savings, e.params.SavingsCap)
	case s.SavingsTick < 0 || s.SavingsTick >= e.params.SavingsIntervalTicks:
		return fmt.Errorf("energy: savings tick %d is outside 0..%d", s.SavingsTick, e.params.SavingsIntervalTicks-1)
	}
	e.reserve, e.savings, e.savingsTick = s.Reserve, s.Savings, s.SavingsTick
	return nil
}
