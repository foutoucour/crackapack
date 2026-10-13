package sim_test

import (
	"bytes"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

// npcConfig is smallConfig plus a crowd of one archetype with a fixed rhythm, and a Counter
// that starts empty so that its stock can only come from the NPCs.
func npcConfig(population, boostersPerDay, keep int) config.Config {
	cfg := smallConfig()
	cfg.InitialStock = map[domain.Rarity]int{}
	cfg.NPCs = config.NPCConfig{
		Population: population,
		Archetypes: map[string]config.ArchetypeConfig{
			"npc": {Share: 1, MinBoostersPerDay: boostersPerDay, MaxBoostersPerDay: boostersPerDay, KeepCopies: keep},
		},
	}
	return cfg
}

// openOnlyPolicy opens boosters while it can afford one and does nothing else, so the player's
// draws depend on nothing but its own random stream.
type openOnlyPolicy struct{}

func (openOnlyPolicy) RunSession(s sim.Session) {
	for {
		v := s.View()
		for _, cat := range v.Catalogues {
			if v.EnergyTotal >= v.BoosterCosts[cat.Code] && s.OpenBooster(cat.Code) == nil {
				break
			}
		}
		if s.View().EnergyTotal == v.EnergyTotal {
			return
		}
	}
}

func testCatalogues() []domain.SetCatalogue {
	return []domain.SetCatalogue{domaintest.NewRavnicaCatalogue(), domaintest.NewClassicCatalogue()}
}

func newRNG(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed)) }

func runResult(t *testing.T, cfg config.Config, policy sim.PlayerPolicy) sim.Result {
	t.Helper()
	result, err := buildEngine(t, cfg, policy).Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result
}

func TestRun_idlePlayerDoesNothing(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()

	result := runResult(t, cfg, sim.NewIdlePolicy())

	for _, d := range result.Days {
		if d.BoostersOpened != 0 || d.CardsSold != 0 || d.CardsBought != 0 {
			t.Errorf("day %d: idle player acted: %+v", d.Day, d)
		}
	}
	want := float64(cfg.WelcomeBonus) + float64(cfg.RunDays)*float64(cfg.DailyReward)
	if got := result.Summary["final_balance"]; got != want {
		t.Errorf("final_balance = %v, want %v: only the welcome bonus and daily rewards", got, want)
	}
}

func TestRun_withoutNPCsReportsNoNPCMetrics(t *testing.T) {
	t.Parallel()

	result := runResult(t, smallConfig(), sim.NewCollectorPolicy())

	for _, d := range result.Days {
		if d.NPC != nil {
			t.Fatalf("day %d has NPC metrics without a crowd: %+v", d.Day, d.NPC)
		}
	}
	for name := range result.Summary {
		if strings.HasPrefix(name, "npc_") {
			t.Errorf("summary has %q without a crowd", name)
		}
	}
	var jsonOut, csvOut bytes.Buffer
	if err := sim.WriteJSON(&jsonOut, result); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if err := sim.WriteCSV(&csvOut, result); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	if strings.Contains(jsonOut.String(), `"NPC"`) || strings.Contains(csvOut.String(), "npc_") {
		t.Error("output mentions NPCs without a crowd: runs without NPCs must not change")
	}
}

func TestRun_NPCsOpenTheirDailyRhythmEveryDay(t *testing.T) {
	t.Parallel()

	result := runResult(t, npcConfig(3, 10, 1), sim.NewIdlePolicy())

	for _, d := range result.Days {
		if d.NPC == nil {
			t.Fatalf("day %d has no NPC metrics", d.Day)
		}
		if want := 3 * 10; d.NPC.BoostersOpened != want {
			t.Errorf("day %d: NPCs opened %d boosters, want %d", d.Day, d.NPC.BoostersOpened, want)
		}
	}
}

func TestRun_NPCSurplusIsTheCounterStock(t *testing.T) {
	t.Parallel()

	result := runResult(t, npcConfig(4, 20, 1), sim.NewIdlePolicy())

	var cumulativeSold int
	for _, d := range result.Days {
		cumulativeSold += d.NPC.CardsSold
		var stock int
		for _, qty := range d.StockByRarity {
			stock += qty
		}
		if stock != cumulativeSold {
			t.Errorf("day %d: Counter stock = %d, want the %d cards NPCs sold so far", d.Day, stock, cumulativeSold)
		}
	}
	last := result.Days[len(result.Days)-1]
	if cumulativeSold == 0 || last.NPC.CardsHeld == 0 {
		t.Errorf("sold %d and held %d at the end: want NPCs to both sell and keep cards", cumulativeSold, last.NPC.CardsHeld)
	}
}

func TestRun_NPCsDoNotMoveThePlayersDraws(t *testing.T) {
	t.Parallel()

	without := runResult(t, npcConfig(0, 10, 1), openOnlyPolicy{})
	with := runResult(t, npcConfig(5, 10, 1), openOnlyPolicy{})

	if without.Days[len(without.Days)-1].BoostersOpened == 0 {
		t.Fatal("the player opened nothing, the test cannot show anything")
	}
	for i := range without.Days {
		if without.Days[i].CollectionUSD != with.Days[i].CollectionUSD {
			t.Errorf("day %d: player collection %d without NPCs, %d with: the crowd moved the player's draws",
				without.Days[i].Day, without.Days[i].CollectionUSD, with.Days[i].CollectionUSD)
		}
	}
}

func TestRun_NPCRunIsReproducibleFromTheSeed(t *testing.T) {
	t.Parallel()

	encode := func() string {
		var buf bytes.Buffer
		if err := sim.WriteJSON(&buf, runResult(t, npcConfig(6, 8, 2), sim.NewCollectorPolicy())); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		return buf.String()
	}

	if a, b := encode(), encode(); a != b {
		t.Error("same seed and config produced different output")
	}
}

func TestRun_summaryTotalsTheCrowd(t *testing.T) {
	t.Parallel()

	result := runResult(t, npcConfig(4, 20, 1), sim.NewIdlePolicy())

	var opened, sold float64
	for _, d := range result.Days {
		opened += float64(d.NPC.BoostersOpened)
		sold += float64(d.NPC.CardsSold)
	}
	last := result.Days[len(result.Days)-1]
	want := map[string]float64{
		"npc_boosters_opened": opened,
		"npc_cards_sold":      sold,
		"npc_cards_held":      float64(last.NPC.CardsHeld),
	}
	for name, w := range want {
		if got, ok := result.Summary[name]; !ok || got != w {
			t.Errorf("summary[%q] = %v (present %t), want %v", name, got, ok, w)
		}
	}
}

func TestNewEngine_rejectsACrowdWithoutArchetypes(t *testing.T) {
	t.Parallel()
	cfg := npcConfig(5, 10, 1)
	cfg.NPCs.Archetypes = nil

	_, err := sim.NewEngine(cfg, testCatalogues(), nil, mergedPrices(testCatalogues(), cfg.Seed), sim.NewIdlePolicy(), newRNG(cfg.Seed))

	if err == nil {
		t.Fatal("want error, got nil")
	}
}
