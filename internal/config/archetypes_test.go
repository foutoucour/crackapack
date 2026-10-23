package config_test

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/config"
)

// minimalValidArchetype returns YAML bytes for a minimal valid archetype with the given name.
// It sets every scalar field so that validateArchetypeSupply and validateArchetypeTrade pass.
func minimalValidArchetype(behaviour string) []byte {
	b := "share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\nfocus_set: false\nbehaviour: "
	if behaviour != "" {
		b += behaviour + "\n"
	} else {
		b += "\"\"\n"
	}
	b += "wish_size: 0\nmargin_pct: 0\noffer_chance_pct: 0\n"
	return []byte(b)
}

func TestLoadArchetypes_nameComesFromTheFileName(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"alpha.yaml": {Data: []byte("share: 5\nmin_boosters_per_day: 2\nmax_boosters_per_day: 4\nkeep_copies: 2\nfocus_set: true\nbehaviour: casual\nwish_size: 3\nmargin_pct: 0\noffer_chance_pct: 1\n")},
		"beta.yaml":  {Data: minimalValidArchetype("")},
	}
	got, err := config.LoadArchetypes(fsys)
	if err != nil {
		t.Fatalf("LoadArchetypes: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	alpha, ok := got["alpha"]
	if !ok {
		t.Fatal("key 'alpha' not found; name must come from file base name")
	}
	if alpha.Share != 5 {
		t.Errorf("alpha.Share = %d, want 5", alpha.Share)
	}
	if alpha.FocusSet != config.FocusSetRandom {
		t.Errorf("alpha.FocusSet = %v, want FocusSetRandom", alpha.FocusSet)
	}
	if _, ok := got["beta"]; !ok {
		t.Error("key 'beta' not found; name must come from file base name")
	}
}

func TestLoadArchetypes_rejects(t *testing.T) {
	t.Parallel()

	validData := minimalValidArchetype("")

	tests := []struct {
		name    string
		fsys    fstest.MapFS
		wantErr string
	}{
		{
			name: "unknown field",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\nunknown_field: bad\n")},
			},
			wantErr: "x.yaml",
		},
		{
			name: "name key is an unknown field",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\nname: x\n")},
			},
			wantErr: "x.yaml",
		},
		{
			name: "unknown trait",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\ntraits: [not_a_real_trait]\n")},
			},
			wantErr: "x.yaml",
		},
		{
			name: "unknown colour",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\ncolors: [X]\n")},
			},
			wantErr: "x.yaml",
		},
		{
			name: "unknown behaviour",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\nbehaviour: not_registered\n")},
			},
			wantErr: "x.yaml",
		},
		{
			name: "bad name uppercase first letter",
			fsys: fstest.MapFS{
				"Casual.yaml": {Data: validData},
			},
			wantErr: "Casual.yaml",
		},
		{
			name: "bad name hyphen",
			fsys: fstest.MapFS{
				"my-arch.yaml": {Data: validData},
			},
			wantErr: "my-arch.yaml",
		},
		{
			name: "bad name dot",
			fsys: fstest.MapFS{
				"a.b.yaml": {Data: validData},
			},
			wantErr: "a.b.yaml",
		},
		{
			name: "bad name starts with digit",
			fsys: fstest.MapFS{
				"1x.yaml": {Data: validData},
			},
			wantErr: "1x.yaml",
		},
		{
			name: "non-.yaml extension .yml",
			fsys: fstest.MapFS{
				"casual.yml": {Data: validData},
			},
			wantErr: "casual.yml",
		},
		{
			name: "non-.yaml extension none",
			fsys: fstest.MapFS{
				"README": {Data: []byte("not yaml")},
			},
			wantErr: "README",
		},
		{
			name: "directory entry",
			fsys: fstest.MapFS{
				"subdir/placeholder.yaml": {Data: validData},
			},
			wantErr: "subdir",
		},
		{
			name: "empty file",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("")},
			},
			wantErr: "x.yaml",
		},
		{
			name:    "empty directory",
			fsys:    fstest.MapFS{},
			wantErr: "no archetype files",
		},
		{
			name: "float in int field",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("share: 1.5\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\n")},
			},
			wantErr: "x.yaml",
		},
		{
			name: "keep_copies above playset",
			fsys: fstest.MapFS{
				"x.yaml": {Data: []byte("share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 5\n")},
			},
			wantErr: "x.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.LoadArchetypes(tt.fsys)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadWithArchetypes_usesOnlyTheInjectedFS(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"only.yaml": {Data: minimalValidArchetype("casual")},
	}
	cfg, err := config.LoadWithArchetypes(strings.NewReader("{}"), fsys)
	if err != nil {
		t.Fatalf("LoadWithArchetypes: %v", err)
	}
	if len(cfg.NPCs.Archetypes) != 1 {
		t.Errorf("len(Archetypes) = %d, want 1: injected FS has one file", len(cfg.NPCs.Archetypes))
	}
	if _, ok := cfg.NPCs.Archetypes["only"]; !ok {
		t.Errorf("archetype 'only' not found; got keys: %v", archetypeKeys(cfg))
	}
}

