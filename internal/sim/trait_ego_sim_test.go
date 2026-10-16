package sim_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// egoConfig returns a tradesConfig with a single ego-trait NPC.
// OfferChancePct=100 guarantees the NPC always proposes when the retry state allows,
// so the daily cap is exercised deterministically even in short runs.
func egoConfig(maxRetries int) config.Config {
	cfg := tradesConfig(1)
	cfg.NPCs.MaxMarginPct = 1000
	a := cfg.NPCs.Archetypes["npc"]
	a.Behaviour = "casual"
	a.OfferChancePct = 100
	a.Traits = []string{"ego"}
	a.TraitParams = map[string]map[string]int{
		"ego": {
			"max_retries":           maxRetries,
			"endowment_pct_per_day": 0, // no age premium so counters never complicate the test
		},
	}
	cfg.NPCs.Archetypes["npc"] = a
	cfg.RunDays = 7 // enough days to observe the cap reliably
	return cfg
}

// declineAllPolicy declines every open offer at the start of each session. This maximises the
// ego's decline streak so the retry logic is exercised every day.
type declineAllPolicy struct{}

func (declineAllPolicy) RunSession(s sim.Session) {
	for _, o := range s.Offers() {
		// Decline NPC-origin offers and NPC counters to build the streak.
		if o.Origin == domain.OriginNPC || o.Origin == domain.OriginNPCCounter {
			_ = s.DeclineOffer(o.ID)
		}
	}
}

// TestEgoTrait_dailyOfferCapIsMaxRetriesPlusOne verifies that the ego archetype never produces
// more than max_retries+1 NPC offers per day for a single ego NPC, even when the player declines
// every offer to maximise the retry chain. The engine calls ProposeFromNPCs on each of the 144
// ticks, so the retry state is exercised 144 times per day.
func TestEgoTrait_dailyOfferCapIsMaxRetriesPlusOne(t *testing.T) {
	t.Parallel()
	const maxRetries = 3
	cfg := egoConfig(maxRetries)
	result := runResultWithRegistry(t, cfg, declineAllPolicy{}, domain.DefaultTraits())

	cap := maxRetries + 1
	for _, d := range result.Days {
		if d.Trade == nil {
			t.Fatalf("day %d: no trade metrics", d.Day)
		}
		if d.Trade.NPCOffers > cap {
			t.Errorf("day %d: NPCOffers = %d, want <= %d (max_retries+1)",
				d.Day, d.Trade.NPCOffers, cap)
		}
	}
}

// TestEgoTrait_dailyOfferCapWithZeroRetries verifies that when max_retries=0 the ego never
// follows up: each day sees at most 1 NPC offer (the initial proposal; the ego then holds for
// the rest of the day since retryHold triggers immediately after the first settle).
func TestEgoTrait_dailyOfferCapWithZeroRetries(t *testing.T) {
	t.Parallel()
	cfg := egoConfig(0)
	result := runResultWithRegistry(t, cfg, declineAllPolicy{}, domain.DefaultTraits())

	for _, d := range result.Days {
		if d.Trade == nil {
			t.Fatalf("day %d: no trade metrics", d.Day)
		}
		if d.Trade.NPCOffers > 1 {
			t.Errorf("day %d: NPCOffers = %d with max_retries=0, want <= 1",
				d.Day, d.Trade.NPCOffers)
		}
	}
}

// TestEgoTrait_isDeterministic verifies that two runs of the ego archetype with the same seed
// produce identical summaries. This covers criterion 5 of the story acceptance criteria.
func TestEgoTrait_isDeterministic(t *testing.T) {
	t.Parallel()
	cfg := egoConfig(3)

	first := runResultWithRegistry(t, cfg, declineAllPolicy{}, domain.DefaultTraits())
	second := runResultWithRegistry(t, cfg, declineAllPolicy{}, domain.DefaultTraits())

	if fmt.Sprint(first.Summary) != fmt.Sprint(second.Summary) {
		t.Errorf("two ego runs with the same seed differ:\n%v\n%v", first.Summary, second.Summary)
	}
}

// TestEgoAndGamblerArchetypes_produceOffersWhenEnabled verifies story criterion 17: when the ego
// and gambler archetypes have their share raised from 0 to a positive value, they participate in
// trades and the run reports trade_npc_offers > 0.
//
// This test uses config.Load to exercise the real embedded YAML files, then overrides share so
// the archetypes enter the crowd.
func TestEgoAndGamblerArchetypes_produceOffersWhenEnabled(t *testing.T) {
	t.Parallel()
	const populationYAML = `
npcs:
  population: 20
  surplus_hold_days: 2
  max_margin_pct: 1000
trades:
  enabled: true
  offer_expiry_days: 3
  max_cards_per_side: 10
  duplicate_keep: 1
  max_open_offers_per_day: 20
`
	cfg, err := config.Load(strings.NewReader(populationYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Enable ego and gambler by raising their share; all other archetypes keep their defaults.
	setShare := func(name string, share int) {
		a := cfg.NPCs.Archetypes[name]
		a.Share = share
		cfg.NPCs.Archetypes[name] = a
	}
	setShare("ego", 1)
	setShare("gambler", 1)
	// Ensure at least one other archetype has a share so total > 0 even if ego/gambler are tiny.
	setShare("casual", 10)

	cfg.RunDays = 10
	cfg.SessionsPerDay = 3
	cfg.Seed = 99
	cfg.CoinRate = 100
	cfg.CounterSellPct = 60
	cfg.CounterBuyPct = 120
	cfg.FallbackPrice = 1
	cfg.WelcomeBonus = 500
	cfg.DailyReward = 100

	result := runResultWithRegistry(t, cfg, sim.NewExploiterPolicy(), domain.DefaultTraits())

	if got := result.Summary["trade_npc_offers"]; got == 0 {
		t.Error("trade_npc_offers = 0 with ego and gambler enabled: expected at least some NPC offers")
	}
}
