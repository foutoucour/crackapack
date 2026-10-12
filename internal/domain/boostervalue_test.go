package domain

import (
	"testing"
)

// makeSingleSheetCatalogue builds a minimal SetCatalogue with one sheet named "main"
// and one BoosterOption that draws count cards from it.
func makeSingleSheetCatalogue(cards []SheetCard, drawCount int) SetCatalogue {
	cardMap := make(map[CardID]Card, len(cards))
	for _, sc := range cards {
		cardMap[sc.ID] = Card{ID: sc.ID}
	}
	return SetCatalogue{
		Code:  "TST",
		Cards: cardMap,
		Sheets: map[string]Sheet{
			"main": {Name: "main", Cards: cards},
		},
		BoosterOptions: []BoosterOption{
			{Contents: map[string]int{"main": drawCount}, Weight: 1},
		},
	}
}

// TestBoosterAverageValueSingleOptionSingleSheet verifies the weighted mean over
// one sheet with one draw slot, all prices known.
// Sheet: card A weight 2 price 100, card B weight 2 price 200.
// sheetMean = (2*100 + 2*200) / 4 = 600/4 = 150. Draw 1 → result = 150.
func TestBoosterAverageValueSingleOptionSingleSheet(t *testing.T) {
	cards := []SheetCard{
		{ID: "A", Weight: 2},
		{ID: "B", Weight: 2},
	}
	cat := makeSingleSheetCatalogue(cards, 1)
	prices := PriceSnapshot{"A": 100, "B": 200}

	got, err := BoosterAverageValue(cat, prices, 0)
	if err != nil {
		t.Fatalf("BoosterAverageValue: %v", err)
	}
	if got != 150 {
		t.Errorf("BoosterAverageValue = %d, want 150", got)
	}
}

// TestBoosterAverageValueNonIntegerFloor proves the result is floored exactly once
// at the end, not rounded at each intermediate step.
// Sheet: 3 cards, weight 1 each, prices 100, 100, 101.
// sheetMean = 301/3 ≈ 100.333…  Draw 1 → floor(301/3) = 100, not 101.
func TestBoosterAverageValueNonIntegerFloor(t *testing.T) {
	cards := []SheetCard{
		{ID: "X1", Weight: 1},
		{ID: "X2", Weight: 1},
		{ID: "X3", Weight: 1},
	}
	cat := makeSingleSheetCatalogue(cards, 1)
	prices := PriceSnapshot{"X1": 100, "X2": 100, "X3": 101}

	got, err := BoosterAverageValue(cat, prices, 0)
	if err != nil {
		t.Fatalf("BoosterAverageValue: %v", err)
	}
	if got != 100 {
		t.Errorf("BoosterAverageValue = %d, want 100 (floor of 301/3, not 101)", got)
	}
}

// TestBoosterAverageValueMultipleOptionsWeighted checks weighted averaging over options.
// Option A (weight 3): draw 1 from sheet A (mean price 100).
// Option B (weight 1): draw 1 from sheet B (mean price 200).
// weightedSum = 3×100 + 1×200 = 500; totalWeight = 4; result = floor(500/4) = 125.
func TestBoosterAverageValueMultipleOptionsWeighted(t *testing.T) {
	cat := SetCatalogue{
		Code: "TST",
		Cards: map[CardID]Card{
			"SA": {ID: "SA"},
			"SB": {ID: "SB"},
		},
		Sheets: map[string]Sheet{
			"sheetA": {Name: "sheetA", Cards: []SheetCard{{ID: "SA", Weight: 1}}},
			"sheetB": {Name: "sheetB", Cards: []SheetCard{{ID: "SB", Weight: 1}}},
		},
		BoosterOptions: []BoosterOption{
			{Contents: map[string]int{"sheetA": 1}, Weight: 3},
			{Contents: map[string]int{"sheetB": 1}, Weight: 1},
		},
	}
	prices := PriceSnapshot{"SA": 100, "SB": 200}

	got, err := BoosterAverageValue(cat, prices, 0)
	if err != nil {
		t.Fatalf("BoosterAverageValue: %v", err)
	}
	if got != 125 {
		t.Errorf("BoosterAverageValue = %d, want 125", got)
	}
}

// TestBoosterAverageValueFallbackPrice checks that a card absent from the snapshot
// uses the fallback price.
// Sheet: card P priced 200, card Q absent (fallback=50). Weights equal (1 each).
// sheetMean = (1*200 + 1*50)/2 = 125. Draw 1 → result = 125.
func TestBoosterAverageValueFallbackPrice(t *testing.T) {
	cards := []SheetCard{
		{ID: "P", Weight: 1},
		{ID: "Q", Weight: 1},
	}
	cat := makeSingleSheetCatalogue(cards, 1)
	prices := PriceSnapshot{"P": 200} // Q deliberately absent

	got, err := BoosterAverageValue(cat, prices, 50)
	if err != nil {
		t.Fatalf("BoosterAverageValue: %v", err)
	}
	if got != 125 {
		t.Errorf("BoosterAverageValue = %d, want 125", got)
	}
}

