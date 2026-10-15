package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// TestArchivistArchetype_fileLoadsAndValidates verifies that the embedded archivist.yaml
// loads and that its values match the documented design:
//   - share=0 (no NPCs until the integration story)
//   - focus_set: oldest → FocusSetOldest
//   - margin_pct in [0, 5] (between casual 0 and targeted_collector 5)
//   - colours: [W] (white only)
//   - behaviour: "targeted_collector"
//   - no traits (archivist trait mixed in separately)
func TestArchivistArchetype_fileLoadsAndValidates(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	a, ok := cfg.NPCs.Archetypes["archivist"]
	if !ok {
		t.Fatal("archivist not found in embedded archetypes")
	}

	if a.Share != 0 {
		t.Errorf("Share = %d, want 0 (share-0 until integration story)", a.Share)
	}
	if a.FocusSet != config.FocusSetOldest {
		t.Errorf("FocusSet = %v, want FocusSetOldest", a.FocusSet)
	}
	if a.MarginPct < 0 || a.MarginPct > 5 {
		t.Errorf("MarginPct = %d, want in [0, 5]", a.MarginPct)
	}
	if len(a.Colors) != 1 || a.Colors[0] != "W" {
		t.Errorf("Colors = %v, want [W]", a.Colors)
	}
	if a.Behaviour != "targeted_collector" {
		t.Errorf("Behaviour = %q, want targeted_collector", a.Behaviour)
	}
	if len(a.Traits) != 0 {
		t.Errorf("Traits = %v, want empty (archivist trait is mixed in separately)", a.Traits)
	}
}

// TestArchivistArchetype_validatesWithTradesOn verifies that the archivist archetype passes
// the full validation path including trades-enabled checks.
func TestArchivistArchetype_validatesWithTradesOn(t *testing.T) {
	t.Parallel()

	// A population of zero with trades off is the default; use explicit trades-on config.
	// share=0 means archivist contributes no members; the other archetypes carry the crowd.
	const yaml = `
npcs:
  population: 10
trades:
  enabled: true
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load with trades enabled: %v — archivist archetype must pass all validations", err)
	}
}
