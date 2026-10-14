package sim_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

func newSheetCardsEngine(supplementary []domain.SetCatalogue) error {
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	cfg := smallConfig()
	prices := mergedPrices([]domain.SetCatalogue{tsp, tsb}, cfg.Seed)
	_ = supplementary
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	_, err := sim.NewEngine(cfg, []domain.SetCatalogue{tsp}, supplementary, prices, sim.NewIdlePolicy(), rng)
	return err
}

// The booster sheets of Time Spiral reference timeshifted cards that live in TSB. Without that
// pool the Counter does not know them, and a run fails late and obscurely.
func TestNewEngine_namesAMissingSupplementaryPool(t *testing.T) {
	t.Parallel()

	err := newSheetCardsEngine(nil)

	if err == nil || !strings.Contains(err.Error(), "TSP") || !strings.Contains(err.Error(), "--supplementary") {
		t.Errorf("error = %v, want one naming the set TSP and the --supplementary flag", err)
	}
}

func TestNewEngine_acceptsTheSupplementaryPoolThatHoldsTheSheetCards(t *testing.T) {
	t.Parallel()
	_, tsb := domaintest.NewTimeSpiralCatalogues()

	if err := newSheetCardsEngine([]domain.SetCatalogue{tsb}); err != nil {
		t.Errorf("NewEngine with TSB supplied: %v", err)
	}
}
