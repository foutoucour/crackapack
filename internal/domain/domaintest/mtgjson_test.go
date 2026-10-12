package domaintest_test

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// ---- comparison helpers ----------------------------------------------------

// checkCatalogueEqualToOriginal compares got against want field by field and
// returns a slice of human-readable difference messages. nil vs empty slices
// are treated as equal so that the serialiser's normalisation (the loader
// always returns a non-nil empty slice for an absent JSON array) does not
// produce false failures.
//
// Fields checked: Code, Name, every card's Name/Number/Rarity/IsBasicLand/
// ScryfallID/Finishes/Colors, every sheet's Cards(id+weight,same order)/
// TotalWeight/Foil/BalanceColors, every booster option's Contents+Weight
// in order.
func checkCatalogueEqualToOriginal(got, want domain.SetCatalogue) []string {
	var errs []string

	if got.Code != want.Code {
		errs = append(errs, "Code: got "+got.Code+", want "+want.Code)
	}
	if got.Name != want.Name {
		errs = append(errs, "Name: got "+got.Name+", want "+want.Name)
	}
	errs = append(errs, checkCards(got.Cards, want.Cards)...)
	errs = append(errs, checkSheets(got.Sheets, want.Sheets)...)
	errs = append(errs, checkBoosterOptions(got.BoosterOptions, want.BoosterOptions)...)
	return errs
}

func checkCards(got, want map[domain.CardID]domain.Card) []string {
	var errs []string
	if len(got) != len(want) {
		errs = append(errs, sprint("len(Cards): got %d, want %d", len(got), len(want)))
	}
	for id, wantCard := range want {
		gotCard, ok := got[id]
		if !ok {
			errs = append(errs, sprint("card %q: missing", id))
			continue
		}
		if gotCard.Name != wantCard.Name {
			errs = append(errs, sprint("card %q: Name got %q, want %q", id, gotCard.Name, wantCard.Name))
		}
		if gotCard.Number != wantCard.Number {
			errs = append(errs, sprint("card %q: Number got %q, want %q", id, gotCard.Number, wantCard.Number))
		}
		if gotCard.Rarity != wantCard.Rarity {
			errs = append(errs, sprint("card %q: Rarity got %q, want %q", id, gotCard.Rarity, wantCard.Rarity))
		}
		if gotCard.IsBasicLand != wantCard.IsBasicLand {
			errs = append(errs, sprint("card %q: IsBasicLand got %v, want %v", id, gotCard.IsBasicLand, wantCard.IsBasicLand))
		}
		if gotCard.ScryfallID != wantCard.ScryfallID {
			errs = append(errs, sprint("card %q: ScryfallID got %q, want %q", id, gotCard.ScryfallID, wantCard.ScryfallID))
		}
		// slices.Equal treats nil and []T{} as equal (both have length 0, no elements differ).
		if !slices.Equal(gotCard.Finishes, wantCard.Finishes) {
			errs = append(errs, sprint("card %q: Finishes got %v, want %v", id, gotCard.Finishes, wantCard.Finishes))
		}
		if !slices.Equal(gotCard.Colors, wantCard.Colors) {
			errs = append(errs, sprint("card %q: Colors got %v, want %v", id, gotCard.Colors, wantCard.Colors))
		}
	}
	return errs
}

func checkSheets(got, want map[string]domain.Sheet) []string {
	var errs []string
	if len(got) != len(want) {
		errs = append(errs, sprint("len(Sheets): got %d, want %d", len(got), len(want)))
	}
	for name, wantSheet := range want {
		gotSheet, ok := got[name]
		if !ok {
			errs = append(errs, sprint("sheet %q: missing", name))
			continue
		}
		if gotSheet.TotalWeight != wantSheet.TotalWeight {
			errs = append(errs, sprint("sheet %q: TotalWeight got %d, want %d",
				name, gotSheet.TotalWeight, wantSheet.TotalWeight))
		}
		if gotSheet.Foil != wantSheet.Foil {
			errs = append(errs, sprint("sheet %q: Foil got %v, want %v", name, gotSheet.Foil, wantSheet.Foil))
		}
		if gotSheet.BalanceColors != wantSheet.BalanceColors {
			errs = append(errs, sprint("sheet %q: BalanceColors got %v, want %v",
				name, gotSheet.BalanceColors, wantSheet.BalanceColors))
		}
		// SheetCard is a struct of two comparable fields; slices.Equal handles nil vs empty.
		if !slices.Equal(gotSheet.Cards, wantSheet.Cards) {
			errs = append(errs, sprint("sheet %q: Cards got %v, want %v",
				name, gotSheet.Cards, wantSheet.Cards))
		}
	}
	return errs
}

