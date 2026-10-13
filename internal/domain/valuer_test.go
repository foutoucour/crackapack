package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestValuerPrice(t *testing.T) {
	t.Parallel()
	valuer := domain.NewValuer(domain.PriceSnapshot{"priced": 250, "free": 0}, 7)

	tests := []struct {
		name string
		id   domain.CardID
		want domain.USDCents
	}{
		{"a card in the snapshot", "priced", 250},
		{"a card priced at zero stays at zero", "free", 0},
		{"a card absent from the snapshot falls back", "missing", 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := valuer.Price(tt.id); got != tt.want {
				t.Errorf("Price(%q) = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}
