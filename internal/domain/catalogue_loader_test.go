package domain_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
)

// validSetJSON returns a minimal valid MTGJSON-shaped set file with three cards:
//   - uuid-aa: common, nonfoil, has ScryfallID, not basic
//   - uuid-bb: common, nonfoil+foil, basic land
//   - uuid-cc: rare, nonfoil+foil, not basic
//
// The common sheet lists uuid-bb before uuid-aa so the ordering test is meaningful.
func validSetJSON() string {
	return `{
  "data": {
    "code": "SYN",
    "name": "Synthetic Set",
    "cards": [
      {
        "uuid": "uuid-aa",
        "name": "Common Card",
        "number": "1",
        "rarity": "common",
        "finishes": ["nonfoil"],
        "supertypes": [],
        "identifiers": {"scryfallId": "scry-0001"}
      },
      {
        "uuid": "uuid-bb",
        "name": "Snow-Covered Island",
        "number": "2",
        "rarity": "common",
        "finishes": ["nonfoil", "foil"],
        "supertypes": ["Basic", "Snow"],
        "identifiers": {}
      },
      {
        "uuid": "uuid-cc",
        "name": "Rare Card",
        "number": "3",
        "rarity": "rare",
        "finishes": ["nonfoil", "foil"],
        "supertypes": [],
        "identifiers": {}
      }
    ],
    "booster": {
      "draft": {
        "boosters": [
          {"contents": {"common": 2, "rare": 1}, "weight": 1}
        ],
        "sheets": {
          "common": {
            "cards": {"uuid-bb": 50, "uuid-aa": 100},
            "totalWeight": 150,
            "foil": false,
            "balanceColors": true
          },
          "rare": {
            "cards": {"uuid-cc": 100},
            "totalWeight": 100,
            "foil": false,
            "balanceColors": false
          }
        }
      }
    }
  }
}`
}

