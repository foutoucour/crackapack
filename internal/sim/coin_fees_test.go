package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// At a coin rate of 100 and a sell percentage of 60, a booster worth $10.00 cashes out at 600
// coins and one worth $20.00 at 1,200. With a cap of 12 energy at 40 coins each the cap is worth
// 480, so only the boosters above 480 coins pay a fee, and they pay the difference.
func TestCoinFees_chargeOnlyWhatPassesTheCap(t *testing.T) {
	t.Parallel()
	const (
		coinRate  = domain.CoinRate(100)
		sellPct   = domain.Percent(60)
		energyCap = 12
	)
	values := []domain.USDCents{300, 1000, 2000} // 180, 600 and 1,200 coins

	tests := []struct {
		name string
		p    domain.Coins
		want map[string]domain.Coins
	}{
		{"no price means no fee", 0, nil},
		{"only the boosters above the cap pay", 40, map[string]domain.Coins{"B": 120, "C": 720}},
		{"a booster worth exactly the cap pays nothing", 50, map[string]domain.Coins{"C": 600}},
		{"a high price removes every fee", 100, map[string]domain.Coins{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := sim.CoinFeesForTest(tc.p, energyCap, coinRate, sellPct, values)
			if err != nil {
				t.Fatalf("coinFees: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("fees = %v, want %v", got, tc.want)
			}
			for code, fee := range tc.want {
				if got[code] != fee {
					t.Errorf("fee of %s = %d, want %d", code, got[code], fee)
				}
			}
		})
	}
}
