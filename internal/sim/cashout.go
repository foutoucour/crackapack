package sim

// cashOutKeep is the number of copies of each card the CashOutPolicy retains.
// All copies above this number are sold to the Counter each session.
const cashOutKeep = 1

// CashOutPolicy is a PlayerPolicy that measures how many coins the Counter
// creates. It opens boosters with all available energy, sells everything above
// one copy, and never buys cards from the Counter.
//
// Per session, in order:
//  1. Open boosters: while energy is sufficient, open boosters for the set with
//     the lowest commons+uncommons progress ratio. Tie-break: smallest SetCode.
//     State is re-read via Session.View() after each booster so the target set
//     is re-evaluated as progress updates. Same rule as CollectorPolicy.
//  2. Sell duplicates: sell all copies above cashOutKeep (1) to the Counter.
//  3. Never buy cards.
type CashOutPolicy struct{}

// NewCashOutPolicy returns a CashOutPolicy ready to use.
func NewCashOutPolicy() CashOutPolicy {
	return CashOutPolicy{}
}

// RunSession implements PlayerPolicy.
func (CashOutPolicy) RunSession(s Session) {
	// Phase 1: open boosters.
	// Re-read View() after each booster so progress and energy are always current.
	// Uses the same pickTargetSet helper as CollectorPolicy.
	for {
		v := s.View()
		target, ok := pickTargetSet(v)
		if !ok {
			break
		}
		if v.EnergyTotal < v.BoosterCosts[target] {
			break
		}
		if err := s.OpenBooster(target); err != nil {
			break
		}
	}

	// Phase 2: sell all copies above one.
	s.SellDuplicates(cashOutKeep)

	// Phase 3: no buying.
}
