package sim

import (
	"errors"
	"sort"

	"github.com/foutoucour/crackapack/internal/domain"
)

// collectorReserveDays is the number of daily rewards the CollectorPolicy keeps
// as a coin floor. The collector never drops its balance below this floor when
// buying cards from the Counter.
//
// PACING ASSUMPTION: setting this to 1 day's reward keeps the simulation results
// sensitive to the daily reward value. Changing this constant shifts when cards
// are purchased relative to daily income; calibration results depend on it.
const collectorReserveDays = 1

// CollectorPolicy is a PlayerPolicy that hunts for complete binders.
//
// Per session, in order:
//  1. Open boosters: while energy is sufficient, open boosters for the set with
//     the lowest commons+uncommons progress ratio. Tie-break: smallest SetCode.
//     State is re-read via Session.View() after each booster, so the target set
//     is re-evaluated as progress updates.
//  2. Sell duplicates: sell all copies above a playset (keep 4) to the Counter.
//     Sub-coin refusals are counted but the card is restored to the collection.
//  3. Buy missing cards: buy the cheapest non-foil, non-basic-land cards the
//     player does not yet own, never dropping the balance below the coin floor
//     (DailyReward × collectorReserveDays). Skips out-of-stock cards.
type CollectorPolicy struct{}

// NewCollectorPolicy returns a CollectorPolicy ready to use.
func NewCollectorPolicy() CollectorPolicy {
	return CollectorPolicy{}
}

// RunSession implements PlayerPolicy.
func (CollectorPolicy) RunSession(s Session) {
	// Phase 1: open boosters.
	// Re-read View() after each booster so progress is always current.
	for {
		v := s.View()
		target, ok := pickTargetSet(v)
		if !ok {
			break
		}
		cost := v.BoosterCosts[target]
		if v.EnergyTotal < cost {
			break
		}
		if err := s.OpenBooster(target); err != nil {
			break
		}
	}

	// Phase 2: sell duplicates above playset.
	s.SellDuplicates(4) //nolint:errcheck // counts are reflected in day metrics via port

	// Phase 3: buy missing non-foil cards, cheapest first.
	v := s.View()
	floor := v.DailyReward * domain.Coins(collectorReserveDays)

	// Build candidates once: non-foil, non-basic-land, not yet owned.
	// All map iterations are sorted for determinism.
	candidates := buildBuyCandidates(v)

	for _, c := range candidates {
		if v.Balance-c.buyCoins < floor {
			break // remaining candidates are at least as expensive; stop
		}
		if err := s.BuyCard(c.key); err != nil {
			if errors.Is(err, domain.ErrOutOfStock) {
				continue // card not in stock; try next
			}
			// ErrInsufficientBalance or sub-coin: stop buying.
			break
		}
		// Refresh balance after successful purchase.
		v = s.View()
	}
}

// cuProgress extracts the commons+uncommons owned and total from a domain.Progress.
func cuProgress(prog domain.Progress) (owned, total int) {
	owned = prog.ByRarity[domain.RarityCommon].Owned + prog.ByRarity[domain.RarityUncommon].Owned
	total = prog.ByRarity[domain.RarityCommon].Total + prog.ByRarity[domain.RarityUncommon].Total
	return owned, total
}

// pickTargetSet returns the set code with the lowest (CUOwned / CUTotal) ratio.
// Sets with no commons+uncommons are skipped. Tie-break: smallest SetCode.
// Returns ("", false) when no valid target exists.
//
// Comparison uses cross-multiplication to avoid floating-point:
//
//	prog is lower when progOwned * bestTotal < bestOwned * progTotal
func pickTargetSet(v WorldView) (string, bool) {
	// Sort catalogues by SetCode for deterministic tie-breaking.
	codes := make([]string, 0, len(v.Catalogues))
	for _, cat := range v.Catalogues {
		codes = append(codes, cat.Code)
	}
	sort.Strings(codes)

	bestCode := ""
	bestOwned, bestTotal := -1, 1

	for _, code := range codes {
		prog, ok := v.SetProgress[code]
		if !ok {
			continue
		}
		cuOwned, cuTotal := cuProgress(prog)
		if cuTotal == 0 {
			continue
		}
		if bestCode == "" {
			bestCode = code
			bestOwned = cuOwned
			bestTotal = cuTotal
			continue
		}
		// cuOwned/cuTotal < bestOwned/bestTotal
		// ↔ cuOwned * bestTotal < bestOwned * cuTotal
		if cuOwned*bestTotal < bestOwned*cuTotal {
			bestCode = code
			bestOwned = cuOwned
			bestTotal = cuTotal
		}
	}
	return bestCode, bestCode != ""
}

// buyCandidate is a card the collector may purchase.
type buyCandidate struct {
	key      domain.CardKey
	buyCoins domain.Coins
}

// buildBuyCandidates returns the sorted list of non-foil, non-basic-land cards
// not yet owned, sorted by buy price ascending, tie-break by CardID ascending.
func buildBuyCandidates(v WorldView) []buyCandidate {
	// Count total owned per CardID (across all finishes) from the snapshot.
	// OwnedCards is keyed by CardKey; sum over both finishes.
	totalOwned := make(map[domain.CardID]int)
	for key, qty := range v.OwnedCards {
		totalOwned[key.ID] += qty
	}

	var candidates []buyCandidate
	seen := make(map[domain.CardID]struct{})

	// Visit catalogues in sorted order for determinism.
	codes := make([]string, 0, len(v.Catalogues))
	catByCode := make(map[string]domain.SetCatalogue, len(v.Catalogues))
	for _, cat := range v.Catalogues {
		codes = append(codes, cat.Code)
		catByCode[cat.Code] = cat
	}
	sort.Strings(codes)

	for _, code := range codes {
		cat := catByCode[code]
		ids := make([]domain.CardID, 0, len(cat.Cards))
		for id := range cat.Cards {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

		for _, id := range ids {
			card := cat.Cards[id]
			if card.IsBasicLand {
				continue
			}
			if !hasNonFoil(card.Finishes) {
				continue
			}
			if _, dup := seen[id]; dup {
				continue // same card in multiple catalogues; deduplicate
			}
			if totalOwned[id] > 0 {
				continue // already owned
			}
			seen[id] = struct{}{}

			price, ok := v.Prices[id]
			if !ok {
				price = v.FallbackPrice
			}
			buyCoins, err := domain.PriceToCoins(price, v.CoinRate, v.CounterBuyPct)
			if err != nil {
				continue // sub-coin card: skip
			}
			candidates = append(candidates, buyCandidate{
				key:      domain.CardKey{ID: id, Finish: domain.FinishNonFoil},
				buyCoins: buyCoins,
			})
		}
	}

	// Sort by buy price ascending; tie-break by CardID ascending.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].buyCoins != candidates[j].buyCoins {
			return candidates[i].buyCoins < candidates[j].buyCoins
		}
		return candidates[i].key.ID < candidates[j].key.ID
	})

	return candidates
}

// hasNonFoil reports whether FinishNonFoil is in finishes.
func hasNonFoil(finishes []domain.Finish) bool {
	for _, f := range finishes {
		if f == domain.FinishNonFoil {
			return true
		}
	}
	return false
}
