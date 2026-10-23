package domain

import "fmt"

// SupplyState is where a CounterSupply stands in its day, so that a save can keep it and Restore
// can bring it back. Its random stream is the caller's, and is saved apart.
type SupplyState struct {
	// Credit is the fixed supply's progress toward its next booster, in ticks of a day.
	Credit int
	// Energy is the energy supply's energy.
	Energy EnergyState
	// Next is the code of the set the energy supply picked and is saving for, empty when none.
	Next string
}

// State returns where the supply stands.
func (s *CounterSupply) State() SupplyState {
	state := SupplyState{Credit: s.credit}
	if s.energy != nil {
		state.Energy = s.energy.State()
		if s.next >= 0 {
			state.Next = s.sets[s.next].Code
		}
	}
	return state
}

// Restore sets the supply to st, which State returned. It returns an error, and changes nothing,
// when st cannot be the state of this supply: a credit outside a day, a picked set it does not
// open, or an energy that does not fit its energy, see Energy.Restore.
func (s *CounterSupply) Restore(st SupplyState) error {
	if st.Credit < 0 || st.Credit >= TicksPerDay {
		return fmt.Errorf("counter supply: credit %d is outside 0..%d", st.Credit, TicksPerDay-1)
	}
	if s.energy == nil {
		s.credit = st.Credit
		return nil
	}
	next := -1
	if st.Next != "" {
		for i, cat := range s.sets {
			if cat.Code == st.Next {
				next = i
			}
		}
		if next < 0 {
			return fmt.Errorf("counter supply: the set picked, %q, is not one the supply opens", st.Next)
		}
	}
	if err := s.energy.Restore(st.Energy); err != nil {
		return fmt.Errorf("counter supply: %w", err)
	}
	s.credit, s.next = st.Credit, next
	return nil
}
