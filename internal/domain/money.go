package domain

import "errors"

// ErrSubCoin is returned by PriceToCoins when a non-zero price at a given rate and percent
// converts to fewer than one coin. Callers must report this explicitly instead of treating the
// card as worth zero.
var ErrSubCoin = errors.New("price converts to less than one coin")

// ErrNegativeInput is returned by PriceToCoins when any of price, rate or pct is negative.
var ErrNegativeInput = errors.New("price, rate and pct must be non-negative")

// PriceToCoins converts a USD price to coins in one integer expression with one truncation:
//
//	coins = floor(price × rate × pct / 10_000)
//
// pct is a whole percentage (Percent(100) = 100%). Passing the Counter's sell or buy rate
// (60 or 120) directly avoids a second truncation step and keeps the buy/sell spread exact.
//
// Special cases:
//   - price == 0 or pct == 0: returns (0, nil).
//   - price > 0, pct > 0, result == 0: returns (0, ErrSubCoin).
//   - any input < 0: returns (0, ErrNegativeInput).
//
// Overflow: the intermediate product price × rate × pct must fit in int64 (max ≈ 9.2 × 10^18).
// For all realistic game values — price ≤ 1 000 000 cents ($10 000), rate ≤ 1 000 coins/USD,
// pct ≤ 10 000 — the product stays below 10^13, well within range.
func PriceToCoins(price USDCents, rate CoinRate, pct Percent) (Coins, error) {
	if price < 0 || rate < 0 || pct < 0 {
		return 0, ErrNegativeInput
	}
	if price == 0 || pct == 0 {
		return 0, nil
	}
	result := Coins(int64(price) * int64(rate) * int64(pct) / 10_000)
	if result == 0 {
		return 0, ErrSubCoin
	}
	return result, nil
}
