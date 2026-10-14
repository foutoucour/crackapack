package main

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// pricesFactor makes the Classic set about 100 times more valuable than the Ravnica one, far
// beyond the 4x that puts a booster in the top energy tier.
const pricesFactor = 100

// catalogueDirFS returns a catalogue directory with the openable sets RAV and CLS, the card pool
// TSB, and a prices.json that is not a set file. CLS is much pricier than RAV.
func catalogueDirFS(t *testing.T) fstest.MapFS {
	t.Helper()
	rav := domaintest.NewRavnicaCatalogue()
	cls := domaintest.NewClassicCatalogue()
	_, tsb := domaintest.NewTimeSpiralCatalogues()

	clsPrices := domain.PriceSnapshot{}
	for id, price := range domaintest.NewPrices(cls, 2) {
		clsPrices[id] = price * pricesFactor
	}
	prices := domaintest.MergePrices(domaintest.NewPrices(rav, 1), clsPrices, domaintest.NewPrices(tsb, 3))

	dir, err := domaintest.NewMapFS([]domain.SetCatalogue{rav, cls}, []domain.SetCatalogue{tsb}, prices)
	if err != nil {
		t.Fatalf("NewMapFS: %v", err)
	}
	return dir
}

func referenceCodes(in inputs) []string {
	codes := make([]string, 0, len(in.reference))
	for _, cat := range in.reference {
		codes = append(codes, cat.Code)
	}
	slices.Sort(codes)
	return codes
}

func TestLoadInputs_referenceIsEveryOpenableSetOfTheDirectory(t *testing.T) {
	t.Parallel()
	dir := catalogueDirFS(t)

	for _, sets := range [][]string{{"RAV"}, {"CLS"}, {"RAV", "CLS"}} {
		var stdout strings.Builder
		a, _ := buildTestArgs(t, nil, nil, &stdout)
		a.catFS = dir
		a.pricesFS = dir
		a.setCodes = sets

		in, err := loadInputs(a)
		if err != nil {
			t.Fatalf("--sets %v: loadInputs: %v", sets, err)
		}

		// TSB has no booster.draft and prices.json is no set file: neither is a reference set.
		if got, want := referenceCodes(in), []string{"CLS", "RAV"}; !slices.Equal(got, want) {
			t.Errorf("--sets %v: reference = %v, want %v", sets, got, want)
		}
	}
}

func TestLoadInputs_unrequestedFileWithDefectIsNamedInTheError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fileName string
		content  string
	}{
		{"malformed json", "BAD.json", `{"data":`},
		{"unknown rarity", "ODD.json", `{"data":{"code":"ODD","name":"Odd",` +
			`"cards":[{"uuid":"odd-1","name":"Odd","number":"1","rarity":"bonus","finishes":["nonfoil"],` +
			`"supertypes":[],"identifiers":{}}],` +
			`"booster":{"draft":{"boosters":[{"contents":{"common":1},"weight":1}],` +
			`"sheets":{"common":{"cards":{"odd-1":10},"totalWeight":10,"foil":false,"balanceColors":false}}}}}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := catalogueDirFS(t)
			dir[tc.fileName] = &fstest.MapFile{Data: []byte(tc.content)}
			var stdout strings.Builder
			a, _ := buildTestArgs(t, nil, nil, &stdout)
			a.catFS = dir
			a.pricesFS = dir
			a.setCodes = []string{"RAV"}

			_, err := loadInputs(a)
			if err == nil {
				t.Fatal("want an error for a defective file that was not requested, got nil")
			}
			if !strings.Contains(err.Error(), tc.fileName) {
				t.Errorf("error does not name %s: %q", tc.fileName, err.Error())
			}
		})
	}
}
