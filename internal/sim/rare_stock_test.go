package sim_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

// TestRun_rareStockMetrics reads the two metrics of ADR-0042 off a run where nobody touches the
// Counter, so its stock is the initial stock: rares hold two copies each and mythics none.
func TestRun_rareStockMetrics(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewRavnicaCatalogue()
	cat.Cards["rare-stock-mythic"] = domain.Card{
		ID:       "rare-stock-mythic",
		Name:     "Stock Mythic",
		Number:   "999",
		Rarity:   domain.RarityMythic,
		Finishes: []domain.Finish{domain.FinishNonFoil},
	}
	var rares, mythics int
	for _, card := range cat.Cards {
		switch {
		case card.IsBasicLand:
		case card.Rarity == domain.RarityRare:
			rares++
		case card.Rarity == domain.RarityMythic:
			mythics++
		}
	}
	if rares == 0 || mythics == 0 {
		t.Fatalf("fixture needs rares and mythics, got %d and %d", rares, mythics)
	}

	cfg := smallConfig()
	cfg.RunDays = 1
	cfg.InitialStock = map[domain.Rarity]int{domain.RarityRare: 2}
	cats := []domain.SetCatalogue{cat}
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	engine, err := sim.NewEngine(cfg, cats, nil, mergedPrices(cats, cfg.Seed), sim.NewIdlePolicy(), rng)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	printings := float64(rares + mythics)
	wantOut := 100 * float64(mythics) / printings
	wantAvg := 2 * float64(rares) / printings
	if got := result.Summary["rare_out_of_stock_pct"]; math.Abs(got-wantOut) > 1e-9 {
		t.Errorf("rare_out_of_stock_pct = %v, want %v", got, wantOut)
	}
	if got := result.Summary["rare_stock_per_printing"]; math.Abs(got-wantAvg) > 1e-9 {
		t.Errorf("rare_stock_per_printing = %v, want %v", got, wantAvg)
	}
}
