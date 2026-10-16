package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// minimalArchetypeWithColors returns a minimal valid archetype YAML block with the given
// colours field. colors is a YAML inline sequence such as "[W, B]", or empty for no field.
func minimalArchetypeWithColors(name, colors string) string {
	line := "  archetypes:\n    " + name +
		": {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1"
	if colors != "" {
		line += ", colors: " + colors
	}
	return "npcs:\n" + line + "}\n"
}

func TestLoad_archetypeColorsRejectsUnknownLetter(t *testing.T) {
	t.Parallel()
	yaml := minimalArchetypeWithColors("casual", "[X]")
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("want error for unknown colour X, got nil")
	}
	if !strings.Contains(err.Error(), "npcs.archetypes.casual.colors") {
		t.Errorf("error %q does not name npcs.archetypes.casual.colors", err)
	}
	if !strings.Contains(err.Error(), `"X"`) {
		t.Errorf("error %q does not name the unknown colour X", err)
	}
}

func TestLoad_archetypeColorsRejectsLowercaseLetter(t *testing.T) {
	t.Parallel()
	yaml := minimalArchetypeWithColors("casual", "[w]")
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("want error for lowercase colour w, got nil")
	}
	if !strings.Contains(err.Error(), "npcs.archetypes.casual.colors") {
		t.Errorf("error %q does not name npcs.archetypes.casual.colors", err)
	}
}

func TestLoad_archetypeColorsRejectsDuplicate(t *testing.T) {
	t.Parallel()
	yaml := minimalArchetypeWithColors("casual", "[B, B]")
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("want error for duplicate colour B, got nil")
	}
	if !strings.Contains(err.Error(), "npcs.archetypes.casual.colors") {
		t.Errorf("error %q does not name npcs.archetypes.casual.colors", err)
	}
	if !strings.Contains(err.Error(), `"B"`) {
		t.Errorf("error %q does not name the duplicate colour B", err)
	}
}

func TestLoad_archetypeColorsAcceptsValidCombinations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		colors string
	}{
		{"empty colors", ""},
		{"single color W", "[W]"},
		{"single color B", "[B]"},
		{"two colors UB", "[U, B]"},
		{"all five colors", "[W, U, B, R, G]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			yaml := minimalArchetypeWithColors("casual", tt.colors)
			if _, err := config.Load(strings.NewReader(yaml)); err != nil {
				t.Fatalf("Load: %v", err)
			}
		})
	}
}

func TestLoad_archetypeColorsReportsFirstBadArchetypeInNameOrder(t *testing.T) {
	t.Parallel()
	// "alpha" < "zulu" in sorted order; both have invalid colours.
	// validateNPCs iterates archetypes sorted by name, so alpha is checked first.
	const yaml = `
npcs:
  archetypes:
    zulu:  {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1, colors: [X]}
    alpha: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1, colors: [Y]}
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "npcs.archetypes.alpha.colors") {
		t.Errorf("error %q does not name alpha.colors (want first bad archetype in sorted order)", err)
	}
}
