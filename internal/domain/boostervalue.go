package domain

import (
	"fmt"
	"math/big"
	"sort"
)

// CostTier maps an upper ratio threshold (in whole percent) to an energy cost.
// Tiers must be strictly ascending by Threshold. The last entry is the catch-all
// for ratios above the highest threshold.
//
// A ratio of 100 means the booster value equals the catalogue median (×1).
// A threshold of 50 means ≤ 0.5× the median, 200 means ≤ 2×, and so on.
type CostTier struct {
	Threshold Percent // upper bound (inclusive), e.g. 50 = "≤ 0.5× median"
	Cost      int     // energy units spent to open one booster at this tier
}

// BoosterAverageValue returns the expected value in USDCents of one booster drawn
// from cat, averaged over cat.BoosterOptions weighted by BoosterOption.Weight.
//
// For each option, the option value is the sum over its Contents of
// count × sheetMeanPrice, where sheetMeanPrice is the weight-averaged card price
// on that sheet (sum of weight_i × price_i, divided by sum of weights). Cards absent
// from prices use fallback. Sheets within each option are visited in sorted name order
// to guarantee deterministic results regardless of map iteration order.
//
// The option values are then averaged, weighted by BoosterOption.Weight. All
// arithmetic uses math/big.Rat for exactness; the result is floored (truncated toward
// zero for positive values) to USDCents exactly once at the end.
//
// Errors are returned when:
//   - cat has no BoosterOptions
//   - the sum of option weights is zero
//   - a sheet named in an option's Contents is absent from cat.Sheets
//   - a referenced sheet has no cards or its total weight is zero
func BoosterAverageValue(cat SetCatalogue, prices PriceSnapshot, fallback USDCents) (USDCents, error) {
	if len(cat.BoosterOptions) == 0 {
		return 0, fmt.Errorf("booster value: catalogue %q has no booster options", cat.Code)
	}

	totalOptWeight := 0
	for _, opt := range cat.BoosterOptions {
		totalOptWeight += opt.Weight
	}
	if totalOptWeight == 0 {
		return 0, fmt.Errorf("booster value: catalogue %q: sum of option weights is zero", cat.Code)
	}

	// weightedSum accumulates opt.Weight × optionValue for each option.
	weightedSum := new(big.Rat)

	for _, opt := range cat.BoosterOptions {
		if opt.Weight == 0 {
			continue
		}

		optionValue := new(big.Rat)

		// Visit Contents in sorted sheet-name order for determinism.
		sheetNames := make([]string, 0, len(opt.Contents))
		for name := range opt.Contents {
			sheetNames = append(sheetNames, name)
		}
		sort.Strings(sheetNames)

		for _, name := range sheetNames {
			count := opt.Contents[name]
			sheet, ok := cat.Sheets[name]
			if !ok {
				return 0, fmt.Errorf("booster value: catalogue %q: sheet %q referenced in option but not in catalogue", cat.Code, name)
			}
			if len(sheet.Cards) == 0 {
				return 0, fmt.Errorf("booster value: catalogue %q: sheet %q has no cards", cat.Code, name)
			}

			// Compute weight sum directly from cards (not the TotalWeight field).
			weightSum := 0
			for _, sc := range sheet.Cards {
				weightSum += sc.Weight
			}
			if weightSum == 0 {
				return 0, fmt.Errorf("booster value: catalogue %q: sheet %q has zero total weight", cat.Code, name)
			}

			// sheetMeanPrice = Σ(weight_i × price_i) / weightSum
			sheetMean := new(big.Rat)
			for _, sc := range sheet.Cards {
				price, ok := prices[sc.ID]
				if !ok {
					price = fallback
				}
				// term = (weight × price) / weightSum
				term := new(big.Rat).SetFrac(
					big.NewInt(int64(sc.Weight)*int64(price)),
					big.NewInt(int64(weightSum)),
				)
				sheetMean.Add(sheetMean, term)
			}

			// optionValue += count × sheetMeanPrice
			countR := new(big.Rat).SetInt64(int64(count))
			optionValue.Add(optionValue, new(big.Rat).Mul(countR, sheetMean))
		}

		// weightedSum += opt.Weight × optionValue
		weightR := new(big.Rat).SetInt64(int64(opt.Weight))
		weightedSum.Add(weightedSum, new(big.Rat).Mul(weightR, optionValue))
	}

	// Divide by total option weight.
	result := new(big.Rat).Quo(weightedSum, new(big.Rat).SetInt64(int64(totalOptWeight)))

	// Floor: Quo on big.Int truncates toward zero; correct for non-negative results.
	var q big.Int
	q.Quo(result.Num(), result.Denom())
	return USDCents(q.Int64()), nil
}

// MedianValue returns the lower median of values. The lower median is defined as
// the element at index (n-1)/2 of the sorted slice:
//   - odd length: the unique middle element
//   - even length: the lower of the two middle elements
//
// MedianValue does not modify the caller's slice.
// Returns an error on empty input.
func MedianValue(values []USDCents) (USDCents, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("median: empty input")
	}
	sorted := make([]USDCents, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[(len(sorted)-1)/2], nil
}

// EnergyCost maps boosterValue relative to median through tiers using integer percent
// arithmetic:
//
//	ratio = boosterValue × 100 / median  (floor division)
//
// It returns the Cost of the first tier where ratio <= tier.Threshold.
// If no tier matches (ratio exceeds all thresholds) it returns the last tier's Cost,
// treating it as a catch-all.
//
// tiers must be non-empty and strictly ascending by Threshold. EnergyCost validates
// this and returns an error if the invariant is violated — an unsorted table would
// silently produce the wrong cost, so the domain enforces the order rather than
// relying solely on config validation.
//
// Errors: median == 0, tiers empty, tiers not strictly ascending by Threshold.
func EnergyCost(boosterValue, median USDCents, tiers []CostTier) (int, error) {
	if len(tiers) == 0 {
		return 0, fmt.Errorf("energy cost: tiers must not be empty")
	}
	if median == 0 {
		return 0, fmt.Errorf("energy cost: median must not be zero")
	}
	for i := 1; i < len(tiers); i++ {
		if tiers[i].Threshold <= tiers[i-1].Threshold {
			return 0, fmt.Errorf(
				"energy cost: tiers must be strictly ascending by Threshold: entry %d (threshold %d) <= entry %d (threshold %d)",
				i, tiers[i].Threshold, i-1, tiers[i-1].Threshold,
			)
		}
	}

	ratio := Percent(int64(boosterValue) * 100 / int64(median))
	for _, t := range tiers {
		if ratio <= t.Threshold {
			return t.Cost, nil
		}
	}
	// ratio exceeds all thresholds: the last tier is the catch-all.
	return tiers[len(tiers)-1].Cost, nil
}
