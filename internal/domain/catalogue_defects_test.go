package domain_test

// Tests for defect C: booster option data validation in the catalogue loader
// and the defensive guard in drawFromSheet (booster.go).

import (
	"errors"
	"math/rand/v2"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// minimalSetWithBooster returns a valid MTGJSON-shaped set file that has one
// sheet named sheetName and the supplied booster options JSON fragment.
// The sheet has one card with weight 100 so it is itself valid.
func minimalSetWithBooster(boosters string) []byte {
	return []byte(`{
  "data": {
    "code": "TST",
    "name": "Test Set",
    "cards": [
      {
        "uuid": "card-001",
        "name": "Test Card",
        "number": "1",
        "rarity": "common",
        "finishes": ["nonfoil"],
        "supertypes": [],
        "identifiers": {}
      }
    ],
    "booster": {
      "draft": {
        "boosters": ` + boosters + `,
        "sheets": {
          "common": {
            "cards": {"card-001": 100},
            "totalWeight": 100,
            "foil": false,
            "balanceColors": false
          }
        }
      }
    }
  }
}`)
}

// TestLoadSetCatalogue_boosterOptionValidation covers the three new validation
// rules on booster options added to LoadSetCatalogue.
func TestLoadSetCatalogue_boosterOptionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		boosters  string
		wantField string
	}{
		{
			name:      "option weight zero",
			boosters:  `[{"contents": {"common": 1}, "weight": 0}]`,
			wantField: "data.booster.draft.boosters[0].weight",
		},
		{
			name:      "option weight negative",
			boosters:  `[{"contents": {"common": 1}, "weight": -5}]`,
			wantField: "data.booster.draft.boosters[0].weight",
		},
		{
			name:      "content count zero",
			boosters:  `[{"contents": {"common": 0}, "weight": 1}]`,
			wantField: "data.booster.draft.boosters[0].contents.common",
		},
		{
			name:      "content count negative",
			boosters:  `[{"contents": {"common": -3}, "weight": 1}]`,
			wantField: "data.booster.draft.boosters[0].contents.common",
		},
		{
			name:      "content references unknown sheet",
			boosters:  `[{"contents": {"ghost": 1}, "weight": 1}]`,
			wantField: "data.booster.draft.boosters[0].contents.ghost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				"set.json": &fstest.MapFile{Data: minimalSetWithBooster(tt.boosters)},
			}
			_, err := domain.LoadSetCatalogue(fsys, "set.json")
			if err == nil {
				t.Fatal("want error, got nil")
			}
			var ce *domain.CatalogueError
			if !errors.As(err, &ce) {
				t.Fatalf("want *CatalogueError, got %T: %v", err, err)
			}
			if ce.Field != tt.wantField {
				t.Errorf("CatalogueError.Field = %q, want %q", ce.Field, tt.wantField)
			}
		})
	}
}

// TestLoadSetCatalogue_boosterContentErrorIsDeterministic verifies that when
// multiple content sheets have issues, the error always names the
// alphabetically-first offending sheet name, not a random one from map iteration.
func TestLoadSetCatalogue_boosterContentErrorIsDeterministic(t *testing.T) {
	t.Parallel()

	// "alpha" < "zeta" alphabetically; both have invalid (zero) counts.
	// The reported field must always be the "alpha" one.
	const boosters = `[{"contents": {"zeta": 0, "alpha": 0}, "weight": 1}]`

	// Run multiple times to surface any non-determinism from map iteration.
	for range 20 {
		fsys := fstest.MapFS{
			"set.json": &fstest.MapFile{Data: minimalSetWithBooster(boosters)},
		}
		_, err := domain.LoadSetCatalogue(fsys, "set.json")
		if err == nil {
			t.Fatal("want error, got nil")
		}
		var ce *domain.CatalogueError
		if !errors.As(err, &ce) {
			t.Fatalf("want *CatalogueError, got %T: %v", err, err)
		}
		const wantField = "data.booster.draft.boosters[0].contents.alpha"
		if ce.Field != wantField {
			t.Errorf("iteration: CatalogueError.Field = %q, want %q (deterministic alphabetical order)", ce.Field, wantField)
		}
	}
}

// TestClassicGenerator_errorNonPositiveCount verifies that a booster option
// with a zero or negative sheet count returns an error instead of panicking.
// This guards the drawFromSheet function defensively.
func TestClassicGenerator_errorNonPositiveCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		count int
	}{
		{"zero count", 0},
		{"negative count", -1},
	}
	gen := domain.NewClassicGenerator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cat := domaintest.NewCatalogue(42)
			cat.BoosterOptions = []domain.BoosterOption{
				{Contents: map[string]int{"rare": tt.count}, Weight: 1},
			}
			rng := rand.New(rand.NewPCG(1, 1))
			_, err := gen.Generate(cat, rng)
			if err == nil {
				t.Fatalf("Generate with count=%d: want error (not panic), got nil", tt.count)
			}
		})
	}
}
