package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/demo"
)

// expensiveFromConfig puts the limit between the RAV booster, about $152, and the CLS one, about
// $14,000. The default limit of $60 would leave out both: the synthetic prices run far above real
// ones.
const expensiveFromConfig = "expensive_booster_value: 50000\n"

func TestLoadInputs_aRequestedExpensiveSetFailsTheRun(t *testing.T) {
	t.Parallel()
	dir := catalogueDirFS(t)
	var stdout strings.Builder
	a, _ := buildTestArgs(t, nil, nil, &stdout)
	a.catFS = dir
	a.pricesFS = dir
	a.setCodes = []string{"RAV", "CLS"}
	a.configReader = strings.NewReader(expensiveFromConfig)

	_, err := loadInputs(a)

	if err == nil {
		t.Fatal("want an error for a requested set in the expensive bundle, got nil")
	}
	for _, want := range []string{"CLS", "CLS ($14269.", "$500.00", "expensive"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q: %q", want, err.Error())
		}
	}
	if strings.Contains(err.Error(), "RAV") {
		t.Errorf("error names RAV, which is not expensive: %q", err.Error())
	}
}

func TestLoadInputs_anExpensiveSetOfTheDirectoryIsLeftOutQuietly(t *testing.T) {
	t.Parallel()
	dir := catalogueDirFS(t)
	var stdout strings.Builder
	a, _ := buildTestArgs(t, nil, nil, &stdout)
	a.catFS = dir
	a.pricesFS = dir
	a.setCodes = []string{"RAV"}
	a.configReader = strings.NewReader(expensiveFromConfig)

	in, err := loadInputs(a)
	if err != nil {
		t.Fatalf("loadInputs: %v", err)
	}

	if got, want := referenceCodes(in), []string{"RAV"}; !slices.Equal(got, want) {
		t.Errorf("reference = %v, want %v: the expensive CLS must not feed the energy cost median", got, want)
	}
	if len(in.catalogues) != 1 || in.catalogues[0].Code != "RAV" {
		t.Errorf("catalogues = %v, want RAV alone", in.catalogues)
	}
}

func demoArgs(ds demo.Dataset, setCodes []string) simulateArgs {
	return simulateArgs{
		configReader: strings.NewReader(ds.ConfigYAML),
		catFS:        ds.Files,
		setCodes:     setCodes,
		suppCodes:    ds.SupplementaryCodes,
		pricesFS:     ds.Files,
		pricesName:   "prices.json",
		player:       "trader",
		policies:     buildPolicies(),
	}
}

// The demo runs at the real limit of $60: its DEX set is expensive and its played sets are not.
func TestLoadInputs_theDemoLeavesItsExpensiveSetOut(t *testing.T) {
	t.Parallel()
	ds := demo.Build()

	in, err := loadInputs(demoArgs(ds, ds.SetCodes))
	if err != nil {
		t.Fatalf("loadInputs: %v", err)
	}

	if got, want := referenceCodes(in), []string{"DEM", "DSP"}; !slices.Equal(got, want) {
		t.Errorf("reference = %v, want %v", got, want)
	}
}

func TestRunPlay_aRequestedExpensiveSetFailsBeforeServing(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	a.setCodes = append([]string{"DEX"}, a.setCodes...)

	err := runPlay(context.Background(), a)

	if err == nil || !strings.Contains(err.Error(), "DEX ($120.00)") {
		t.Errorf("error = %v, want one naming DEX at $120.00", err)
	}
}

func TestLoadInputs_theDemoRefusesItsExpensiveSet(t *testing.T) {
	t.Parallel()
	ds := demo.Build()

	_, err := loadInputs(demoArgs(ds, append([]string{"DEX"}, ds.SetCodes...)))

	if err == nil || !strings.Contains(err.Error(), "DEX ($120.00)") {
		t.Errorf("error = %v, want one naming DEX at $120.00", err)
	}
}
