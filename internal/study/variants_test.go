package study_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/study"
)

func TestLoadVariants_readsNamesNotesAndOverrides(t *testing.T) {
	t.Parallel()
	yaml := `
variants:
  - name: baseline
    note: the game's world
  - name: bigger-crowd
    overrides:
      - {path: npcs.population, value: 24}
      - {path: npcs.surplus_hold_days, value: 14}
`

	got, err := study.LoadVariants(strings.NewReader(yaml))

	if err != nil {
		t.Fatalf("LoadVariants: %v", err)
	}
	if len(got) != 2 || got[0].Name != "baseline" || got[0].Note != "the game's world" || len(got[0].Overrides) != 0 {
		t.Fatalf("variants = %+v, want baseline with its note and no override first", got)
	}
	if o := got[1].Overrides; len(o) != 2 || o[0].Path != "npcs.population" || o[0].Value != 24 || o[1].Path != "npcs.surplus_hold_days" {
		t.Errorf("overrides = %+v, want the two of bigger-crowd in order", o)
	}
}

func TestLoadVariants_rejectsWhatItCannotRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"no variant", "variants: []\n", "no variant"},
		{"empty name", "variants:\n  - note: nameless\n", "name"},
		{"duplicate name", "variants:\n  - name: a\n  - name: a\n", "twice"},
		{"unknown field", "variants:\n  - name: a\n    colour: red\n", "colour"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := study.LoadVariants(strings.NewReader(tt.yaml))

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("LoadVariants = %v, want an error mentioning %q", err, tt.want)
			}
		})
	}
}