func checkBoosterOptions(got, want []domain.BoosterOption) []string {
	var errs []string
	if len(got) != len(want) {
		errs = append(errs, sprint("len(BoosterOptions): got %d, want %d", len(got), len(want)))
		return errs
	}
	for i := range want {
		if got[i].Weight != want[i].Weight {
			errs = append(errs, sprint("BoosterOptions[%d]: Weight got %d, want %d",
				i, got[i].Weight, want[i].Weight))
		}
		if !maps.Equal(got[i].Contents, want[i].Contents) {
			errs = append(errs, sprint("BoosterOptions[%d]: Contents got %v, want %v",
				i, got[i].Contents, want[i].Contents))
		}
	}
	return errs
}

// assertCatalogueEqualToOriginal is the test-reporting wrapper for
// checkCatalogueEqualToOriginal.
func assertCatalogueEqualToOriginal(t *testing.T, got, want domain.SetCatalogue) {
	t.Helper()
	for _, msg := range checkCatalogueEqualToOriginal(got, want) {
		t.Error(msg)
	}
}

// checkCardPoolEqualToOriginal checks Code, Name and Cards of a loaded card
// pool against the original builder output, normalising nil vs empty slices.
// Sheets and BoosterOptions are not checked because CardPoolToJSON does not
// serialise them.
func checkCardPoolEqualToOriginal(got, want domain.SetCatalogue) []string {
	var errs []string
	if got.Code != want.Code {
		errs = append(errs, sprint("Code: got %q, want %q", got.Code, want.Code))
	}
	if got.Name != want.Name {
		errs = append(errs, sprint("Name: got %q, want %q", got.Name, want.Name))
	}
	errs = append(errs, checkCards(got.Cards, want.Cards)...)
	if len(got.Sheets) != 0 {
		errs = append(errs, sprint("Sheets: got %d, want 0 (card pool has no booster section)", len(got.Sheets)))
	}
	if len(got.BoosterOptions) != 0 {
		errs = append(errs, sprint("BoosterOptions: got %d, want 0", len(got.BoosterOptions)))
	}
	return errs
}

