package domaintest_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// TestCatalogueToJSON_roundTripRavnica verifies that CatalogueToJSON produces
// valid JSON that LoadSetCatalogue can parse, and that re-serialising and
// re-loading gives byte-stable results.
func TestCatalogueToJSON_roundTripRavnica(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	roundTripSetCatalogue(t, cat)
}

func TestCatalogueToJSON_roundTripClassic(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewClassicCatalogue()
	roundTripSetCatalogue(t, cat)
}

func TestCatalogueToJSON_roundTripTimeSpiralMain(t *testing.T) {
	t.Parallel()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	roundTripSetCatalogue(t, tsp)
}

// roundTripSetCatalogue is a shared helper that asserts:
//  1. CatalogueToJSON succeeds and produces parseable JSON.
//  2. The loaded catalogue has the correct Code, Name, card count, sheet count,
//     and booster option count.
//  3. Re-serialising and re-loading gives a reflect.DeepEqual result (stability).
func roundTripSetCatalogue(t *testing.T, cat domain.SetCatalogue) {
	t.Helper()

	data, err := domaintest.CatalogueToJSON(cat)
	if err != nil {
		t.Fatalf("CatalogueToJSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("CatalogueToJSON returned empty bytes")
	}

	mapFS := fstest.MapFS{cat.Code + ".json": &fstest.MapFile{Data: data}}
	got1, err := domain.LoadSetCatalogue(mapFS, cat.Code+".json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue (first load): %v", err)
	}

	// Structural checks against the original builder output.
	if got1.Code != cat.Code {
		t.Errorf("Code = %q, want %q", got1.Code, cat.Code)
	}
	if got1.Name != cat.Name {
		t.Errorf("Name = %q, want %q", got1.Name, cat.Name)
	}
	if len(got1.Cards) != len(cat.Cards) {
		t.Errorf("len(Cards) = %d, want %d", len(got1.Cards), len(cat.Cards))
	}
	if len(got1.Sheets) != len(cat.Sheets) {
		t.Errorf("len(Sheets) = %d, want %d", len(got1.Sheets), len(cat.Sheets))
	}
	if len(got1.BoosterOptions) != len(cat.BoosterOptions) {
		t.Errorf("len(BoosterOptions) = %d, want %d", len(got1.BoosterOptions), len(cat.BoosterOptions))
	}

	// Verify all original card IDs are present with matching Rarity and IsBasicLand.
	for id, wantCard := range cat.Cards {
		gotCard, ok := got1.Cards[id]
		if !ok {
			t.Errorf("card %q missing after round-trip", id)
			continue
		}
		if gotCard.Rarity != wantCard.Rarity {
			t.Errorf("card %q: Rarity = %q, want %q", id, gotCard.Rarity, wantCard.Rarity)
		}
		if gotCard.IsBasicLand != wantCard.IsBasicLand {
			t.Errorf("card %q: IsBasicLand = %v, want %v", id, gotCard.IsBasicLand, wantCard.IsBasicLand)
		}
	}

	// Stability: re-serialise got1 and load again; the two loads must be equal.
	data2, err := domaintest.CatalogueToJSON(got1)
	if err != nil {
		t.Fatalf("CatalogueToJSON (second serialise): %v", err)
	}
	mapFS2 := fstest.MapFS{cat.Code + ".json": &fstest.MapFile{Data: data2}}
	got2, err := domain.LoadSetCatalogue(mapFS2, cat.Code+".json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue (second load): %v", err)
	}
	if !reflect.DeepEqual(got1, got2) {
		t.Error("catalogue differs between first and second round-trip load")
	}
}

// TestCardPoolToJSON_roundTripTSB verifies that CardPoolToJSON produces JSON
// that LoadCardPool can parse, with correct Code, Name, and Cards.
// Sheets and BoosterOptions are not preserved by design.
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

	if got.Code != tsb.Code {
		t.Errorf("Code = %q, want %q", got.Code, tsb.Code)
	}
	if got.Name != tsb.Name {
		t.Errorf("Name = %q, want %q", got.Name, tsb.Name)
	}
	if len(got.Cards) != len(tsb.Cards) {
		t.Errorf("len(Cards) = %d, want %d", len(got.Cards), len(tsb.Cards))
	}
	for id := range tsb.Cards {
		if _, ok := got.Cards[id]; !ok {
			t.Errorf("card %q missing after round-trip", id)
		}
	}
	// Sheets and BoosterOptions are not serialised by CardPoolToJSON.
	if len(got.Sheets) != 0 {
		t.Errorf("Sheets: got %d, want 0 (card pool has no booster section)", len(got.Sheets))
	}
	if len(got.BoosterOptions) != 0 {
		t.Errorf("BoosterOptions: got %d, want 0", len(got.BoosterOptions))
	}
}

// TestCardPoolToJSON_stability verifies that CardPoolToJSON → LoadCardPool → CardPoolToJSON
// produces identical bytes on the second serialisation.
func TestCardPoolToJSON_stability(t *testing.T) {
	t.Parallel()

	_, tsb := domaintest.NewTimeSpiralCatalogues()

	data1, err := domaintest.CardPoolToJSON(tsb)
	if err != nil {
		t.Fatalf("CardPoolToJSON (first): %v", err)
	}
	mapFS := fstest.MapFS{tsb.Code + ".json": &fstest.MapFile{Data: data1}}
	got1, err := domain.LoadCardPool(mapFS, tsb.Code+".json")
	if err != nil {
		t.Fatalf("LoadCardPool: %v", err)
	}
	data2, err := domaintest.CardPoolToJSON(got1)
	if err != nil {
		t.Fatalf("CardPoolToJSON (second): %v", err)
	}
	mapFS2 := fstest.MapFS{tsb.Code + ".json": &fstest.MapFile{Data: data2}}
	got2, err := domain.LoadCardPool(mapFS2, tsb.Code+".json")
	if err != nil {
		t.Fatalf("LoadCardPool (second): %v", err)
	}
	if !reflect.DeepEqual(got1, got2) {
		t.Error("card pool differs between first and second round-trip load")
	}
}

// TestPriceSnapshotToJSON_roundTrip verifies that PriceSnapshotToJSON produces
// JSON that LoadPriceSnapshot reads back to an equal PriceSnapshot.
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
	if !reflect.DeepEqual(snap, got) {
		t.Errorf("PriceSnapshot differs after round-trip: got %v, want %v", got, snap)
	}
}

// TestMergePrices_combinesDistinctIDs verifies that MergePrices merges two
// non-overlapping snapshots into one that contains all entries.
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

// TestMergePrices_lastWins verifies that the last snapshot wins on duplicate IDs.
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

// TestNewMapFS_containsExpectedKeys verifies that NewMapFS produces a MapFS
// with catalogue files and prices.json.
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

	// TSB is a card pool (parseable by LoadCardPool, NOT by LoadSetCatalogue).
	if _, err := domain.LoadCardPool(mapFS, "TSB.json"); err != nil {
		t.Errorf("LoadCardPool(TSB.json): %v", err)
	}
}
