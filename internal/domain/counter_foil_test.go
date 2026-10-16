package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// TestCounterFoilPrice pins the foil price rule: a foil is worth FoilPct percent of its card's
// reference price at the Counter, both when it buys and when it sells, and a non-foil is unaffected.
func TestCounterFoilPrice(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(0)
	foil := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishFoil}
	nonFoil := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishNonFoil}

	tests := []struct {
		name         string
		foilPct      domain.Percent
		wantSellFoil domain.Coins // the player pays this for the foil
		wantBuyFoil  domain.Coins // the Counter pays this for the foil
	}{
		// $1.00 at rate 100: a non-foil sells for 120 coins and is bought for 60.
		{"unset keeps the foil at the non-foil price", 0, 120, 60},
		{"100 percent keeps the foil at the non-foil price", 100, 120, 60},
		{"300 percent triples both prices", 300, 360, 180},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			params := counterParams()
			params.FoilPct = tc.foilPct

			c, j := newTestCounter(t, []domain.SetCatalogue{cat}, counterPrices(), params)
			if err := c.BuyFromPlayer(foil); err != nil {
				t.Fatalf("BuyFromPlayer foil: %v", err)
			}
			if got := j.Balance(domain.AccountPlayer); got != tc.wantBuyFoil {
				t.Errorf("the Counter paid %d for the foil, want %d", got, tc.wantBuyFoil)
			}

			mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})
			before := j.Balance(domain.AccountPlayer)
			if err := c.SellToPlayer(foil); err != nil {
				t.Fatalf("SellToPlayer foil: %v", err)
			}
			if got := before - j.Balance(domain.AccountPlayer); got != tc.wantSellFoil {
				t.Errorf("the player paid %d for the foil, want %d", got, tc.wantSellFoil)
			}

			// The non-foil of the same card keeps its price whatever the foil costs.
			before = j.Balance(domain.AccountPlayer)
			if err := c.SellToPlayer(nonFoil); err != nil {
				t.Fatalf("SellToPlayer non-foil: %v", err)
			}
			if got := before - j.Balance(domain.AccountPlayer); got != 120 {
				t.Errorf("the player paid %d for the non-foil, want 120", got)
			}
		})
	}
}