// TestBoosterAverageValueMultipleDrawsFromSheet verifies count > 1 multiplies sheetMean.
// Sheet: one card, price 30. Draw 3 → optionValue = 3×30 = 90.
func TestBoosterAverageValueMultipleDrawsFromSheet(t *testing.T) {
	cards := []SheetCard{{ID: "M", Weight: 1}}
	cat := makeSingleSheetCatalogue(cards, 3)
	prices := PriceSnapshot{"M": 30}

	got, err := BoosterAverageValue(cat, prices, 0)
	if err != nil {
		t.Fatalf("BoosterAverageValue: %v", err)
	}
	if got != 90 {
		t.Errorf("BoosterAverageValue = %d, want 90 (3 × 30)", got)
	}
}

// TestBoosterAverageValueErrors verifies each error condition.
func TestBoosterAverageValueErrors(t *testing.T) {
	baseCards := []SheetCard{{ID: "C", Weight: 1}}
	baseCat := makeSingleSheetCatalogue(baseCards, 1)
	prices := PriceSnapshot{"C": 100}

	t.Run("no booster options", func(t *testing.T) {
		cat := baseCat
		cat.BoosterOptions = nil
		_, err := BoosterAverageValue(cat, prices, 0)
		if err == nil {
			t.Error("expected error for empty BoosterOptions, got nil")
		}
	})

	t.Run("all option weights zero", func(t *testing.T) {
		cat := baseCat
		cat.BoosterOptions = []BoosterOption{
			{Contents: map[string]int{"main": 1}, Weight: 0},
		}
		_, err := BoosterAverageValue(cat, prices, 0)
		if err == nil {
			t.Error("expected error for zero total option weight, got nil")
		}
	})

	t.Run("sheet referenced but missing", func(t *testing.T) {
		cat := SetCatalogue{
			Code:  "TST",
			Cards: map[CardID]Card{"C": {ID: "C"}},
			Sheets: map[string]Sheet{
				"main": {Name: "main", Cards: baseCards},
			},
			BoosterOptions: []BoosterOption{
				{Contents: map[string]int{"ghost": 1}, Weight: 1},
			},
		}
		_, err := BoosterAverageValue(cat, prices, 0)
		if err == nil {
			t.Error("expected error for missing sheet, got nil")
		}
	})

	t.Run("sheet has no cards", func(t *testing.T) {
		cat := SetCatalogue{
			Code:  "TST",
			Cards: map[CardID]Card{},
			Sheets: map[string]Sheet{
				"empty": {Name: "empty", Cards: []SheetCard{}},
			},
			BoosterOptions: []BoosterOption{
				{Contents: map[string]int{"empty": 1}, Weight: 1},
			},
		}
		_, err := BoosterAverageValue(cat, prices, 0)
		if err == nil {
			t.Error("expected error for sheet with no cards, got nil")
		}
	})

	t.Run("sheet total weight is zero", func(t *testing.T) {
		cat := SetCatalogue{
			Code:  "TST",
			Cards: map[CardID]Card{"ZW": {ID: "ZW"}},
			Sheets: map[string]Sheet{
				"main": {Name: "main", Cards: []SheetCard{{ID: "ZW", Weight: 0}}},
			},
			BoosterOptions: []BoosterOption{
				{Contents: map[string]int{"main": 1}, Weight: 1},
			},
		}
		_, err := BoosterAverageValue(cat, PriceSnapshot{"ZW": 10}, 0)
		if err == nil {
			t.Error("expected error for zero sheet total weight, got nil")
		}
	})
}

