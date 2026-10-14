package demo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// TestBuild_structure verifies the demo dataset passes through real domain loaders
// and meets the structural requirements described in docs/simulation.md.
func TestBuild_structure(t *testing.T) {
	t.Parallel()
	ds := demo.Build()

	// Load DEM via LoadSetCatalogue (must have a booster section).
	dem, err := domain.LoadSetCatalogue(ds.Files, "DEM.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue DEM: %v", err)
	}

	// Load DSP via LoadSetCatalogue.
	dsp, err := domain.LoadSetCatalogue(ds.Files, "DSP.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue DSP: %v", err)
	}

	// Load DST via LoadCardPool (no booster section required).
	dst, err := domain.LoadCardPool(ds.Files, "DST.json")
	if err != nil {
		t.Fatalf("LoadCardPool DST: %v", err)
	}

	// Load prices.
	prices, err := domain.LoadPriceSnapshot(ds.Files, "prices.json")
	if err != nil {
		t.Fatalf("LoadPriceSnapshot: %v", err)
	}

	// --- DEM structural checks ---
	t.Run("DEM_card_counts", func(t *testing.T) {
		t.Parallel()
		counts := rarityCount(dem)
		// 80 commons + 5 basics = 85 common-rarity cards.
		if counts[domain.RarityCommon] < 80 {
			t.Errorf("DEM: commons = %d, want >= 80", counts[domain.RarityCommon])
		}
		if counts[domain.RarityUncommon] < 30 {
			t.Errorf("DEM: uncommons = %d, want >= 30", counts[domain.RarityUncommon])
		}
		if counts[domain.RarityRare] < 15 {
			t.Errorf("DEM: rares = %d, want >= 15", counts[domain.RarityRare])
		}
		if counts[domain.RarityMythic] < 3 {
			t.Errorf("DEM: mythics = %d, want >= 3", counts[domain.RarityMythic])
		}
	})

	t.Run("DEM_rare_sheet_holds_mythics", func(t *testing.T) {
		t.Parallel()
		for _, sc := range dem.Sheets["rare"].Cards {
			if dem.Cards[sc.ID].Rarity == domain.RarityMythic {
				return
			}
		}
		t.Error("DEM: the rare sheet holds no mythic, so no booster can open one")
	})

	t.Run("DEM_booster_sizes", func(t *testing.T) {
		t.Parallel()
		checkBoosterSizes(t, "DEM", dem, 15)
	})

	t.Run("DEM_booster_weight_sum", func(t *testing.T) {
		t.Parallel()
		checkBoosterWeightSum(t, "DEM", dem, 1000)
	})

	// --- DSP structural checks ---
	t.Run("DSP_card_counts", func(t *testing.T) {
		t.Parallel()
		counts := rarityCount(dsp)
		if counts[domain.RarityCommon] < 60 {
			t.Errorf("DSP: commons = %d, want >= 60", counts[domain.RarityCommon])
		}
		if counts[domain.RarityUncommon] < 20 {
			t.Errorf("DSP: uncommons = %d, want >= 20", counts[domain.RarityUncommon])
		}
	})

	t.Run("DSP_booster_sizes", func(t *testing.T) {
		t.Parallel()
		checkBoosterSizes(t, "DSP", dsp, 15)
	})

	t.Run("DSP_booster_weight_sum", func(t *testing.T) {
		t.Parallel()
		checkBoosterWeightSum(t, "DSP", dsp, 1000)
	})

	// --- DST structural checks ---
	t.Run("DST_specials", func(t *testing.T) {
		t.Parallel()
		if len(dst.Cards) < 20 {
			t.Errorf("DST: want >= 20 cards, got %d", len(dst.Cards))
		}
		for _, c := range dst.Cards {
			if c.Rarity != domain.RaritySpecial {
				t.Errorf("DST card %q: rarity = %q, want %q", c.ID, c.Rarity, domain.RaritySpecial)
			}
		}
	})

	// --- Price distribution checks ---
	t.Run("prices_distribution", func(t *testing.T) {
		t.Parallel()
		checkPriceDistribution(t, []domain.SetCatalogue{dem, dsp, dst}, prices)
	})

	// --- SetCodes and SupplementaryCodes ---
	t.Run("dataset_codes", func(t *testing.T) {
		t.Parallel()
		if len(ds.SetCodes) != 2 || ds.SetCodes[0] != "DEM" || ds.SetCodes[1] != "DSP" {
			t.Errorf("SetCodes = %v, want [DEM DSP]", ds.SetCodes)
		}
		if len(ds.SupplementaryCodes) != 1 || ds.SupplementaryCodes[0] != "DST" {
			t.Errorf("SupplementaryCodes = %v, want [DST]", ds.SupplementaryCodes)
		}
	})

	// --- Config parses ---
	t.Run("config_valid", func(t *testing.T) {
		t.Parallel()
		cfg, err := config.Load(strings.NewReader(ds.ConfigYAML))
		if err != nil {
			t.Fatalf("config.Load: %v", err)
		}
		if cfg.RunDays != 90 {
			t.Errorf("run_days = %d, want 90", cfg.RunDays)
		}
		if len(cfg.SweepRates) == 0 {
			t.Error("sweep_rates is empty")
		}
		if len(cfg.Targets) == 0 {
			t.Error("targets is empty")
		}
	})
}

