package sim

import (
	"errors"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ConvertingCollectorPolicy is a CollectorPolicy that also uses coin top-up to open boosters
// when the player is short on energy.
//
// It adds two behaviours on top of CollectorPolicy:
//
//  1. Coin top-up in Phase 1: when the player lacks energy for the next booster but has at
//     least the configured minimum real energy (min_energy_units), it calls OpenBoosterTopUp
//     to pay coins for the shortfall. It only does so when the remaining balance after
//     paying the top-up and the mandatory fee stays above the coin floor (daily reward buffer
//     plus the smallest mandatory fee), so it never starves itself of the fee buffer.
//
//  2. Raised coin floor in Phase 3 (buy cards): the floor includes the smallest mandatory
//     booster fee in the run. This ensures the player accumulates and retains enough coins to
//     open at least one booster, fixing the stall that the plain CollectorPolicy suffers when
//     coin fees are large relative to daily income.
//
// When top-up is disabled (CoinTopupEnabled == false) the policy behaves identically to
// CollectorPolicy: it opens boosters only with energy and keeps the standard coin floor.
//
// The energy-first variant, see NewEnergyFirstPolicy, also keeps the coins of the next top-up out
// of the card purchases in Phase 3. The plain policy spends every coin above its floor on cards
// each session, so at a low income its balance never reaches the price of a top-up.
type ConvertingCollectorPolicy struct {
	energyFirst bool
}

// NewConvertingCollectorPolicy returns a ConvertingCollectorPolicy ready to use.
func NewConvertingCollectorPolicy() ConvertingCollectorPolicy {
	return ConvertingCollectorPolicy{}
}

// NewEnergyFirstPolicy returns a ConvertingCollectorPolicy that saves for the next top-up before it
// buys cards, so energy and cards compete for the same coins.
func NewEnergyFirstPolicy() ConvertingCollectorPolicy {
	return ConvertingCollectorPolicy{energyFirst: true}
}

// RunSession implements PlayerPolicy.
func (p ConvertingCollectorPolicy) RunSession(s Session) {
	// Phase 1: open boosters. Try a normal open first; fall back to coin top-up when energy is
	// short and the balance permits.
	for {
		v := s.View()
		target, ok := pickTargetSet(v)
		if !ok {
			break
		}
		cost := v.BoosterCosts[target]

		if v.EnergyTotal >= cost {
			// Enough energy: open normally.
			if err := s.OpenBooster(target); err != nil {
				break
			}
			continue
		}

		// Energy shortfall: try coin top-up when enabled and minimum energy is satisfied.
		if !v.CoinTopupEnabled || v.EnergyTotal < v.CoinTopupMinEnergyUnits {
			break // not enough real energy, or top-up is off
		}

		missingUnits := cost - v.EnergyTotal
		topUpCost := topUpCostFromView(v, missingUnits)
		fee := v.BoosterFees[target] // mandatory fee (may be 0)

		// Keep enough coins: daily reward buffer plus the smallest mandatory fee (to avoid spending
		// the fee buffer on top-up payments).
		floor := v.DailyReward*domain.Coins(collectorReserveDays) + minimumBoosterFee(v)
		if v.Balance-topUpCost-fee < floor {
			break // can't afford top-up while keeping the reserve
		}

		if err := s.OpenBoosterTopUp(target); err != nil {
			break
		}
	}

	// Phase 2: sell duplicates above playset.
	s.SellDuplicates(domain.PlaysetSize)

	// Phase 3: buy missing cards, cheapest first.
	// Raised floor: keep enough for the mandatory fee on the cheapest available set, so the fee
	// buffer built up in Phase 1 is not spent on card purchases.
	v := s.View()
	minFee := minimumBoosterFee(v)
	floor := v.DailyReward*domain.Coins(collectorReserveDays) + minFee
	if p.energyFirst {
		floor += topUpReserve(v)
	}

	candidates := buildBuyCandidates(v)
	for _, c := range candidates {
		if v.Balance-c.buyCoins < floor {
			break
		}
		if err := s.BuyCard(c.key); err != nil {
			if errors.Is(err, domain.ErrOutOfStock) || errors.Is(err, domain.ErrSubCoin) {
				continue
			}
			break
		}
		v = s.View()
	}
}

// topUpCostFromView computes the total coin cost to buy n energy units via top-up,
// given the current CoinTopupRateNow and CoinTopupEscalationStep from the view.
// For the min_energy variant step is 0, so all units cost the same rate.
func topUpCostFromView(v WorldView, n int) domain.Coins {
	var total domain.Coins
	for i := range n {
		rate := v.CoinTopupRateNow + v.CoinTopupEscalationStep*domain.Coins(i)
		total += rate
	}
	return total
}

// topUpReserve returns the coins the energy-first policy keeps back from card purchases: the top-up
// of a whole booster of the set it would open next, plus that set's mandatory fee. The player
// always brings the minimum real energy, so the top-up covers the cost minus that minimum. It is
// zero when top-up is off or no set can be opened.
func topUpReserve(v WorldView) domain.Coins {
	if !v.CoinTopupEnabled {
		return 0
	}
	target, ok := pickTargetSet(v)
	if !ok {
		return 0
	}
	missingUnits := max(v.BoosterCosts[target]-v.CoinTopupMinEnergyUnits, 0)
	return topUpCostFromView(v, missingUnits) + v.BoosterFees[target]
}

// minimumBoosterFee returns the smallest non-zero mandatory booster fee across all sets in v,
// or 0 when no set has a fee.
func minimumBoosterFee(v WorldView) domain.Coins {
	var min domain.Coins
	for _, fee := range v.BoosterFees {
		if fee > 0 && (min == 0 || fee < min) {
			min = fee
		}
	}
	return min
}
