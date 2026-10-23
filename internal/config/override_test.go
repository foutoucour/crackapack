package config_test

import (
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v3"

	"github.com/foutoucour/crackapack/internal/config"
)

func loadDefaults(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestApplyOverrides_setsTheNamedKeys(t *testing.T) {
	t.Parallel()

	got, err := config.ApplyOverrides(loadDefaults(t), []config.Override{
		{Path: "counter_buy_pct", Value: 150},
		{Path: "npcs.archetypes.casual.keep_copies", Value: 3},
		{Path: "energy.cost_table[1].cost", Value: 6},
		{Path: "trades.offer_expiry_days", Value: 4},
	})
	if err != nil {
		t.Fatalf("ApplyOverrides: %v", err)
	}

	if got.CounterBuyPct != 150 {
		t.Errorf("counter_buy_pct = %d, want 150", got.CounterBuyPct)
	}
	casual := got.NPCs.Archetypes["casual"]
	if casual.KeepCopies != 3 {
		t.Errorf("casual keep_copies = %d, want 3", casual.KeepCopies)
	}
	if casual.Share == 0 || casual.Behaviour == "" {
		t.Errorf("casual lost its other fields: %+v", casual)
	}
	if got.Energy.CostTable[1].Cost != 6 {
		t.Errorf("cost_table[1].cost = %d, want 6", got.Energy.CostTable[1].Cost)
	}
	if got.Trades.OfferExpiryDays != 4 {
		t.Errorf("offer_expiry_days = %d, want 4", got.Trades.OfferExpiryDays)
	}
}

func TestApplyOverrides_leavesTheBaseUntouched(t *testing.T) {
	t.Parallel()

	base := loadDefaults(t)
	if _, err := config.ApplyOverrides(base, []config.Override{{Path: "npcs.archetypes.casual.keep_copies", Value: 3}}); err != nil {
		t.Fatalf("ApplyOverrides: %v", err)
	}
	if got := base.NPCs.Archetypes["casual"].KeepCopies; got != 1 {
		t.Errorf("base casual keep_copies = %d, want 1: the override leaked into the base", got)
	}
}

func TestApplyOverrides_rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		override config.Override
		want     string
	}{
		{"unknown key", config.Override{Path: "counter_sell_pcts", Value: 50}, "counter_sell_pcts"},
		{"unknown archetype", config.Override{Path: "npcs.archetypes.nobody.share", Value: 1}, "nobody"},
		{"index out of range", config.Override{Path: "energy.cost_table[9].cost", Value: 1}, "cost_table[9]"},
		{"a mapping is not a number", config.Override{Path: "energy", Value: 1}, "energy"},
		{"fraction on a whole number", config.Override{Path: "counter_sell_pct", Value: 55.5}, "counter_sell_pct"},
		{"invalid combination", config.Override{Path: "counter_sell_pct", Value: 200}, "counter_sell_pct"},
		{"value out of bounds", config.Override{Path: "sessions_per_day", Value: 100000}, "sessions_per_day"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.ApplyOverrides(loadDefaults(t), []config.Override{tc.override})
			if err == nil {
				t.Fatal("ApplyOverrides succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestConfig_marshalsToWhatLoadAccepts pins that a Config written as YAML loads back to the same
// Config, so a tuned config can be saved and handed to simulate.
func TestConfig_marshalsToWhatLoadAccepts(t *testing.T) {
	t.Parallel()

	base := loadDefaults(t)
	tuned, err := config.ApplyOverrides(base, []config.Override{{Path: "npcs.archetypes.casual.keep_copies", Value: 2}})
	if err != nil {
		t.Fatalf("ApplyOverrides: %v", err)
	}
	// An archetype that opens only the oldest set exercises FocusSet's encoding.
	archetype := tuned.NPCs.Archetypes["casual"]
	archetype.FocusSet = config.FocusSetOldest
	tuned.NPCs.Archetypes["casual"] = archetype

	data, err := yaml.Marshal(tuned)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	reloaded, err := config.Load(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Load of marshalled config: %v", err)
	}
	if got := reloaded.NPCs.Archetypes["casual"]; got.KeepCopies != 2 || got.FocusSet != config.FocusSetOldest {
		t.Errorf("reloaded casual = %+v, want keep_copies 2 and focus_set oldest", got)
	}
	if reloaded.CoinRate != tuned.CoinRate || reloaded.Seed != tuned.Seed {
		t.Errorf("reloaded scalars differ: %+v", reloaded)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Parallel()
	got, err := config.LoadOverrides(strings.NewReader("- {path: counter_shelf_days, value: 28}\n- {path: initial_stock.rare, value: 0}\n"))
	if err != nil {
		t.Fatalf("LoadOverrides: %v", err)
	}
	want := []config.Override{{Path: "counter_shelf_days", Value: 28}, {Path: "initial_stock.rare", Value: 0}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("LoadOverrides = %+v, want %+v", got, want)
	}
	for _, bad := range []string{"- {path: x, value: 1, extra: 2}", "path: x"} {
		if _, err := config.LoadOverrides(strings.NewReader(bad)); err == nil {
			t.Errorf("LoadOverrides(%q) went through", bad)
		}
	}
}
