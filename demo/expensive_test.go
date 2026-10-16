package demo

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// TestBuild_demoHoldsAnExpensiveSetThatIsNotPlayed checks that the demo directory carries a set in
// the expensive bundle, so the exclusion has something to act on, and that the played sets stay in
// the common bundle at the real limit.
func TestBuild_demoHoldsAnExpensiveSetThatIsNotPlayed(t *testing.T) {
	t.Parallel()
	ds := Build()

	if slices.Contains(ds.SetCodes, "DEX") {
		t.Errorf("SetCodes = %v, want DEX left out: it is an expensive set", ds.SetCodes)
	}
	dex, err := domain.LoadSetCatalogue(ds.Files, "DEX.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue DEX: %v", err)
	}
	prices, err := domain.LoadPriceSnapshot(ds.Files, "prices.json")
	if err != nil {
		t.Fatalf("LoadPriceSnapshot: %v", err)
	}

	for _, code := range append([]string{"DEX"}, ds.SetCodes...) {
		cat := dex
		if code != "DEX" {
			cat, err = domain.LoadSetCatalogue(ds.Files, code+".json")
			if err != nil {
				t.Fatalf("LoadSetCatalogue %s: %v", code, err)
			}
		}
		value, err := domain.BoosterAverageValue(cat, prices, 1)
		if err != nil {
			t.Fatalf("BoosterAverageValue %s: %v", code, err)
		}
		if expensive := value >= domain.ExpensiveBoosterValue; expensive != (code == "DEX") {
			t.Errorf("%s booster value = %d, at or above the %d limit = %v, want that only for DEX",
				code, value, domain.ExpensiveBoosterValue, expensive)
		}
	}
}
