package domain

import (
	"errors"
	"fmt"
)

// EnergyCap is the dearest energy cost of the cost tiers, the cost a booster has once it passes the
// last tier.
func EnergyCap(tiers []CostTier) int {
	var dearest int
	for _, tier := range tiers {
		dearest = max(dearest, tier.Cost)
	}
	return dearest
}

// BoosterCoinFees returns the coin fee of one booster of each openable set, keyed by set code, for a
// price of p coins per energy point. A booster worth more than the cap's energy costs its cash-out
// value, what the Counter would pay the player for its cards, minus the cap times p. A set worth no
// more than that has no fee. With p at 0 no set has one, and the result is nil. avgValues is the
// average booster value of each openable set, in order.
//
// The player and the Counter share this rule, so one set costs both the same.
func BoosterCoinFees(p Coins, energyCap int, coinRate CoinRate, sellPct Percent, openable []SetCatalogue, avgValues []USDCents) (map[string]Coins, error) {
	if p <= 0 {
		return nil, nil
	}
	capValue := Coins(energyCap) * p
	fees := make(map[string]Coins, len(openable))
	for i, cat := range openable {
		value, err := PriceToCoins(avgValues[i], coinRate, sellPct)
		if err != nil && !errors.Is(err, ErrSubCoin) {
			return nil, fmt.Errorf("coin fee of %q: %w", cat.Code, err)
		}
		if value > capValue {
			fees[cat.Code] = value - capValue
		}
	}
	return fees, nil
}