// TestBuild_sweepRealism runs the demo sweep and asserts three properties:
//  1. Not all days_to_100pct_cu.* metrics equal 1 at the lowest rate (milestones take time).
//  2. At least one rate passes all calibration targets.
//  3. At least one rate fails at least one calibration target.
//
// The determinism assertion uses a 5-day abbreviated run to keep the test fast.
func TestBuild_sweepRealism(t *testing.T) {
	// Not t.Parallel: this test runs multiple simulation passes and is the slow one.
	ds := demo.Build()
	cfg, err := config.Load(strings.NewReader(ds.ConfigYAML))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cats, supps, prices := loadDemoInputs(t, ds)

	makeRun := func(c config.Config) func(domain.CoinRate) (sim.Result, error) {
		return func(rate domain.CoinRate) (sim.Result, error) {
			rc := c
			rc.CoinRate = rate
			rng := rand.New(rand.NewPCG(c.Seed, c.Seed))
			engine, err := sim.NewEngine(rc, cats, supps, prices, sim.NewCollectorPolicy(), rng)
			if err != nil {
				return sim.Result{}, fmt.Errorf("NewEngine: %w", err)
			}
			return engine.Run()
		}
	}

	sr, err := sim.Sweep(cfg.SweepRates, makeRun(cfg), cfg.Targets)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	// 1. Milestones not all day 1 at the lowest configured rate.
	lowestRate := cfg.SweepRates[0]
	for _, rr := range sr.Rates {
		if rr.Rate != lowestRate {
			continue
		}
		for key, val := range rr.Result.Summary {
			if strings.HasPrefix(key, "days_to_100pct_cu.") && val <= 1 {
				t.Errorf("rate %d: %s = %.0f, want > 1 (binder should not complete on day 1)", rr.Rate, key, val)
			}
		}
	}

	// 2. At least one rate passes ALL targets.
	anyAllPass := false
	for _, rr := range sr.Rates {
		if rr.Pass {
			anyAllPass = true
			break
		}
	}
	if !anyAllPass {
		t.Error("no rate passes all calibration targets; adjust targets so some rates can pass")
	}

	// 3. At least one rate FAILS at least one target.
	anyFail := false
	for _, rr := range sr.Rates {
		for _, o := range rr.Outcomes {
			if !o.Pass {
				anyFail = true
				break
			}
		}
		if anyFail {
			break
		}
	}
	if !anyFail {
		t.Error("every rate passes every target; adjust targets so some rates can fail")
	}
}

