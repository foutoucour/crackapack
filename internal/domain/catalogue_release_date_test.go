package domain_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// minimalSetJSON returns a one-card, one-booster set JSON with the given releaseDate field injected.
// Pass an empty string for dateField to omit the field entirely.
func minimalSetJSON(dateField string) string {
	date := ""
	if dateField != "" {
		date = `"releaseDate": "` + dateField + `",`
	}
	return `{
  "data": {
    "code": "SYN", "name": "Synthetic",
    ` + date + `
    "cards": [
      {"uuid":"u1","name":"N","number":"1","rarity":"common","finishes":["nonfoil"],"supertypes":[],"identifiers":{}}
    ],
    "booster": {
      "draft": {
        "boosters": [{"contents":{"common":1},"weight":1}],
        "sheets": {"common":{"cards":{"u1":1},"totalWeight":1,"foil":false,"balanceColors":false}}
      }
    }
  }
}`
}

func mapFSWith(name, content string) fstest.MapFS {
	return fstest.MapFS{name: &fstest.MapFile{Data: []byte(content)}}
}

// TestLoadSetCatalogue_releaseDateLoaded verifies that a valid ISO release date is
// read into SetCatalogue.ReleaseDate without modification.
func TestLoadSetCatalogue_releaseDateLoaded(t *testing.T) {
	t.Parallel()

	cat, err := domain.LoadSetCatalogue(mapFSWith("set.json", minimalSetJSON("2005-10-07")), "set.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue: %v", err)
	}
	if cat.ReleaseDate != "2005-10-07" {
		t.Errorf("ReleaseDate = %q, want %q", cat.ReleaseDate, "2005-10-07")
	}
}

// TestLoadSetCatalogue_releaseDateAbsent verifies that an absent release date field
// loads as an empty string (not an error).
func TestLoadSetCatalogue_releaseDateAbsent(t *testing.T) {
	t.Parallel()

	cat, err := domain.LoadSetCatalogue(mapFSWith("set.json", minimalSetJSON("")), "set.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue: %v", err)
	}
	if cat.ReleaseDate != "" {
		t.Errorf("ReleaseDate = %q, want empty string when field is absent", cat.ReleaseDate)
	}
}

// TestLoadSetCatalogue_releaseDateInvalid verifies that a malformed release date is
// rejected with an error naming the data.releaseDate field.
func TestLoadSetCatalogue_releaseDateInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		date string
	}{
		{"not a date", "tomorrow"},
		{"wrong separator", "2005/10/07"},
		{"day month swapped", "07-10-2005"},
		{"partial", "2005-10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.LoadSetCatalogue(mapFSWith("set.json", minimalSetJSON(tt.date)), "set.json")
			if err == nil {
				t.Fatal("want error for invalid date, got nil")
			}
			if !strings.Contains(err.Error(), "data.releaseDate") {
				t.Errorf("error %q does not name data.releaseDate", err)
			}
		})
	}
}

// TestLoadSetCatalogue_releaseDateNumericType verifies that a JSON numeric value for
// releaseDate is rejected with a clear field-level error.
func TestLoadSetCatalogue_releaseDateNumericType(t *testing.T) {
	t.Parallel()

	json := `{"data":{"code":"SYN","name":"X",` +
		`"releaseDate":20051007,` +
		`"cards":[],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`
	_, err := domain.LoadSetCatalogue(mapFSWith("set.json", json), "set.json")
	if err == nil {
		t.Fatal("want error for numeric releaseDate, got nil")
	}
}

// TestLoadSetCatalogue_releaseDateRoundTrip verifies that the domaintest builders
// carry dates that survive a JSON round-trip through domaintest.CatalogueToJSON /
// domain.LoadSetCatalogue.
func TestLoadSetCatalogue_releaseDateRoundTrip(t *testing.T) {
	t.Parallel()

	type wantSet struct {
		cat  domain.SetCatalogue
		date string
	}
	cases := []wantSet{
		{domaintest.NewRavnicaCatalogue(), "2005-10-07"},
		{domaintest.NewClassicCatalogue(), "1999-04-21"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.cat.Code, func(t *testing.T) {
			t.Parallel()
			data, err := domaintest.CatalogueToJSON(tc.cat)
			if err != nil {
				t.Fatalf("CatalogueToJSON: %v", err)
			}
			fs := fstest.MapFS{"set.json": &fstest.MapFile{Data: data}}
			loaded, err := domain.LoadSetCatalogue(fs, "set.json")
			if err != nil {
				t.Fatalf("LoadSetCatalogue: %v", err)
			}
			if loaded.ReleaseDate != tc.date {
				t.Errorf("round-trip ReleaseDate = %q, want %q", loaded.ReleaseDate, tc.date)
			}
		})
	}
}
