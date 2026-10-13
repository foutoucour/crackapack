package sim_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

// scaledPrices returns prices for cat with every price multiplied by factor.
func scaledPrices(cat domain.SetCatalogue, seed uint64, factor domain.USDCents) domain.PriceSnapshot {
	scaled := make(domain.PriceSnapshot)
	for id, price := range domaintest.NewPrices(cat, seed) {
		scaled[id] = price * factor
	}
	return scaled
}

// maxWaitOf runs a short simulation of openable and returns max_booster_wait_minutes, which
// follows from the highest energy cost among the openable sets. A nil reference leaves the engine
// on its default.
func maxWaitOf(t *testing.T, openable, reference []domain.SetCatalogue, prices domain.PriceSnapshot) float64 {
	t.Helper()
	cfg := smallConfig()
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	var opts []sim.Option
	if reference != nil {
		opts = append(opts, sim.WithReferenceCatalogues(reference))
	}
	e, err := sim.NewEngine(cfg, openable, nil, prices, sim.NewCollectorPolicy(), rng, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	result, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result.Summary["max_booster_wait_minutes"]
}

// With the reference {cheap, pricey} the median booster value is the cheap set's, so the cheap set
// costs 5 energy and the pricey one, worth about 100 times more, costs 12.
func TestNewEngine_energyCostIgnoresWhichSetsAreRun(t *testing.T) {
	t.Parallel()
	cheap := domaintest.NewRavnicaCatalogue()
	pricey := domaintest.NewClassicCatalogue()
	prices := domaintest.MergePrices(
		domaintest.NewPrices(cheap, 1),
		scaledPrices(pricey, 2, 100),
	)
	reference := []domain.SetCatalogue{cheap, pricey}

	const (
		waitCost5  = 50.0  // 5 reserve ticks at 10 minutes a tick
		waitCost12 = 160.0 // 10 reserve ticks + 2 savings points x 3 ticks, at 10 minutes a tick
	)
	tests := []struct {
		name      string
		openable  []domain.SetCatalogue
		reference []domain.SetCatalogue
		want      float64
	}{
		{"cheap set alone", []domain.SetCatalogue{cheap}, reference, waitCost5},
		{"pricey set alone", []domain.SetCatalogue{pricey}, reference, waitCost12},
		{"both sets together", []domain.SetCatalogue{cheap, pricey}, reference, waitCost12},
		{"without a reference a set alone is always tier x1", []domain.SetCatalogue{pricey}, nil, waitCost5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := maxWaitOf(t, tc.openable, tc.reference, prices); got != tc.want {
				t.Errorf("max_booster_wait_minutes = %v, want %v", got, tc.want)
			}
		})
	}
}

// A reference set that no price covers would be valued at fallback_price and collapse the median,
// moving every cost a tier without a sign, so the engine refuses it.
func TestNewEngine_errorWhenReferenceSetHasNoPrice(t *testing.T) {
	t.Parallel()
	openable := domaintest.NewRavnicaCatalogue()
	unpriced := domaintest.NewClassicCatalogue()
	prices := domaintest.NewPrices(openable, 1)
	cfg := smallConfig()
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))

	_, err := sim.NewEngine(cfg, []domain.SetCatalogue{openable}, nil, prices, sim.NewCollectorPolicy(), rng,
		sim.WithReferenceCatalogues([]domain.SetCatalogue{openable, unpriced}))

	if err == nil {
		t.Fatal("want an error for a reference set with no price, got nil")
	}
	if !strings.Contains(err.Error(), unpriced.Code) {
		t.Errorf("error does not name %s: %q", unpriced.Code, err.Error())
	}
	if !strings.Contains(err.Error(), "fetchdata") {
		t.Errorf("error does not say how to fetch the prices: %q", err.Error())
	}
}