// TestBuild_crowd verifies the demo ships NPCs of every archetype and that they act: a crowd
// that never opens a booster, or an archetype with no member, would leave the NPC path unexercised.
func TestBuild_crowd(t *testing.T) {
	t.Parallel()
	ds := demo.Build()
	cfg, err := config.Load(strings.NewReader(ds.ConfigYAML))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	var totalShare int
	for _, a := range cfg.NPCs.Archetypes {
		totalShare += a.Share
	}
	for name, a := range cfg.NPCs.Archetypes {
		if members := cfg.NPCs.Population * a.Share / totalShare; members < 1 {
			t.Errorf("archetype %q has %d members in the demo crowd of %d, want at least 1", name, members, cfg.NPCs.Population)
		}
	}

	cfg.RunDays = 3
	cats, supps, prices := loadDemoInputs(t, ds)
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	engine, err := sim.NewEngine(cfg, cats, supps, prices, sim.NewIdlePolicy(), rng)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, metric := range []string{"npc_boosters_opened", "npc_cards_sold", "npc_cards_held"} {
		if result.Summary[metric] <= 0 {
			t.Errorf("%s = %v, want > 0", metric, result.Summary[metric])
		}
	}
}

// TestBuild_determinism verifies two runs on a 5-day abbreviated config produce
// byte-identical Summary output. The determinism check uses a short run to keep
// test time low; the full 90-day sweep is exercised by TestBuild_sweepRealism.
func TestBuild_determinism(t *testing.T) {
	t.Parallel()
	ds := demo.Build()
	cfg, err := config.Load(strings.NewReader(ds.ConfigYAML))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.RunDays = 5

	cats, supps, prices := loadDemoInputs(t, ds)
	rate := cfg.SweepRates[0]

	summary1 := runOnce(t, cfg, rate, cats, supps, prices)
	summary2 := runOnce(t, cfg, rate, cats, supps, prices)

	b1, _ := json.Marshal(summary1)
	b2, _ := json.Marshal(summary2)
	if !bytes.Equal(b1, b2) {
		t.Errorf("two runs produced different summaries:\nrun1: %s\nrun2: %s", b1, b2)
	}
}

// TestBuild_demoExercisesTheCounterAsASpecialPlayer checks that the demo config turns on the
// Counter's supply and treasury, and that a run reports what they did.
func TestBuild_demoExercisesTheCounterAsASpecialPlayer(t *testing.T) {
	t.Parallel()
	ds := demo.Build()
	cfg, err := config.Load(strings.NewReader(ds.ConfigYAML))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.CounterSupply.Mode != config.CounterSupplyFixed || !cfg.CounterBudget.Enabled {
		t.Fatalf("demo config has supply %q and budget enabled %v, want a fixed supply and a budget",
			cfg.CounterSupply.Mode, cfg.CounterBudget.Enabled)
	}
	cfg.RunDays = 5

	cats, supps, prices := loadDemoInputs(t, ds)
	summary := runOnce(t, cfg, cfg.SweepRates[0], cats, supps, prices)

	if summary["counter_boosters_opened"] <= 0 {
		t.Errorf("counter_boosters_opened = %v, want the Counter to open boosters", summary["counter_boosters_opened"])
	}
	if summary["counter_treasury"] <= 0 {
		t.Errorf("counter_treasury = %v, want a treasury that still holds coins", summary["counter_treasury"])
	}
}

// --- helpers ---

func rarityCount(cat domain.SetCatalogue) map[domain.Rarity]int {
	m := make(map[domain.Rarity]int)
	for _, c := range cat.Cards {
		m[c.Rarity]++
	}
	return m
}

func checkBoosterSizes(t *testing.T, code string, cat domain.SetCatalogue, wantSize int) {
	t.Helper()
	for i, opt := range cat.BoosterOptions {
		total := 0
		for _, n := range opt.Contents {
			total += n
		}
		if total != wantSize {
			t.Errorf("%s booster option %d: %d cards, want %d", code, i, total, wantSize)
		}
	}
}

