package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func TestCounterAcceptSurplusAddsStockWithoutLedgerEntry(t *testing.T) {
	t.Parallel()

	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), counterParams())
	key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	before := c.Stock(key)

	if err := c.AcceptSurplus(key, 3); err != nil {
		t.Fatalf("AcceptSurplus: %v", err)
	}

	if got, want := c.Stock(key), before+3; got != want {
		t.Errorf("Stock = %d, want %d", got, want)
	}
	if n := len(j.Entries()); n != 0 {
		t.Errorf("journal has %d entries, want none: NPCs hold no coins", n)
	}
}

func TestCounterAcceptSurplusRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	known := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	tests := []struct {
		name    string
		key     domain.CardKey
		n       int
		wantErr error
	}{
		{name: "unknown card", key: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, n: 1, wantErr: domain.ErrUnknownCard},
		{name: "finish not listed for the card", key: domain.CardKey{ID: "synth-c-0001", Finish: domain.FinishFoil}, n: 1, wantErr: domain.ErrUnknownCard},
		{name: "zero copies", key: known, n: 0},
		{name: "negative copies", key: known, n: -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), counterParams())
			before := c.Stock(tt.key)

			err := c.AcceptSurplus(tt.key, tt.n)

			if err == nil {
				t.Fatal("want error, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			var invalid *domain.InvalidQuantityError
			if tt.wantErr == nil && !errors.As(err, &invalid) {
				t.Errorf("error = %v, want InvalidQuantityError", err)
			}
			if got := c.Stock(tt.key); got != before {
				t.Errorf("Stock changed to %d after a rejected intake, want %d", got, before)
			}
		})
	}
}
