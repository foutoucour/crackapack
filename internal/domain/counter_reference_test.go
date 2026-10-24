package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func TestCounterReference(t *testing.T) {
	t.Parallel()

	prices := counterPrices()
	prices["synth-c-0003"] = 0
	params := counterParams()
	params.FoilPct = 300
	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, prices, params)

	tests := []struct {
		name    string
		key     domain.CardKey
		want    domain.Coins
		wantErr error
	}{
		// $2 at 100 coins a dollar, whole: neither the 60% the Counter pays nor the 120% it asks.
		{name: "a priced card", key: domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}, want: 200},
		{name: "a card the snapshot lacks is worth the fallback", key: domain.CardKey{ID: "synth-c-0002", Finish: domain.FinishNonFoil}, want: 10},
		{name: "a foil is worth the foil percentage of its card", key: domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishFoil}, want: 300},
		{name: "a card priced at zero is worth nothing", key: domain.CardKey{ID: "synth-c-0003", Finish: domain.FinishNonFoil}, want: 0},
		{name: "an unknown card", key: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, wantErr: domain.ErrUnknownCard},
		{name: "a finish the card lacks", key: domain.CardKey{ID: "synth-c-0001", Finish: domain.FinishFoil}, wantErr: domain.ErrUnknownCard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.Reference(tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Reference error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Reference = %d, want %d", got, tt.want)
			}
		})
	}
}

// A card worth a fraction of a coin at the coin rate is worth one coin, as the Counter pays it.
func TestCounterReference_aCardWorthLessThanACoinIsWorthOne(t *testing.T) {
	t.Parallel()
	params := counterParams()
	// 5 cents at 10 coins a dollar is half a coin.
	params.CoinRate = 10
	params.FallbackPrice = 5
	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), params)

	got, err := c.Reference(domain.CardKey{ID: "synth-c-0002", Finish: domain.FinishNonFoil})
	if err != nil {
		t.Fatalf("Reference: %v", err)
	}
	if got != 1 {
		t.Errorf("Reference = %d, want 1", got)
	}
}