func checkBoosterWeightSum(t *testing.T, code string, cat domain.SetCatalogue, wantSum int) {
	t.Helper()
	total := 0
	for _, opt := range cat.BoosterOptions {
		total += opt.Weight
	}
	if total != wantSum {
		t.Errorf("%s booster option weights sum to %d, want %d", code, total, wantSum)
	}
}

func checkPriceDistribution(t *testing.T, cats []domain.SetCatalogue, prices domain.PriceSnapshot) {
	t.Helper()

	// Collect common and rare prices.
	var commonPrices []domain.USDCents
	var rarePrices []domain.USDCents
	totalNonBasic := 0
	priced := 0

	for _, cat := range cats {
		for id, card := range cat.Cards {
			if card.IsBasicLand {
				continue
			}
			totalNonBasic++
			if p, ok := prices[id]; ok {
				priced++
				switch card.Rarity {
				case domain.RarityCommon:
					commonPrices = append(commonPrices, p)
				case domain.RarityRare:
					rarePrices = append(rarePrices, p)
				case domain.RarityUncommon, domain.RaritySpecial, domain.RarityMythic:
					// not checked individually
				}
			}
		}
	}

	// > 90% of non-basic cards must have a price.
	if totalNonBasic > 0 && float64(priced)/float64(totalNonBasic) < 0.9 {
		t.Errorf("only %.1f%% of non-basic cards have a price (want > 90%%)",
			100*float64(priced)/float64(totalNonBasic))
	}

	// At least one card must be unpriced (exercises fallback path).
	if priced == totalNonBasic {
		t.Error("all non-basic cards have a price; want at least one unpriced")
	}

	// Median common price must be in [3, 30] cents.
	if len(commonPrices) > 0 {
		median := medianInts(commonPrices)
		if median < 3 || median > 30 {
			t.Errorf("median common price = %d cents, want in [3, 30]", median)
		}
	}

	// At least one rare must exceed $20 (2000 cents) — chase rare.
	hasChase := false
	for _, p := range rarePrices {
		if p > 2000 {
			hasChase = true
			break
		}
	}
	if !hasChase {
		t.Error("no rare priced above 2000 cents ($20); want at least one chase rare")
	}
}

func medianInts(prices []domain.USDCents) domain.USDCents {
	if len(prices) == 0 {
		return 0
	}
	sorted := make([]domain.USDCents, len(prices))
	copy(sorted, prices)
	// Simple insertion sort (small slice).
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted[len(sorted)/2]
}

func loadDemoInputs(t *testing.T, ds demo.Dataset) (cats, supps []domain.SetCatalogue, prices domain.PriceSnapshot) {
	t.Helper()
	for _, code := range ds.SetCodes {
		cat, err := domain.LoadSetCatalogue(ds.Files, code+".json")
		if err != nil {
			t.Fatalf("LoadSetCatalogue %s: %v", code, err)
		}
		cats = append(cats, cat)
	}
	for _, code := range ds.SupplementaryCodes {
		supp, err := domain.LoadCardPool(ds.Files, code+".json")
		if err != nil {
			t.Fatalf("LoadCardPool %s: %v", code, err)
		}
		supps = append(supps, supp)
	}
	var err error
	prices, err = domain.LoadPriceSnapshot(ds.Files, "prices.json")
	if err != nil {
		t.Fatalf("LoadPriceSnapshot: %v", err)
	}
	return cats, supps, prices
}

func runOnce(t *testing.T, cfg config.Config, rate domain.CoinRate, cats, supps []domain.SetCatalogue, prices domain.PriceSnapshot) sim.Summary {
	t.Helper()
	cfg.CoinRate = rate
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	engine, err := sim.NewEngine(cfg, cats, supps, prices, sim.NewCollectorPolicy(), rng)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result.Summary
}