// sprint is a thin sprintf wrapper to avoid importing fmt in production-facing
// test helpers. It uses fmt under the hood.
func sprint(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// ---- round-trip tests ------------------------------------------------------

func TestCatalogueToJSON_roundTripRavnica(t *testing.T) {
	t.Parallel()
	assertSetCatalogueRoundTrip(t, domaintest.NewRavnicaCatalogue())
}

func TestCatalogueToJSON_roundTripClassic(t *testing.T) {
	t.Parallel()
	assertSetCatalogueRoundTrip(t, domaintest.NewClassicCatalogue())
}

func TestCatalogueToJSON_roundTripTimeSpiralMain(t *testing.T) {
	t.Parallel()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	assertSetCatalogueRoundTrip(t, tsp)
}

// assertSetCatalogueRoundTrip is the shared round-trip helper for full catalogues.
// It serialises cat with CatalogueToJSON, loads the JSON with LoadSetCatalogue,
// and performs a strict field-by-field comparison against the ORIGINAL builder output.
func assertSetCatalogueRoundTrip(t *testing.T, cat domain.SetCatalogue) {
	t.Helper()

	data, err := domaintest.CatalogueToJSON(cat)
	if err != nil {
		t.Fatalf("CatalogueToJSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("CatalogueToJSON returned empty bytes")
	}

	mapFS := fstest.MapFS{cat.Code + ".json": &fstest.MapFile{Data: data}}
	got, err := domain.LoadSetCatalogue(mapFS, cat.Code+".json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue: %v", err)
	}

	assertCatalogueEqualToOriginal(t, got, cat)
}

// TestCardPoolToJSON_roundTripTSB verifies that CardPoolToJSON → LoadCardPool
// preserves Code, Name, and all card fields against the ORIGINAL builder output.
func TestCardPoolToJSON_roundTripTSB(t *testing.T) {
	t.Parallel()

	_, tsb := domaintest.NewTimeSpiralCatalogues()

	data, err := domaintest.CardPoolToJSON(tsb)
	if err != nil {
		t.Fatalf("CardPoolToJSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("CardPoolToJSON returned empty bytes")
	}

	mapFS := fstest.MapFS{tsb.Code + ".json": &fstest.MapFile{Data: data}}
	got, err := domain.LoadCardPool(mapFS, tsb.Code+".json")
	if err != nil {
		t.Fatalf("LoadCardPool: %v", err)
	}

	for _, msg := range checkCardPoolEqualToOriginal(got, tsb) {
		t.Error(msg)
	}
}

// TestCatalogueComparison_detectsMutation is a negative test that proves
// checkCatalogueEqualToOriginal can report a difference. It serialises the
// Ravnica catalogue to JSON, replaces the colors of one card (dropping "W" to
// empty), loads the mutated JSON, and asserts that the comparison helper
// returns at least one error message.
func TestCatalogueComparison_detectsMutation(t *testing.T) {
	t.Parallel()

	original := domaintest.NewRavnicaCatalogue()

	data, err := domaintest.CatalogueToJSON(original)
	if err != nil {
		t.Fatalf("CatalogueToJSON: %v", err)
	}

	// Drop the first occurrence of ["W"] from the colors field. rav-c-000 is
	// serialised first (ascending UUID order) and has Colors: []string{"W"}.
	// Replacing "W" with an empty array changes that card's colors so that the
	// comparison helper must report a mismatch on Colors.
	mutated := bytes.Replace(data, []byte(`["W"]`), []byte(`[]`), 1)
	if bytes.Equal(mutated, data) {
		t.Fatal("mutation did not change the JSON; serialiser format may have changed")
	}

	mapFS := fstest.MapFS{original.Code + ".json": &fstest.MapFile{Data: mutated}}
	loaded, err := domain.LoadSetCatalogue(mapFS, original.Code+".json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue with mutated JSON: %v", err)
	}

	diffs := checkCatalogueEqualToOriginal(loaded, original)
	if len(diffs) == 0 {
		t.Error("expected comparison to detect the Colors mutation but no differences were reported")
	}
}

// ---- price snapshot round-trip ---------------------------------------------

func TestPriceSnapshotToJSON_roundTrip(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewRavnicaCatalogue()
	snap := domaintest.NewPrices(cat, 42)

	data, err := domaintest.PriceSnapshotToJSON(snap)
	if err != nil {
		t.Fatalf("PriceSnapshotToJSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("PriceSnapshotToJSON returned empty bytes")
	}

	mapFS := fstest.MapFS{"prices.json": &fstest.MapFile{Data: data}}
	got, err := domain.LoadPriceSnapshot(mapFS, "prices.json")
	if err != nil {
		t.Fatalf("LoadPriceSnapshot: %v", err)
	}
	if !maps.Equal(snap, got) {
		t.Errorf("PriceSnapshot differs after round-trip: got %v, want %v", got, snap)
	}
}

// ---- MergePrices tests -----------------------------------------------------

func TestMergePrices_combinesDistinctIDs(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	cls := domaintest.NewClassicCatalogue()
	snapRAV := domaintest.NewPrices(rav, 1)
	snapCLS := domaintest.NewPrices(cls, 2)

	merged := domaintest.MergePrices(snapRAV, snapCLS)

	for id := range snapRAV {
		if _, ok := merged[id]; !ok {
			t.Errorf("RAV card %q missing from merged snapshot", id)
		}
	}
	for id := range snapCLS {
		if _, ok := merged[id]; !ok {
			t.Errorf("CLS card %q missing from merged snapshot", id)
		}
	}
}

func TestMergePrices_lastWins(t *testing.T) {
	t.Parallel()

	id := domain.CardID("test-id")
	snap1 := domain.PriceSnapshot{id: 100}
	snap2 := domain.PriceSnapshot{id: 200}

	merged := domaintest.MergePrices(snap1, snap2)
	if merged[id] != 200 {
		t.Errorf("merged[%q] = %d, want 200 (last wins)", id, merged[id])
	}
}

// ---- NewMapFS test ---------------------------------------------------------

func TestNewMapFS_containsExpectedKeys(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue()
	cls := domaintest.NewClassicCatalogue()
	_, tsb := domaintest.NewTimeSpiralCatalogues()
	prices := domaintest.MergePrices(
		domaintest.NewPrices(rav, 1),
		domaintest.NewPrices(cls, 2),
	)

	mapFS, err := domaintest.NewMapFS(
		[]domain.SetCatalogue{rav, cls},
		[]domain.SetCatalogue{tsb},
		prices,
	)
	if err != nil {
		t.Fatalf("NewMapFS: %v", err)
	}

	for _, key := range []string{"RAV.json", "CLS.json", "TSB.json", "prices.json"} {
		f, ok := mapFS[key]
		if !ok {
			t.Errorf("key %q missing from MapFS", key)
			continue
		}
		if len(f.Data) == 0 {
			t.Errorf("key %q has empty data", key)
		}
	}

	// RAV and CLS are full catalogues (parseable by LoadSetCatalogue).
	for _, code := range []string{"RAV", "CLS"} {
		if _, err := domain.LoadSetCatalogue(mapFS, code+".json"); err != nil {
			t.Errorf("LoadSetCatalogue(%s.json): %v", code, err)
		}
	}

	// TSB is a card pool (parseable by LoadCardPool, not by LoadSetCatalogue).
	if _, err := domain.LoadCardPool(mapFS, "TSB.json"); err != nil {
		t.Errorf("LoadCardPool(TSB.json): %v", err)
	}
}
