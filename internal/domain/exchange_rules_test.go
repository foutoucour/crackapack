package domain

import (
	"errors"
	"slices"
	"testing"
)

func TestRequiredTopUp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		player USDCents
		npc    USDCents
		margin Percent
		rate   CoinRate
		want   Coins
	}{
		{"equal value, no margin", 100, 100, 0, 100, 0},
		{"the player gives more than needed", 200, 100, 0, 100, 0},
		{"one cent short costs one coin at rate 100", 100, 101, 0, 100, 1},
		{"the margin sits on top of the value", 100, 150, 10, 100, 65},
		{"a fractional coin rounds up in the NPC's favour", 100, 150, 1, 100, 52},
		{"a margin on equal value", 100, 100, 1, 100, 1},
		{"a lower rate makes a coin worth more dollars", 100, 150, 0, 10, 5},
		{"a priced card asked for with nothing offered", 0, 100, 0, 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := requiredTopUp(tt.player, tt.npc, tt.margin, tt.rate); got != tt.want {
				t.Errorf("requiredTopUp(%d, %d, %d%%, rate %d) = %d, want %d", tt.player, tt.npc, tt.margin, tt.rate, got, tt.want)
			}
		})
	}
}

func TestNormalizeLinesMergesAndSorts(t *testing.T) {
	t.Parallel()
	b := CardKey{ID: "b", Finish: FinishNonFoil}
	a := CardKey{ID: "a", Finish: FinishNonFoil}
	aFoil := CardKey{ID: "a", Finish: FinishFoil}

	got, err := normalizeLines([]TradeLine{{b, 1}, {a, 1}, {aFoil, 2}, {a, 2}})
	if err != nil {
		t.Fatalf("normalizeLines: %v", err)
	}

	// Finishes sort as strings, so "foil" comes before "nonfoil".
	want := []TradeLine{{aFoil, 2}, {a, 3}, {b, 1}}
	if !slices.Equal(got, want) {
		t.Errorf("normalizeLines = %v, want %v", got, want)
	}
}

func TestNormalizeLinesRejectsNonPositiveQuantities(t *testing.T) {
	t.Parallel()
	key := CardKey{ID: "a", Finish: FinishNonFoil}
	for _, qty := range []int{0, -1} {
		if _, err := normalizeLines([]TradeLine{{key, qty}}); !errors.Is(err, ErrInvalidLine) {
			t.Errorf("quantity %d: err = %v, want ErrInvalidLine", qty, err)
		}
	}
}

func TestLinesValueAndCardCount(t *testing.T) {
	t.Parallel()
	valuer := NewValuer(PriceSnapshot{"a": 100, "b": 250}, 7)
	lines := []TradeLine{
		{CardKey{ID: "a", Finish: FinishNonFoil}, 2},
		{CardKey{ID: "b", Finish: FinishNonFoil}, 1},
		{CardKey{ID: "c", Finish: FinishNonFoil}, 1},
	}

	if got := linesValue(valuer, lines); got != 2*100+250+7 {
		t.Errorf("linesValue = %d, want 457", got)
	}
	if got := cardCount(lines); got != 4 {
		t.Errorf("cardCount = %d, want 4", got)
	}
}