// TestMedianValue covers the median definition, even/odd lengths and error case.
func TestMedianValue(t *testing.T) {
	cases := []struct {
		name    string
		input   []USDCents
		want    USDCents
		wantErr bool
	}{
		{
			name:  "odd length: unique middle",
			input: []USDCents{5, 3, 1, 4, 2}, // sorted: [1 2 3 4 5], idx=2 → 3
			want:  3,
		},
		{
			name:  "even length: lower median",
			input: []USDCents{4, 1, 3, 2}, // sorted: [1 2 3 4], idx=1 → 2
			want:  2,
		},
		{
			name:  "single element",
			input: []USDCents{42},
			want:  42,
		},
		{
			name:  "two elements: lower",
			input: []USDCents{10, 20}, // sorted: [10 20], idx=0 → 10
			want:  10,
		},
		{
			name:    "empty input",
			input:   []USDCents{},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MedianValue(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("MedianValue: %v", err)
			}
			if got != tc.want {
				t.Errorf("MedianValue = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestMedianValueDoesNotMutateInput confirms MedianValue does not sort the caller's slice.
func TestMedianValueDoesNotMutateInput(t *testing.T) {
	input := []USDCents{30, 10, 20}
	original := []USDCents{30, 10, 20}
	if _, err := MedianValue(input); err != nil {
		t.Fatal(err)
	}
	for i, v := range input {
		if v != original[i] {
			t.Errorf("input[%d] = %d, want %d (MedianValue must not mutate the caller's slice)", i, v, original[i])
		}
	}
}

// defaultCostTiers returns the energy cost tiers from docs/core-features.md.
func defaultCostTiers() []CostTier {
	return []CostTier{
		{Threshold: 50, Cost: 3},   // ≤ 0.5× median
		{Threshold: 100, Cost: 5},  // ≤ 1× median
		{Threshold: 200, Cost: 8},  // ≤ 2× median
		{Threshold: 400, Cost: 12}, // catch-all (4× and above)
	}
}

// TestEnergyCostDefaultTable exercises all four tiers from docs/core-features.md
// plus the above-all-thresholds catch-all.
func TestEnergyCostDefaultTable(t *testing.T) {
	tiers := defaultCostTiers()
	median := USDCents(100) // median = 100¢

	cases := []struct {
		name         string
		boosterValue USDCents
		wantCost     int
	}{
		// ratio = boosterValue*100/100 = boosterValue
		{"exactly at threshold 50 (≤0.5×)", 50, 3},
		{"below threshold 50", 25, 3},
		{"exactly at threshold 100 (×1)", 100, 5},
		{"between 50 and 100", 75, 5},
		{"exactly at threshold 200 (×2)", 200, 8},
		{"between 100 and 200", 150, 8},
		{"exactly at threshold 400 (×4)", 400, 12},
		{"above all thresholds (catch-all)", 500, 12},
		{"zero booster value (≤ any threshold)", 0, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EnergyCost(tc.boosterValue, median, tiers)
			if err != nil {
				t.Fatalf("EnergyCost: %v", err)
			}
			if got != tc.wantCost {
				t.Errorf("EnergyCost(%d, %d, tiers) = %d, want %d", tc.boosterValue, median, got, tc.wantCost)
			}
		})
	}
}

// TestEnergyCostRatioIntegerDivision confirms ratio uses floor (integer) division.
// booster=149, median=100 → ratio=149/100*100 = floor(149) = 149 → tier ≤200 → cost 8.
// booster=50, median=99 → ratio=floor(50*100/99)=floor(5000/99)=50 → tier ≤50 → cost 3.
func TestEnergyCostRatioIntegerDivision(t *testing.T) {
	tiers := defaultCostTiers()

	cases := []struct {
		name         string
		boosterValue USDCents
		median       USDCents
		wantCost     int
	}{
		{"149/100: ratio=149, tier 200→8", 149, 100, 8},
		{"50/99: ratio=50 (floor), tier 50→3", 50, 99, 3},
		{"51/99: ratio=51 (floor), tier 100→5", 51, 99, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EnergyCost(tc.boosterValue, tc.median, tiers)
			if err != nil {
				t.Fatalf("EnergyCost: %v", err)
			}
			if got != tc.wantCost {
				t.Errorf("EnergyCost(%d, %d) = %d, want %d", tc.boosterValue, tc.median, got, tc.wantCost)
			}
		})
	}
}

// TestEnergyCostErrors checks all error conditions.
func TestEnergyCostErrors(t *testing.T) {
	tiers := defaultCostTiers()

	t.Run("median zero", func(t *testing.T) {
		_, err := EnergyCost(100, 0, tiers)
		if err == nil {
			t.Error("expected error for median=0, got nil")
		}
	})

	t.Run("empty tiers", func(t *testing.T) {
		_, err := EnergyCost(100, 100, nil)
		if err == nil {
			t.Error("expected error for empty tiers, got nil")
		}
	})

	t.Run("tiers not strictly ascending", func(t *testing.T) {
		unsorted := []CostTier{
			{Threshold: 100, Cost: 5},
			{Threshold: 50, Cost: 3}, // out of order
			{Threshold: 200, Cost: 8},
		}
		_, err := EnergyCost(100, 100, unsorted)
		if err == nil {
			t.Error("expected error for unsorted tiers, got nil")
		}
	})

	t.Run("duplicate thresholds", func(t *testing.T) {
		dups := []CostTier{
			{Threshold: 100, Cost: 5},
			{Threshold: 100, Cost: 5}, // duplicate, not strictly ascending
		}
		_, err := EnergyCost(100, 100, dups)
		if err == nil {
			t.Error("expected error for duplicate thresholds, got nil")
		}
	})
}
