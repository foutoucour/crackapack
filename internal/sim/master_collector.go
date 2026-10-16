package sim

import (
	"errors"
	"sort"

	"github.com/foutoucour/crackapack/internal/domain"
)

// MasterCollectorPolicy is a CollectorPolicy that also hunts for the foil of every card, the
// master set. It plays the collector's three phases first, so the non-foil cards keep their
// priority, then spends what is left above the coin floor on the foils it lacks, cheapest first.
// An out-of-stock foil is skipped.
type MasterCollectorPolicy struct{}

// NewMasterCollectorPolicy returns a MasterCollectorPolicy ready to use.
func NewMasterCollectorPolicy() MasterCollectorPolicy {
	return MasterCollectorPolicy{}
}

// RunSession implements PlayerPolicy.
func (MasterCollectorPolicy) RunSession(s Session) {
	NewCollectorPolicy().RunSession(s)

	v := s.View()
	floor := v.DailyReward * domain.Coins(collectorReserveDays)
	for _, c := range buildFoilCandidates(v) {
		if v.Balance-c.buyCoins < floor {
			return // the list is sorted ascending, so the rest costs at least as much
		}
		if err := s.BuyCard(c.key); err != nil {
			if errors.Is(err, domain.ErrOutOfStock) || errors.Is(err, domain.ErrSubCoin) {
				continue
			}
			return
		}
		v = s.View()
	}
}

// buildFoilCandidates returns the foils of the non-basic-land cards the player owns no foil copy
// of, sorted by buy price ascending, tie-break by CardID ascending.
func buildFoilCandidates(v WorldView) []buyCandidate {
	var candidates []buyCandidate
	seen := make(map[domain.CardID]struct{})
	for _, cat := range v.Catalogues {
		for id, card := range cat.Cards {
			key := domain.CardKey{ID: id, Finish: domain.FinishFoil}
			if card.IsBasicLand || !hasFinish(card.Finishes, domain.FinishFoil) || v.OwnedCards[key] > 0 {
				continue
			}
			if _, dup := seen[id]; dup {
				continue // same card in multiple catalogues
			}
			seen[id] = struct{}{}

			price, ok := v.Prices[id]
			if !ok {
				price = v.FallbackPrice
			}
			buyCoins, err := domain.PriceToCoins(domain.FoilPrice(price, v.CounterFoilPct), v.CoinRate, v.CounterBuyPct)
			if err != nil || buyCoins == 0 {
				continue // sub-coin or zero-priced foil
			}
			candidates = append(candidates, buyCandidate{key: key, buyCoins: buyCoins})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].buyCoins != candidates[j].buyCoins {
			return candidates[i].buyCoins < candidates[j].buyCoins
		}
		return candidates[i].key.ID < candidates[j].key.ID
	})
	return candidates
}

// hasFinish reports whether f is in finishes.
func hasFinish(finishes []domain.Finish, f domain.Finish) bool {
	for _, have := range finishes {
		if have == f {
			return true
		}
	}
	return false
}
