package domain_test

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestPriceToCoins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		price   domain.USDCents
		rate    domain.CoinRate
		pct     domain.Percent
		want    domain.Coins
		wantErr error
	}{
		// Zero inputs that return (0, nil).
		{name: "zero price", price: 0, rate: 100, pct: 100, want: 0},
		{name: "zero pct", price: 100, rate: 100, pct: 0, want: 0},

		// Valid conversions.
		{name: "dollar at 100 coins per USD reference", price: 100, rate: 100, pct: 100, want: 100},
		{name: "dollar at 100 coins per USD sell 60pct", price: 100, rate: 100, pct: 60, want: 60},
		{name: "dollar at 100 coins per USD buy 120pct", price: 100, rate: 100, pct: 120, want: 120},
		{name: "five cents at 20 coins per USD boundary", price: 5, rate: 20, pct: 100, want: 1},
		{name: "five cents at 25 coins per USD", price: 5, rate: 25, pct: 100, want: 1},
		{name: "high value card at 500 coins per USD buy", price: 1_000_000, rate: 500, pct: 120, want: 6_000_000},

		// Sub-coin: non-zero price and pct but result truncates to zero.
		{name: "five cents at 10 coins per USD sub-coin", price: 5, rate: 10, pct: 100, wantErr: domain.ErrSubCoin},
		{name: "five cents at 19 coins per USD sub-coin", price: 5, rate: 19, pct: 100, wantErr: domain.ErrSubCoin},
		{name: "zero rate with non-zero price and pct sub-coin", price: 100, rate: 0, pct: 100, wantErr: domain.ErrSubCoin},

		// Negative inputs.
		{name: "negative price", price: -1, rate: 100, pct: 100, wantErr: domain.ErrNegativeInput},
		{name: "negative rate", price: 100, rate: -1, pct: 100, wantErr: domain.ErrNegativeInput},
		{name: "negative pct", price: 100, rate: 100, pct: -1, wantErr: domain.ErrNegativeInput},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.PriceToCoins(tc.price, tc.rate, tc.pct)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("PriceToCoins(%d, %d, %d) error = %v, want %v",
					tc.price, tc.rate, tc.pct, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("PriceToCoins(%d, %d, %d) = %d, want %d",
					tc.price, tc.rate, tc.pct, got, tc.want)
			}
		})
	}
}

// TestPriceToCoinsMonotonic asserts that the result is non-decreasing in price, rate and pct.
// A fixed seed keeps the run deterministic.
func TestPriceToCoinsMonotonic(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(42, 0))

	const iterations = 500
	for range iterations {
		price := domain.USDCents(rng.Int64N(100_000) + 1)
		rate := domain.CoinRate(rng.Int64N(490) + 10) // 10 to 499 coins/USD
		pct := domain.Percent(rng.Int64N(900) + 100)  // 100 to 999

		deltaPrice := domain.USDCents(rng.Int64N(1_000) + 1)
		deltaRate := domain.CoinRate(rng.Int64N(100) + 1)
		deltaPct := domain.Percent(rng.Int64N(100) + 1)

		// Monotonic in price.
		c1, _ := domain.PriceToCoins(price, rate, pct)
		c2, _ := domain.PriceToCoins(price+deltaPrice, rate, pct)
		if c2 < c1 {
			t.Errorf("not monotonic in price: PriceToCoins(%d,%d,%d)=%d > PriceToCoins(%d,%d,%d)=%d",
				price, rate, pct, c1, price+deltaPrice, rate, pct, c2)
		}

		// Monotonic in rate.
		c3, _ := domain.PriceToCoins(price, rate, pct)
		c4, _ := domain.PriceToCoins(price, rate+deltaRate, pct)
		if c4 < c3 {
			t.Errorf("not monotonic in rate: PriceToCoins(%d,%d,%d)=%d > PriceToCoins(%d,%d,%d)=%d",
				price, rate, pct, c3, price, rate+deltaRate, pct, c4)
		}

		// Monotonic in pct.
		c5, _ := domain.PriceToCoins(price, rate, pct)
		c6, _ := domain.PriceToCoins(price, rate, pct+deltaPct)
		if c6 < c5 {
			t.Errorf("not monotonic in pct: PriceToCoins(%d,%d,%d)=%d > PriceToCoins(%d,%d,%d)=%d",
				price, rate, pct, c5, price, rate, pct+deltaPct, c6)
		}
	}
}

// TestPriceToCoinsNoProfitRoundTrip asserts that for pct=120 (buy) and pct=60 (sell),
// the buy amount is always at least twice the sell amount. This means a sell-then-buy
// round trip at the Counter never profits, regardless of rate or price.
//
// Proof sketch: floor(2x) >= 2·floor(x) for x >= 0, because 2·floor(x) = 2x - 2·frac(x)
// and floor(2x) = 2x - frac(2x), with frac(2x) <= 2·frac(x).
func TestPriceToCoinsNoProfitRoundTrip(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(7, 0))

	const iterations = 500
	for range iterations {
		price := domain.USDCents(rng.Int64N(100_000) + 1)
		rate := domain.CoinRate(rng.Int64N(490) + 10)

		sell, _ := domain.PriceToCoins(price, rate, 60)
		buy, _ := domain.PriceToCoins(price, rate, 120)

		// ErrSubCoin means 0 coins; the inequality 0 >= 2*0 holds trivially.
		if buy < 2*sell {
			t.Errorf("round-trip profit possible: price=%d rate=%d sell=%d buy=%d (want buy >= 2*sell)",
				price, rate, sell, buy)
		}
	}
}
