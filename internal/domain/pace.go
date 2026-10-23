package domain

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
)

// MaxBoostersToShare bounds what BoostersToShare returns: a binder that needs more boosters than
// this is out of reach for any player, and ranks as such.
const MaxBoostersToShare = 100_000

// halfACard is the expected number of missing cards under which a whole binder counts as reached:
// the chance that a card is still missing never drops to zero.
const halfACard = 0.5

// BoostersToShare is the number of boosters of binder after which a player who only opens them is
// expected to hold share of the binder: the fewest n for which the expected number of binder cards
// that n boosters have not given yet is at most (1-share) of the binder, or half a card when that
// is less. binder is the catalogue a binder counts, such as WithPools returns, and basic lands do
// not count. share is in (0, 1].
//
// The chance that a booster gives a card follows the booster options and the sheet weights. A
// sheet draws its cards as if it put each one back, which overstates the repeats of a booster a
// little, the same for every set. The result is at most MaxBoostersToShare. It is the measure the
// stages of a game rank the sets by, see
// docs/adr/0053-the-stages-rank-the-sets-by-the-energy-to-the-unlock-threshold.md.
func BoostersToShare(binder SetCatalogue, share float64) (int, error) {
	if share <= 0 || share > 1 {
		return 0, fmt.Errorf("boosters to share: share must be in (0, 1], got %v", share)
	}
	missing, err := missChances(binder)
	if err != nil {
		return 0, fmt.Errorf("boosters to share of %q: %w", binder.Code, err)
	}
	bar := max((1-share)*float64(len(missing)), halfACard)
	expectedMissing := func(n int) float64 {
		sum := 0.0
		for _, q := range missing {
			sum += math.Pow(q, float64(n))
		}
		return sum
	}
	if expectedMissing(MaxBoostersToShare) > bar {
		return MaxBoostersToShare, nil
	}
	// The expected count of missing cards only falls as boosters are opened.
	return sort.Search(MaxBoostersToShare, func(n int) bool { return expectedMissing(n) <= bar }), nil
}

// missChances returns, for each card the binder counts, the chance that one booster does not give
// it, in card ID order so that the sums over them do not depend on the map's order.
func missChances(binder SetCatalogue) ([]float64, error) {
	if len(binder.BoosterOptions) == 0 {
		return nil, errors.New("no booster options")
	}
	totalWeight := 0
	for _, opt := range binder.BoosterOptions {
		totalWeight += opt.Weight
	}
	if totalWeight <= 0 {
		return nil, errors.New("the booster options weigh nothing")
	}
	// miss is the chance that a booster does not give a card, summed over the options that give it.
	// A card that an option does not draw is missed by it for sure, which the sum adds at the end.
	miss := make(map[CardID]float64)
	drawnBy := make(map[CardID]float64)
	for _, opt := range binder.BoosterOptions {
		if opt.Weight == 0 {
			continue
		}
		optShare := float64(opt.Weight) / float64(totalWeight)
		logMiss, err := optionLogMiss(binder, opt)
		if err != nil {
			return nil, err
		}
		for id, lm := range logMiss {
			miss[id] += optShare * math.Exp(lm)
			drawnBy[id] += optShare
		}
	}
	ids := make([]CardID, 0, len(binder.Cards))
	for id, card := range binder.Cards {
		if !card.IsBasicLand {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	out := make([]float64, len(ids))
	for i, id := range ids {
		out[i] = miss[id] + (1 - drawnBy[id])
	}
	return out, nil
}

// optionLogMiss returns, for each card that opt draws, the log of the chance that a booster of opt
// does not give it.
func optionLogMiss(binder SetCatalogue, opt BoosterOption) (map[CardID]float64, error) {
	logMiss := make(map[CardID]float64)
	// In name order, so that a card on two sheets sums its logs in the same order on every call.
	for _, name := range slices.Sorted(maps.Keys(opt.Contents)) {
		count := opt.Contents[name]
		sheet, ok := binder.Sheets[name]
		if !ok {
			return nil, fmt.Errorf("sheet %q is in a booster option and not in the catalogue", name)
		}
		weight := 0
		for _, sc := range sheet.Cards {
			weight += sc.Weight
		}
		if weight <= 0 {
			return nil, fmt.Errorf("sheet %q weighs nothing", name)
		}
		for _, sc := range sheet.Cards {
			logMiss[sc.ID] += float64(count) * math.Log1p(-float64(sc.Weight)/float64(weight))
		}
	}
	return logMiss, nil
}
