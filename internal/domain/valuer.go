package domain

// Valuer gives the reference value of a card: its price in the run's snapshot, or the fallback
// price when the card has none. The Counter and trades value cards through it, so both agree.
//
// A foil and its non-foil share one CardID and therefore one value, a documented simplification
// pending a per-finish price source.
type Valuer struct {
	prices   PriceSnapshot
	fallback USDCents
}

// NewValuer returns a Valuer over prices. fallback values the cards absent from prices.
func NewValuer(prices PriceSnapshot, fallback USDCents) Valuer {
	return Valuer{prices: prices, fallback: fallback}
}

// Price returns the reference value of the printing id in USD cents.
func (v Valuer) Price(id CardID) USDCents {
	if p, ok := v.prices[id]; ok {
		return p
	}
	return v.fallback
}
