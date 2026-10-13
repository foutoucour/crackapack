package sim_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

func TestOutputs_tradesAddTheirColumnsOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	with := runResult(t, tradesConfig(10), openOnlyPolicy{})
	without := runResult(t, npcConfig(10, 6, 1), openOnlyPolicy{})

	var withJSON, withCSV, withoutJSON, withoutCSV bytes.Buffer
	for _, c := range []struct {
		buf    *bytes.Buffer
		result sim.Result
		csv    bool
	}{{&withJSON, with, false}, {&withCSV, with, true}, {&withoutJSON, without, false}, {&withoutCSV, without, true}} {
		var err error
		if c.csv {
			err = sim.WriteCSV(c.buf, c.result)
		} else {
			err = sim.WriteJSON(c.buf, c.result)
		}
		if err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if !strings.Contains(withJSON.String(), `"Trade"`) {
		t.Error("result.json lacks the Trade block with trades on")
	}
	header, _, _ := strings.Cut(withCSV.String(), "\n")
	for _, col := range []string{"trade_topup", "trade_player_proposals", "trade_npc_offers", "trade_accepted", "trade_countered", "trade_refused", "trade_expired", "trade_volume_coins", "trade_topup_coins"} {
		if !strings.Contains(header, col) {
			t.Errorf("days.csv header lacks %q: %s", col, header)
		}
	}
	if strings.Contains(withoutJSON.String(), `"Trade"`) || strings.Contains(withoutCSV.String(), "trade_") {
		t.Error("output mentions trades without trades: runs without trades must not change")
	}
}

func TestCompare_refusesATradeRunAgainstARunWithoutTrades(t *testing.T) {
	t.Parallel()
	with := runResult(t, tradesConfig(10), openOnlyPolicy{})
	cfg := npcConfig(10, 6, 1)
	cfg.RunDays = with.Days[len(with.Days)-1].Day
	without := runResult(t, cfg, openOnlyPolicy{})

	if _, err := sim.Compare(with, without); err == nil {
		t.Error("Compare accepted a run with trades against a run without")
	}
}

func TestCompare_comparesTwoTradeRuns(t *testing.T) {
	t.Parallel()
	a := runResult(t, tradesConfig(10), openOnlyPolicy{})
	b := runResult(t, tradesConfig(10), openOnlyPolicy{})

	if _, err := sim.Compare(a, b); err != nil {
		t.Errorf("Compare: %v", err)
	}
}
