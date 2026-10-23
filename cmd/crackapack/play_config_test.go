package main

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The real game plays the product's base config with the play overlay: a Counter whose rares and
// mythics start out of stock and come only from its daily boosters, which then do bring some.
func TestPlayConfig_theRealGameHasAScarceCounter(t *testing.T) {
	t.Parallel()
	cfg, err := playConfig(false, demo.Dataset{})
	if err != nil {
		t.Fatalf("playConfig: %v", err)
	}
	ds := demo.Build()
	clock := playtest.NewClock()
	save := filepath.Join(t.TempDir(), "save.json")
	a := playArgs{
		catFS: ds.Files, scope: true, pricesFS: ds.Files, pricesName: "prices.json",
		cfg: cfg, now: clock.Now, seed: 42, savePath: save, store: play.NewFileStore(save), stdout: io.Discard,
	}
	game, _, err := startGame(a, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}

	rares := make([]domain.CardID, 0, 15)
	for _, set := range []string{"DEM", "DSP"} {
		b, err := game.Binder(set)
		if err != nil {
			t.Fatalf("Binder(%s): %v", set, err)
		}
		for _, s := range b.Slots {
			if s.Card.Rarity == domain.RarityRare {
				rares = append(rares, s.Card.ID)
			}
		}
	}
	quote := func(id domain.CardID) play.Quote {
		t.Helper()
		q, err := game.CounterQuote(id, domain.FinishNonFoil)
		if err != nil {
			t.Fatalf("CounterQuote(%s): %v", id, err)
		}
		return q
	}
	if q := quote("dem-c-000"); !q.InStock {
		t.Errorf("a common = %+v, want it in the initial stock", q)
	}
	for _, id := range append(rares, "dem-m-000") {
		if q := quote(id); q.InStock || !q.LastInStock.IsZero() {
			t.Fatalf("%s = %+v at the start, want never in stock", id, q)
		}
	}

	clock.Advance(3 * 24 * time.Hour)
	stocked := 0
	for _, id := range rares {
		if quote(id).InStock {
			stocked++
		}
	}
	if stocked == 0 {
		t.Errorf("no rare of the %d is in stock after three days of the Counter's boosters", len(rares))
	}
}

// The real game's NPCs trade on a market, as many as the play overlay sets, holding their surplus.
func TestPlayConfig_theRealGameHasAMarket(t *testing.T) {
	t.Parallel()
	cfg, err := playConfig(false, demo.Dataset{})
	if err != nil {
		t.Fatalf("playConfig: %v", err)
	}
	rules, err := counterRules(cfg, domain.PriceSnapshot{})
	if err != nil {
		t.Fatalf("counterRules: %v", err)
	}
	if rules.Market == nil || rules.Market.SurplusHoldDays < 1 {
		t.Fatalf("market = %+v, want one whose NPCs hold their surplus", rules.Market)
	}
	if rules.Crowd.Size != cfg.NPCs.Population || rules.Crowd.Size == 0 {
		t.Errorf("crowd of %d, want the %d NPCs of the config, more than none", rules.Crowd.Size, cfg.NPCs.Population)
	}
	for name, spec := range rules.Crowd.Archetypes {
		if spec.Share > 0 && spec.Behaviour == "" {
			t.Errorf("archetype %s does not trade", name)
		}
	}
}
