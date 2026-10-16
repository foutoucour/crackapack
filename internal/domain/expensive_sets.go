package domain

// ExpensiveBoosterValue is the booster value, in USDCents, from which a real set belongs to the
// expensive bundle: $60. The 12-energy cap is flat, so a booster this dear pays out far more coins
// per unit of energy than the 2005-2007 sets and breaks the economy. See the "Expensive old sets
// and the 12-energy cap" section of docs/specs-review.md.
const ExpensiveBoosterValue USDCents = 6000

// SplitByBoosterValue sorts cats into two bundles by the average value of one booster at prices,
// priced at fallback for a card the snapshot lacks. A set worth limit or more goes to the expensive
// bundle, every other set to the common one. Each bundle keeps the order of cats. The caller picks
// the limit: real data uses ExpensiveBoosterValue, synthetic data its own.
//
// It fails when the value of a set cannot be computed, see BoosterAverageValue.
func SplitByBoosterValue(cats []SetCatalogue, prices PriceSnapshot, fallback, limit USDCents) (common, expensive []SetCatalogue, err error) {
	for _, cat := range cats {
		value, err := BoosterAverageValue(cat, prices, fallback)
		if err != nil {
			return nil, nil, err
		}
		if value >= limit {
			expensive = append(expensive, cat)
			continue
		}
		common = append(common, cat)
	}
	return common, expensive, nil
}
