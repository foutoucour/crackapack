package domain

import (
	"slices"
	"strings"
	"testing"
)

// boosterWorth builds a catalogue whose booster draws one card priced at value, and the snapshot
// that prices it.
func boosterWorth(code string, value USDCents) (SetCatalogue, PriceSnapshot) {
	id := CardID(code + "-1")
	cat := makeSingleSheetCatalogue([]SheetCard{{ID: id, Weight: 1}}, 1)
	cat.Code = code
	return cat, PriceSnapshot{id: value}
}

// testLimit is the limit the tests split at. Passing it is the point of the parameter: callers with
// synthetic prices pick their own.
const testLimit USDCents = 5000

func catalogueCodes(cats []SetCatalogue) []string {
	codes := make([]string, 0, len(cats))
	for _, cat := range cats {
		codes = append(codes, cat.Code)
	}
	return codes
}

func TestSplitByBoosterValueKeepsTheLimitInTheExpensiveBundle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value USDCents
		want  string // the bundle that holds the set
	}{
		{"one cent under the limit", testLimit - 1, "common"},
		{"exactly the limit", testLimit, "expensive"},
		{"far above the limit", 100 * testLimit, "expensive"},
		{"a free booster", 0, "common"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cat, prices := boosterWorth("AAA", tc.value)

			common, expensive, err := SplitByBoosterValue([]SetCatalogue{cat}, prices, 0, testLimit)
			if err != nil {
				t.Fatalf("SplitByBoosterValue: %v", err)
			}

			want := map[string][]string{"common": {"AAA"}, "expensive": {}}
			if tc.want == "expensive" {
				want = map[string][]string{"common": {}, "expensive": {"AAA"}}
			}
			if got := catalogueCodes(common); !slices.Equal(got, want["common"]) {
				t.Errorf("common = %v, want %v", got, want["common"])
			}
			if got := catalogueCodes(expensive); !slices.Equal(got, want["expensive"]) {
				t.Errorf("expensive = %v, want %v", got, want["expensive"])
			}
		})
	}
}

func TestSplitByBoosterValueKeepsTheOrderOfEachBundle(t *testing.T) {
	t.Parallel()
	cheapA, pricesA := boosterWorth("CHA", 100)
	dearA, pricesB := boosterWorth("DRA", testLimit)
	cheapB, pricesC := boosterWorth("CHB", 200)
	dearB, pricesD := boosterWorth("DRB", 2*testLimit)
	prices := PriceSnapshot{}
	for _, p := range []PriceSnapshot{pricesA, pricesB, pricesC, pricesD} {
		for id, price := range p {
			prices[id] = price
		}
	}

	common, expensive, err := SplitByBoosterValue([]SetCatalogue{cheapA, dearA, cheapB, dearB}, prices, 0, testLimit)
	if err != nil {
		t.Fatalf("SplitByBoosterValue: %v", err)
	}

	if got, want := catalogueCodes(common), []string{"CHA", "CHB"}; !slices.Equal(got, want) {
		t.Errorf("common = %v, want %v", got, want)
	}
	if got, want := catalogueCodes(expensive), []string{"DRA", "DRB"}; !slices.Equal(got, want) {
		t.Errorf("expensive = %v, want %v", got, want)
	}
}

func TestSplitByBoosterValuePricesAnUnpricedCardAtTheFallback(t *testing.T) {
	t.Parallel()
	cat, _ := boosterWorth("FBK", 0)

	common, expensive, err := SplitByBoosterValue([]SetCatalogue{cat}, PriceSnapshot{}, testLimit, testLimit)
	if err != nil {
		t.Fatalf("SplitByBoosterValue: %v", err)
	}

	if len(common) != 0 || !slices.Equal(catalogueCodes(expensive), []string{"FBK"}) {
		t.Errorf("common = %v, expensive = %v, want FBK expensive", catalogueCodes(common), catalogueCodes(expensive))
	}
}

func TestSplitByBoosterValueNamesTheSetThatHasNoBooster(t *testing.T) {
	t.Parallel()
	pool := SetCatalogue{Code: "POOL"}

	_, _, err := SplitByBoosterValue([]SetCatalogue{pool}, PriceSnapshot{}, 0, testLimit)

	if err == nil || !strings.Contains(err.Error(), "POOL") {
		t.Fatalf("want an error naming POOL for a set without booster options, got %v", err)
	}
}
