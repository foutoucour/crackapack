package sim_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// supplyConfig is smallConfig with an empty Counter that a fixed supply stocks.
func supplyConfig(boostersPerDay int) config.Config {
	cfg := smallConfig()
	cfg.InitialStock = map[domain.Rarity]int{}
	cfg.CounterSupply = config.CounterSupplyConfig{Mode: config.CounterSupplyFixed, BoostersPerDay: boostersPerDay}
	return cfg
}

func TestRun_CounterSupplyOpensItsDailyRhythm(t *testing.T) {
	t.Parallel()
	cfg := supplyConfig(10)

	result := runResult(t, cfg, sim.NewIdlePolicy())

	for _, d := range result.Days {
		if d.Counter == nil || d.Counter.BoostersOpened != 10 {
			t.Errorf("day %d: Counter metrics = %+v, want 10 boosters opened", d.Day, d.Counter)
		}
	}
	if got, want := result.Summary["counter_boosters_opened"], float64(10*cfg.RunDays); got != want {
		t.Errorf("counter_boosters_opened = %v, want %v", got, want)
	}
	last := result.Days[len(result.Days)-1]
	var stock int
	for _, qty := range last.StockByRarity {
		stock += qty
	}
	if float64(stock) != result.Summary["counter_cards_opened"] || stock == 0 {
		t.Errorf("stock = %d, want the %v cards the Counter opened", stock, result.Summary["counter_cards_opened"])
	}
}

func TestRun_withoutCounterSupplyReportsNoCounterMetrics(t *testing.T) {
	t.Parallel()

	result := runResult(t, smallConfig(), sim.NewIdlePolicy())

	for _, d := range result.Days {
		if d.Counter != nil {
			t.Fatalf("day %d: Counter metrics present in a run without a supply", d.Day)
		}
	}
	for key := range result.Summary {
		if strings.HasPrefix(key, "counter_") {
			t.Errorf("summary has %q in a run without a supply", key)
		}
	}
}

func TestRun_CounterSupplyDoesNotMoveThePlayersDraws(t *testing.T) {
	t.Parallel()

	without := runResult(t, supplyConfig(1), openOnlyPolicy{})
	cfg := supplyConfig(40)
	with := runResult(t, cfg, openOnlyPolicy{})

	if without.Days[len(without.Days)-1].BoostersOpened == 0 {
		t.Fatal("the player opened nothing, the test cannot show anything")
	}
	for i := range without.Days {
		if without.Days[i].CollectionUSD != with.Days[i].CollectionUSD {
			t.Errorf("day %d: player collection %d against %d: the supply moved the player's draws",
				without.Days[i].Day, without.Days[i].CollectionUSD, with.Days[i].CollectionUSD)
		}
	}
}

func TestRun_CounterSupplyRunIsReproducibleFromTheSeed(t *testing.T) {
	t.Parallel()

	a := runResult(t, supplyConfig(8), sim.NewIdlePolicy())
	b := runResult(t, supplyConfig(8), sim.NewIdlePolicy())

	if a.Summary["counter_cards_opened"] != b.Summary["counter_cards_opened"] {
		t.Errorf("cards opened %v and %v differ for the same seed", a.Summary["counter_cards_opened"], b.Summary["counter_cards_opened"])
	}
}

func TestWriteCSV_countersSupplyColumnsAppearOnlyWithASupply(t *testing.T) {
	t.Parallel()
	with := runResult(t, supplyConfig(5), sim.NewIdlePolicy())
	without := runResult(t, smallConfig(), sim.NewIdlePolicy())

	header := func(r sim.Result) string {
		var buf bytes.Buffer
		if err := sim.WriteCSV(&buf, r); err != nil {
			t.Fatalf("WriteCSV: %v", err)
		}
		return strings.SplitN(buf.String(), "\n", 2)[0]
	}

	if h := header(with); !strings.Contains(h, "counter_boosters_opened") || !strings.Contains(h, "counter_cards_opened") {
		t.Errorf("header with a supply = %q, want the two counter columns", h)
	}
	if h := header(without); strings.Contains(h, "counter_boosters_opened") || strings.Contains(h, "counter_cards_opened") {
		t.Errorf("header without a supply = %q, want no column of the Counter's openings", h)
	}
}

func TestCompare_refusesARunWithACounterSupplyAgainstOneWithout(t *testing.T) {
	t.Parallel()
	with := runResult(t, supplyConfig(5), sim.NewIdlePolicy())
	without := runResult(t, smallConfig(), sim.NewIdlePolicy())

	if _, err := sim.Compare(with, without); err == nil {
		t.Error("Compare succeeded, want an error: the columns differ")
	}
}
