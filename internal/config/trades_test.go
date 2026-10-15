package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

func TestLoad_tradesDefaultToOffWithDocumentedValues(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := config.TradesConfig{Enabled: false, OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, HistorySize: 0, MaxOpenOffersPerDay: 20}
	if cfg.Trades != want {
		t.Errorf("Trades = %+v, want %+v", cfg.Trades, want)
	}
	if cfg.NPCs.SurplusHoldDays != 2 {
		t.Errorf("NPCs.SurplusHoldDays = %d, want 2", cfg.NPCs.SurplusHoldDays)
	}
	for _, name := range defaultArchetypeNames() {
		a := cfg.NPCs.Archetypes[name]
		// archivist intentionally reuses the targeted_collector behaviour (share=0 until integration)
		if a.Behaviour == name || name == "archivist" {
			continue
		}
		t.Errorf("archetype %q has behaviour %q, want one of the same name", name, a.Behaviour)
	}
	if got := cfg.NPCs.Archetypes["casual"].MarginPct; got != 0 {
		t.Errorf("casual margin = %d, want 0", got)
	}
	if got := cfg.NPCs.Archetypes["speculator"].MarginPct; got != 15 {
		t.Errorf("speculator margin = %d, want 15", got)
	}
}

func TestLoad_tradesOverrideMergesWithTheDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("npcs: {population: 10}\ntrades: {enabled: true, offer_expiry_days: 5}\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Trades.Enabled || cfg.Trades.OfferExpiryDays != 5 {
		t.Errorf("Trades = %+v, want enabled with 5 days of expiry", cfg.Trades)
	}
	if cfg.Trades.MaxCardsPerSide != 10 {
		t.Errorf("MaxCardsPerSide = %d, want the default 10 kept", cfg.Trades.MaxCardsPerSide)
	}
}

func TestLoad_tradesRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	const archetype = "npcs: {archetypes: {casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1, %s}}}"
	tests := []struct {
		name    string
		yaml    string
		wantKey string
	}{
		{"zero expiry", "trades: {offer_expiry_days: 0}", "trades.offer_expiry_days"},
		{"zero card cap", "trades: {max_cards_per_side: 0}", "trades.max_cards_per_side"},
		{"duplicate keep above a playset", "trades: {duplicate_keep: 5}", "trades.duplicate_keep"},
		{"duplicate keep of zero", "trades: {duplicate_keep: 0}", "trades.duplicate_keep"},
		{"negative history", "trades: {history_size: -1}", "trades.history_size"},
		{"negative daily offers", "trades: {max_open_offers_per_day: -1}", "trades.max_open_offers_per_day"},
		{"negative hold", "npcs: {surplus_hold_days: -1}", "npcs.surplus_hold_days"},
		{"unknown behaviour", strings.Replace(archetype, "%s", "behaviour: gambler", 1), "npcs.archetypes.casual.behaviour"},
		{"negative margin", strings.Replace(archetype, "%s", "behaviour: casual, margin_pct: -1", 1), "npcs.archetypes.casual.margin_pct"},
		{"offer chance above 100", strings.Replace(archetype, "%s", "behaviour: casual, offer_chance_pct: 101", 1), "npcs.archetypes.casual.offer_chance_pct"},
		{"negative offer chance", strings.Replace(archetype, "%s", "behaviour: casual, offer_chance_pct: -1", 1), "npcs.archetypes.casual.offer_chance_pct"},
		{"negative wish size", strings.Replace(archetype, "%s", "behaviour: casual, wish_size: -1", 1), "npcs.archetypes.casual.wish_size"},
		{"trades without a crowd", "trades: {enabled: true}", "trades.enabled"},
		{"trades without a hold", "npcs: {population: 5, surplus_hold_days: 0}\ntrades: {enabled: true}", "npcs.surplus_hold_days"},
		{
			name:    "trades with an archetype that has no behaviour",
			yaml:    "npcs: {population: 5, archetypes: {casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}}}\ntrades: {enabled: true}",
			wantKey: "npcs.archetypes.casual.behaviour",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tt.yaml))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantKey) {
				t.Errorf("error %q does not name %q", err, tt.wantKey)
			}
		})
	}
}

func TestLoad_tradesAcceptsAnArchetypeWithoutBehaviourWhileTradesAreOff(t *testing.T) {
	t.Parallel()
	const yaml = "npcs: {archetypes: {casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}}}"
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load: %v", err)
	}
}
