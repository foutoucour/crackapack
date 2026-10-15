package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// focusSetFromYAML is a convenience that parses a single-archetype YAML snippet and
// returns the FocusSet field, or fails the test if parsing fails.
func focusSetFromYAML(t *testing.T, yaml string) config.FocusSet {
	t.Helper()
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, ok := cfg.NPCs.Archetypes["x"]
	if !ok {
		t.Fatal("archetype 'x' not found")
	}
	return a.FocusSet
}

func archYAML(focusSetValue string) string {
	return "npcs:\n  archetypes:\n    x: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1, focus_set: " + focusSetValue + "}\n"
}

// TestFocusSet_falseDecodesAsNone verifies that YAML false (and its aliases) map to FocusSetNone.
func TestFocusSet_falseDecodesAsNone(t *testing.T) {
	t.Parallel()

	for _, val := range []string{"false", "no", "off"} {
		val := val
		t.Run(val, func(t *testing.T) {
			t.Parallel()
			got := focusSetFromYAML(t, archYAML(val))
			if got != config.FocusSetNone {
				t.Errorf("focus_set: %q → %v, want FocusSetNone", val, got)
			}
		})
	}
}

// TestFocusSet_trueDecodesAsRandom verifies that YAML true (and its aliases) map to FocusSetRandom.
func TestFocusSet_trueDecodesAsRandom(t *testing.T) {
	t.Parallel()

	for _, val := range []string{"true", "yes", "on"} {
		val := val
		t.Run(val, func(t *testing.T) {
			t.Parallel()
			got := focusSetFromYAML(t, archYAML(val))
			if got != config.FocusSetRandom {
				t.Errorf("focus_set: %q → %v, want FocusSetRandom", val, got)
			}
		})
	}
}

// TestFocusSet_oldestStringDecodesAsOldest verifies that the bare word "oldest" and its
// quoted form both decode to FocusSetOldest.
func TestFocusSet_oldestStringDecodesAsOldest(t *testing.T) {
	t.Parallel()

	for _, val := range []string{"oldest", `"oldest"`} {
		val := val
		t.Run(val, func(t *testing.T) {
			t.Parallel()
			got := focusSetFromYAML(t, archYAML(val))
			if got != config.FocusSetOldest {
				t.Errorf("focus_set: %v → %v, want FocusSetOldest", val, got)
			}
		})
	}
}

// TestFocusSet_omittedDecodesAsNone verifies that omitting focus_set leaves FocusSetNone
// (the zero value).
func TestFocusSet_omittedDecodesAsNone(t *testing.T) {
	t.Parallel()

	const yaml = "npcs:\n  archetypes:\n    x: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}\n"
	got := focusSetFromYAML(t, yaml)
	if got != config.FocusSetNone {
		t.Errorf("omitted focus_set → %v, want FocusSetNone", got)
	}
}

// TestFocusSet_rejectsUnknownStrings verifies that values other than the three accepted
// forms produce a load error.
func TestFocusSet_rejectsUnknownStrings(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, val string }{
		{"newest", "newest"},
		{"integer 1", "1"},
		{"integer 0", "0"},
		{"empty string", `""`},
		{"random word", "maybe"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(archYAML(tt.val)))
			if err == nil {
				t.Errorf("focus_set: %q: want error, got nil", tt.val)
			}
		})
	}
}

// TestFocusSet_rejectsListAndMap verifies that complex YAML nodes (lists, maps) are rejected.
func TestFocusSet_rejectsListAndMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "list",
			yaml: "npcs:\n  archetypes:\n    x: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1, focus_set: [RAV]}\n",
		},
		{
			name: "map",
			yaml: "npcs:\n  archetypes:\n    x: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1, focus_set: {set: RAV}}\n",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tt.yaml))
			if err == nil {
				t.Errorf("focus_set %s: want error, got nil", tt.name)
			}
		})
	}
}

// TestFocusSet_shippedFilesHaveExpectedValues verifies that every embedded archetype file
// decodes focus_set to the expected mode.
func TestFocusSet_shippedFilesHaveExpectedValues(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Focus set modes per shipped file.
	wantFocusSet := map[string]config.FocusSet{
		"casual":             config.FocusSetNone,
		"playset_hunter":     config.FocusSetNone,
		"speculator":         config.FocusSetNone,
		"targeted_collector": config.FocusSetRandom,
		"whale":              config.FocusSetNone,
		"archivist":          config.FocusSetOldest,
	}

	for name, want := range wantFocusSet {
		name, want := name, want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, ok := cfg.NPCs.Archetypes[name]
			if !ok {
				t.Fatalf("archetype %q not found in embedded set", name)
			}
			if a.FocusSet != want {
				t.Errorf("FocusSet = %v, want %v", a.FocusSet, want)
			}
		})
	}
}
