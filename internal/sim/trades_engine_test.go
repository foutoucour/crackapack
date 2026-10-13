package sim_test

import (
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// tradesConfig is npcConfig with trades on: every NPC is one archetype that trades, always
// proposes when it has a match and wishes for many cards, so a short run sees offers.
func tradesConfig(population int) config.Config {
	cfg := npcConfig(population, 6, 1)
	cfg.NPCs.SurplusHoldDays = 2
	a := cfg.NPCs.Archetypes["npc"]
	a.Behaviour = "casual"
	a.WishSize = 10
	a.OfferChancePct = 50
	cfg.NPCs.Archetypes["npc"] = a
	cfg.Trades = config.TradesConfig{Enabled: true, OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, MaxOpenOffersPerDay: 20}
	cfg.RunDays = 15
	return cfg
}

func TestRun_tradesOffLeaveACrowdRunUnchanged(t *testing.T) {
	t.Parallel()
	plain := npcConfig(4, 10, 1)
	configured := plain
	configured.NPCs = config.NPCConfig{
		Population:      4,
		SurplusHoldDays: 2,
		Archetypes: map[string]config.ArchetypeConfig{
			"npc": {Share: 1, MinBoostersPerDay: 10, MaxBoostersPerDay: 10, KeepCopies: 1, Behaviour: "casual", WishSize: 10, OfferChancePct: 100},
		},
	}

	want := runResult(t, plain, sim.NewCollectorPolicy())
	got := runResult(t, configured, sim.NewCollectorPolicy())

	if !reflect.DeepEqual(got, want) {
		t.Error("a run with trades.enabled false differs from the same run without trade settings")
	}
}

func TestRun_tradesReportTradeMetrics(t *testing.T) {
	t.Parallel()

	result := runResult(t, tradesConfig(20), openOnlyPolicy{})

	for _, d := range result.Days {
		if d.Trade == nil {
			t.Fatalf("day %d has no trade metrics", d.Day)
		}
		if _, ok := d.MoneySupply[domain.AccountTradeTopUp]; !ok {
			t.Errorf("day %d: the money supply lacks the %s account", d.Day, domain.AccountTradeTopUp)
		}
	}
	if got := result.Summary["trade_npc_offers"]; got == 0 {
		t.Error("trade_npc_offers = 0: with a crowd that always wishes and a player that collects duplicates, NPCs should have proposed")
	}
	if _, ok := result.Summary["trade_accepted"]; !ok {
		t.Error("the summary lacks trade_accepted")
	}
}

func TestRun_withoutTradesReportsNoTradeMetrics(t *testing.T) {
	t.Parallel()

	result := runResult(t, npcConfig(4, 10, 1), sim.NewCollectorPolicy())

	for _, d := range result.Days {
		if d.Trade != nil {
			t.Fatalf("day %d has trade metrics with trades off", d.Day)
		}
		if _, ok := d.MoneySupply[domain.AccountTradeTopUp]; ok {
			t.Fatalf("day %d lists the top-up account with trades off", d.Day)
		}
	}
}

func TestRun_tradesDoNotMoveThePlayersBoosterDraws(t *testing.T) {
	t.Parallel()
	off := npcConfig(20, 6, 1)
	on := tradesConfig(20)
	on.RunDays = off.RunDays

	without := runResult(t, off, openOnlyPolicy{})
	with := runResult(t, on, openOnlyPolicy{})

	for i := range without.Days {
		if without.Days[i].BoostersOpened != with.Days[i].BoostersOpened || without.Days[i].CollectionUSD != with.Days[i].CollectionUSD {
			t.Fatalf("day %d: the player's draws changed when trades were enabled: %d boosters / %d cents without, %d / %d with",
				i+1, without.Days[i].BoostersOpened, without.Days[i].CollectionUSD, with.Days[i].BoostersOpened, with.Days[i].CollectionUSD)
		}
	}
}

func TestNewEngine_tradesNeedACrowd(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.Trades = config.TradesConfig{Enabled: true, OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1}

	cats := testCatalogues()
	_, err := sim.NewEngine(cfg, cats, nil, mergedPrices(cats, cfg.Seed), sim.NewIdlePolicy(), newRNG(cfg.Seed))

	if err == nil {
		t.Error("NewEngine accepted trades without an NPC crowd")
	}
}
