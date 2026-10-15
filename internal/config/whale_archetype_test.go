package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// TestWhaleArchetype_hasAllFiveColors verifies that the embedded whale.yaml carries
// exactly the five WUBRG colours as documented in ADR-0022.
func TestWhaleArchetype_hasAllFiveColors(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	whale, ok := cfg.NPCs.Archetypes["whale"]
	if !ok {
		t.Fatal("whale not found in embedded archetypes")
	}

	want := []string{"W", "U", "B", "R", "G"}
	if !reflect.DeepEqual(whale.Colors, want) {
		t.Errorf("Colors = %v, want %v", whale.Colors, want)
	}
}

// TestWhaleArchetype_validatesWithTradesOn verifies that the whale archetype (which now
// carries all five colours) passes full validation when trades are enabled.
func TestWhaleArchetype_validatesWithTradesOn(t *testing.T) {
	t.Parallel()

	const yaml = `
npcs:
  population: 10
trades:
  enabled: true
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load with trades enabled: %v — whale with all-five colours must still pass validation", err)
	}
}