func TestLoadWithArchetypes_overrideReplacesTheEntryWhole(t *testing.T) {
	t.Parallel()

	// casual in the injected FS has wish_size 9.
	casualData := []byte("share: 1\nmin_boosters_per_day: 1\nmax_boosters_per_day: 1\nkeep_copies: 1\nfocus_set: false\nbehaviour: casual\nwish_size: 9\nmargin_pct: 0\noffer_chance_pct: 0\n")
	fsys := fstest.MapFS{
		"casual.yaml": {Data: casualData},
		"extra.yaml":  {Data: minimalValidArchetype("casual")},
	}

	// The user override for casual lacks wish_size, so it becomes 0 (replace-the-whole-entry rule).
	// new_name is a new archetype added by the user.
	const yaml = `
npcs:
  archetypes:
    casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
    new_name: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}
`
	cfg, err := config.LoadWithArchetypes(strings.NewReader(yaml), fsys)
	if err != nil {
		t.Fatalf("LoadWithArchetypes: %v", err)
	}

	// The embedded casual (wish_size 9) is replaced whole; the override lacks wish_size → 0.
	if got := cfg.NPCs.Archetypes["casual"].WishSize; got != 0 {
		t.Errorf("casual.WishSize = %d, want 0 (user override replaces the entry whole)", got)
	}

	// A MapFS-only archetype not mentioned in the user YAML is kept.
	if _, ok := cfg.NPCs.Archetypes["extra"]; !ok {
		t.Error("archetype 'extra' not found; MapFS-only archetypes must be kept")
	}

	// A new name from the user YAML is added.
	if _, ok := cfg.NPCs.Archetypes["new_name"]; !ok {
		t.Error("archetype 'new_name' not found; user YAML can add archetypes")
	}
}

func TestLoad_embeddedArchetypesKeepTheirPreviousValues(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Exact values from the embedded YAML files.
	// Traits and TraitParams are nil because the embedded files omit those fields.
	// Colours: only whale carries [W U B R G]; the others omit colors, leaving it nil.
	want := map[string]config.ArchetypeConfig{
		"casual": {
			Share: 50, MinBoostersPerDay: 4, MaxBoostersPerDay: 8, KeepCopies: 1,
			FocusSet: config.FocusSetNone, Behaviour: "casual", WishSize: 5, MarginPct: 0, OfferChancePct: 2,
			ResponseDelayHours: 0,
		},
		"playset_hunter": {
			Share: 15, MinBoostersPerDay: 3, MaxBoostersPerDay: 6, KeepCopies: 4,
			FocusSet: config.FocusSetNone, Behaviour: "playset_hunter", WishSize: 5, MarginPct: 5, OfferChancePct: 2,
			ResponseDelayHours: 4,
		},
		"targeted_collector": {
			Share: 20, MinBoostersPerDay: 2, MaxBoostersPerDay: 5, KeepCopies: 1,
			FocusSet: config.FocusSetRandom, Behaviour: "targeted_collector", WishSize: 5, MarginPct: 5, OfferChancePct: 2,
			ResponseDelayHours: 8,
		},
		"speculator": {
			Share: 10, MinBoostersPerDay: 2, MaxBoostersPerDay: 4, KeepCopies: 2,
			FocusSet: config.FocusSetNone, Behaviour: "speculator", WishSize: 5, MarginPct: 15, OfferChancePct: 1,
			ResponseDelayHours: 0,
		},
		"whale": {
			Share: 5, MinBoostersPerDay: 30, MaxBoostersPerDay: 50, KeepCopies: 1,
			FocusSet: config.FocusSetNone, Behaviour: "whale", WishSize: 3, MarginPct: 10, OfferChancePct: 1,
			ResponseDelayHours: 12,
			Colors:             []string{"W", "U", "B", "R", "G"},
		},
	}

	// Subset check: later stories may add more embedded files.
	for name, wantA := range want {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := cfg.NPCs.Archetypes[name]
			if !ok {
				t.Fatalf("archetype %q not found in embedded set", name)
			}
			if !reflect.DeepEqual(got, wantA) {
				t.Errorf("archetype %q: got %+v, want %+v", name, got, wantA)
			}
		})
	}
}

func TestLoad_everyShippedArchetypeValidates(t *testing.T) {
	t.Parallel()
	// Load with a non-zero population and trades enabled to exercise all validation paths.
	const yaml = `
npcs:
  population: 10
trades:
  enabled: true
`
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for name, a := range cfg.NPCs.Archetypes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if a.Behaviour == "" {
				t.Errorf("archetype %q has no behaviour; shipped archetypes must name a behaviour", name)
			}
		})
	}
}

// The market wants answers at once from some NPCs and the next day from others, see
// docs/simulation.md#the-market.
func TestLoad_shippedArchetypesAnswerAtOnceAndAfterADay(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fastest, slowest := 1<<31, -1
	for _, a := range cfg.NPCs.Archetypes {
		fastest, slowest = min(fastest, a.ResponseDelayHours), max(slowest, a.ResponseDelayHours)
	}
	if fastest != 0 || slowest < 24 {
		t.Errorf("response delays run from %dh to %dh, want from 0h to at least 24h", fastest, slowest)
	}
}

// archetypeKeys returns the map keys of cfg.NPCs.Archetypes for use in failure messages.
func archetypeKeys(cfg config.Config) []string {
	keys := make([]string, 0, len(cfg.NPCs.Archetypes))
	for k := range cfg.NPCs.Archetypes {
		keys = append(keys, k)
	}
	return keys
}

// Ensure fstest.MapFS satisfies fs.FS so the compiler catches mismatches.
var _ fs.FS = (fstest.MapFS)(nil)