func TestLoadSetCatalogue_errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFS   func() fstest.MapFS
		file      string
		wantField string
	}{
		{
			name:      "missing file",
			setupFS:   func() fstest.MapFS { return fstest.MapFS{} },
			file:      "set.json",
			wantField: "(file)",
		},
		{
			name: "malformed JSON",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(`{invalid`)}}
			},
			file:      "set.json",
			wantField: "(json)",
		},
		{
			name: "missing data.code",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"","name":"X","cards":[],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.code",
		},
		{
			name: "missing data.name",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"","cards":[],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.name",
		},
		{
			name: "missing data.booster.draft",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[],"booster":{}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.booster.draft",
		},
		{
			name: "unknown rarity",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[` +
						`{"uuid":"u1","name":"n","number":"1","rarity":"mythic","finishes":[],"supertypes":[],"identifiers":{}}` +
						`],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.cards[0].rarity",
		},
		// Fix 2: JSON type errors use te.Field from *json.UnmarshalTypeError.
		{
			name: "JSON type error uses field path",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[],"booster":{"draft":{"boosters":[],"sheets":{` +
						`"common":{"cards":{},"totalWeight":"bad","foil":false,"balanceColors":false}}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.booster.draft.sheets.common.totalWeight",
		},
		// Fix 3: structural validations that would corrupt draws.
		{
			name: "empty uuid",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[` +
						`{"uuid":"","name":"n","number":"1","rarity":"common","finishes":["nonfoil"],"supertypes":[],"identifiers":{}}` +
						`],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.cards[0].uuid",
		},
		{
			name: "duplicate uuid",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[` +
						`{"uuid":"u1","name":"n","number":"1","rarity":"common","finishes":["nonfoil"],"supertypes":[],"identifiers":{}},` +
						`{"uuid":"u1","name":"m","number":"2","rarity":"common","finishes":["nonfoil"],"supertypes":[],"identifiers":{}}` +
						`],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.cards[1].uuid",
		},
		{
			name: "unknown finish",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[` +
						`{"uuid":"u1","name":"n","number":"1","rarity":"common","finishes":["foil-etched"],"supertypes":[],"identifiers":{}}` +
						`],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.cards[0].finishes[0]",
		},
		{
			name: "sheet card weight zero",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[],"booster":{"draft":{"boosters":[],"sheets":{` +
						`"common":{"cards":{"uuid-aa":0},"totalWeight":0,"foil":false,"balanceColors":false}}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.booster.draft.sheets.common.cards.uuid-aa",
		},
		{
			name: "sheet card weight negative",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[],"booster":{"draft":{"boosters":[],"sheets":{` +
						`"common":{"cards":{"uuid-aa":-1},"totalWeight":-1,"foil":false,"balanceColors":false}}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.booster.draft.sheets.common.cards.uuid-aa",
		},
		{
			name: "sheet totalWeight mismatch",
			setupFS: func() fstest.MapFS {
				return fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(
					`{"data":{"code":"SYN","name":"X","cards":[],"booster":{"draft":{"boosters":[],"sheets":{` +
						`"common":{"cards":{"uuid-aa":100},"totalWeight":999,"foil":false,"balanceColors":false}}}}}}`,
				)}}
			},
			file:      "set.json",
			wantField: "data.booster.draft.sheets.common.totalWeight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.LoadSetCatalogue(tt.setupFS(), tt.file)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			var ce *domain.CatalogueError
			if !errors.As(err, &ce) {
				t.Fatalf("want *CatalogueError, got %T: %v", err, err)
			}
			if ce.File != tt.file {
				t.Errorf("CatalogueError.File = %q, want %q", ce.File, tt.file)
			}
			if ce.Field != tt.wantField {
				t.Errorf("CatalogueError.Field = %q, want %q", ce.Field, tt.wantField)
			}
		})
	}
}

func TestLoadSetCatalogue_success(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"set.json": &fstest.MapFile{Data: []byte(validSetJSON())},
	}
	cat, err := domain.LoadSetCatalogue(fsys, "set.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set metadata.
	if cat.Code != "SYN" {
		t.Errorf("Code = %q, want SYN", cat.Code)
	}
	if cat.Name != "Synthetic Set" {
		t.Errorf("Name = %q, want Synthetic Set", cat.Name)
	}

	// Cards.
	if len(cat.Cards) != 3 {
		t.Fatalf("len(Cards) = %d, want 3", len(cat.Cards))
	}

	cardTests := []struct {
		id          domain.CardID
		name        string
		number      string
		rarity      domain.Rarity
		finishes    []domain.Finish
		isBasicLand bool
		scryfallID  string
	}{
		{
			id: "uuid-aa", name: "Common Card", number: "1",
			rarity: domain.RarityCommon, finishes: []domain.Finish{domain.FinishNonFoil},
			isBasicLand: false, scryfallID: "scry-0001",
		},
		{
			id: "uuid-bb", name: "Snow-Covered Island", number: "2",
			rarity: domain.RarityCommon, finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			isBasicLand: true, scryfallID: "",
		},
		{
			id: "uuid-cc", name: "Rare Card", number: "3",
			rarity: domain.RarityRare, finishes: []domain.Finish{domain.FinishNonFoil, domain.FinishFoil},
			isBasicLand: false, scryfallID: "",
		},
	}
	for _, want := range cardTests {
		got, ok := cat.Cards[want.id]
		if !ok {
			t.Errorf("card %q missing from catalogue", want.id)
			continue
		}
		if got.Name != want.name {
			t.Errorf("card %q: Name = %q, want %q", want.id, got.Name, want.name)
		}
		if got.Number != want.number {
			t.Errorf("card %q: Number = %q, want %q", want.id, got.Number, want.number)
		}
		if got.Rarity != want.rarity {
			t.Errorf("card %q: Rarity = %q, want %q", want.id, got.Rarity, want.rarity)
		}
		if len(got.Finishes) != len(want.finishes) {
			t.Errorf("card %q: len(Finishes) = %d, want %d", want.id, len(got.Finishes), len(want.finishes))
		} else {
			for i, f := range want.finishes {
				if got.Finishes[i] != f {
					t.Errorf("card %q: Finishes[%d] = %q, want %q", want.id, i, got.Finishes[i], f)
				}
			}
		}
		if got.IsBasicLand != want.isBasicLand {
			t.Errorf("card %q: IsBasicLand = %v, want %v", want.id, got.IsBasicLand, want.isBasicLand)
		}
		if got.ScryfallID != want.scryfallID {
			t.Errorf("card %q: ScryfallID = %q, want %q", want.id, got.ScryfallID, want.scryfallID)
		}
	}

	// Sheets.
	commonSheet, ok := cat.Sheets["common"]
	if !ok {
		t.Fatal("sheet \"common\" missing")
	}
	if !commonSheet.BalanceColors {
		t.Error("common sheet: BalanceColors = false, want true")
	}
	if commonSheet.Foil {
		t.Error("common sheet: Foil = true, want false")
	}
	if commonSheet.TotalWeight != 150 {
		t.Errorf("common sheet: TotalWeight = %d, want 150", commonSheet.TotalWeight)
	}
	if len(commonSheet.Cards) != 2 {
		t.Fatalf("common sheet: len(Cards) = %d, want 2", len(commonSheet.Cards))
	}
	// Cards must be sorted by ID: uuid-aa before uuid-bb.
	if commonSheet.Cards[0].ID != "uuid-aa" || commonSheet.Cards[0].Weight != 100 {
		t.Errorf("common sheet Cards[0] = %+v, want {uuid-aa 100}", commonSheet.Cards[0])
	}
	if commonSheet.Cards[1].ID != "uuid-bb" || commonSheet.Cards[1].Weight != 50 {
		t.Errorf("common sheet Cards[1] = %+v, want {uuid-bb 50}", commonSheet.Cards[1])
	}

	rareSheet, ok := cat.Sheets["rare"]
	if !ok {
		t.Fatal("sheet \"rare\" missing")
	}
	if len(rareSheet.Cards) != 1 || rareSheet.Cards[0].ID != "uuid-cc" {
		t.Errorf("rare sheet Cards = %+v, want [{uuid-cc 100}]", rareSheet.Cards)
	}

	// Booster options.
	if len(cat.BoosterOptions) != 1 {
		t.Fatalf("len(BoosterOptions) = %d, want 1", len(cat.BoosterOptions))
	}
	opt := cat.BoosterOptions[0]
	if opt.Weight != 1 {
		t.Errorf("BoosterOptions[0].Weight = %d, want 1", opt.Weight)
	}
	if opt.Contents["common"] != 2 {
		t.Errorf("BoosterOptions[0].Contents[common] = %d, want 2", opt.Contents["common"])
	}
	if opt.Contents["rare"] != 1 {
		t.Errorf("BoosterOptions[0].Contents[rare] = %d, want 1", opt.Contents["rare"])
	}
}

func TestLoadSetCatalogue_sheetOrderIsDeterministic(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"set.json": &fstest.MapFile{Data: []byte(validSetJSON())},
	}

	cat1, err := domain.LoadSetCatalogue(fsys, "set.json")
	if err != nil {
		t.Fatal(err)
	}
	cat2, err := domain.LoadSetCatalogue(fsys, "set.json")
	if err != nil {
		t.Fatal(err)
	}

	sheet1 := cat1.Sheets["common"]
	sheet2 := cat2.Sheets["common"]

	if len(sheet1.Cards) != len(sheet2.Cards) {
		t.Fatalf("loads disagree: len(Cards) = %d vs %d", len(sheet1.Cards), len(sheet2.Cards))
	}
	for i := range sheet1.Cards {
		if sheet1.Cards[i] != sheet2.Cards[i] {
			t.Errorf("Cards[%d] differs between loads: %+v vs %+v", i, sheet1.Cards[i], sheet2.Cards[i])
		}
	}

	// Verify the sort order is ascending by ID.
	for i := 1; i < len(sheet1.Cards); i++ {
		if sheet1.Cards[i].ID < sheet1.Cards[i-1].ID {
			t.Errorf("cards not sorted: Cards[%d].ID %q < Cards[%d].ID %q",
				i, sheet1.Cards[i].ID, i-1, sheet1.Cards[i-1].ID)
		}
	}
}
