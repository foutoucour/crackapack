package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestCollectionCopiesSumsFinishesOfOnePrinting(t *testing.T) {
	t.Parallel()

	c := domain.NewCollection()
	nonFoil := domain.CardKey{ID: "a", Finish: domain.FinishNonFoil}
	foil := domain.CardKey{ID: "a", Finish: domain.FinishFoil}
	other := domain.CardKey{ID: "b", Finish: domain.FinishNonFoil}
	for key, n := range map[domain.CardKey]int{nonFoil: 2, foil: 1, other: 3} {
		if err := c.Add(key, n); err != nil {
			t.Fatalf("Add(%v, %d): %v", key, n, err)
		}
	}

	tests := []struct {
		name string
		id   domain.CardID
		want int
	}{
		{name: "both finishes of one printing", id: "a", want: 3},
		{name: "another printing is not mixed in", id: "b", want: 3},
		{name: "absent printing", id: "missing", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := c.Copies(tt.id); got != tt.want {
				t.Errorf("Copies(%q) = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}
