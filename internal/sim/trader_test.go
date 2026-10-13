package sim_test

import (
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

func TestTraderPolicy_tradesWithNPCs(t *testing.T) {
	t.Parallel()
	cfg := tradesConfig(25)
	cfg.RunDays = 20

	result := runResult(t, cfg, sim.NewTraderPolicy())

	if got := result.Summary["trade_accepted"]; got == 0 {
		t.Errorf("trade_accepted = 0 over 20 days with 25 NPCs: the trader never completed a trade; summary: %v", result.Summary)
	}
	if got := result.Summary["trade_player_proposals"] + result.Summary["trade_npc_offers"]; got == 0 {
		t.Error("neither the trader nor the NPCs proposed a trade")
	}
}

func TestTraderPolicy_isDeterministic(t *testing.T) {
	t.Parallel()
	cfg := tradesConfig(25)

	first := runResult(t, cfg, sim.NewTraderPolicy())
	second := runResult(t, cfg, sim.NewTraderPolicy())

	if !reflect.DeepEqual(first, second) {
		t.Error("two runs of the trader with the same seed differ")
	}
}

func TestTraderPolicy_withoutTradesPlaysLikeTheCollector(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()

	trader := runResult(t, cfg, sim.NewTraderPolicy())
	collector := runResult(t, cfg, sim.NewCollectorPolicy())

	if !reflect.DeepEqual(trader, collector) {
		t.Error("the trader differs from the collector in a run where trades are off")
	}
}
