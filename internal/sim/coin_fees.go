package sim

import (
	"errors"
	"fmt"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// energyCap is the dearest energy cost of the cost table, the cost a booster has once it passes the
// last tier.
func energyCap(table []config.EnergyCost) int {
	var dearest int
	for _, tier := range table {
		dearest = max(dearest, tier.Cost)
	}
	return dearest
}

// coinFees returns the coin fee of one booster of each openable set, keyed by set code, for a price
// of p coins per energy point. A booster worth more than the cap's energy costs its cash-out value,
// what the Counter would pay the player for its cards, minus the cap times p. A set worth no more
// than that has no fee. With p at 0 no set has one, and the result is nil.
//
// The player and the Counter share this rule, so one set costs both the same.
func coinFees(p domain.Coins, cap int, coinRate domain.CoinRate, sellPct domain.Percent, openable []domain.SetCatalogue, avgValues []domain.USDCents) (map[string]domain.Coins, error) {
	if p <= 0 {
		return nil, nil
	}
	capValue := domain.Coins(cap) * p
	fees := make(map[string]domain.Coins, len(openable))
	for i, cat := range openable {
		value, err := domain.PriceToCoins(avgValues[i], coinRate, sellPct)
		if err != nil && !errors.Is(err, domain.ErrSubCoin) {
			return nil, fmt.Errorf("sim: coin fee of %q: %w", cat.Code, err)
		}
		if value > capValue {
			fees[cat.Code] = value - capValue
		}
	}
	return fees, nil
}
