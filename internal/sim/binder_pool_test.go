package sim_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/sim"
)

// viewRecorder is a policy that does nothing but keep the views it is given.
type viewRecorder struct{ views []sim.WorldView }

func (r *viewRecorder) RunSession(s sim.Session) { r.views = append(r.views, s.View()) }

// timeSpiralRun runs Time Spiral, with the timeshifted set as its pool, under policy.
func timeSpiralRun(t *testing.T, cfg config.Config, policy sim.PlayerPolicy) sim.Result {
	t.Helper()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	prices := mergedPrices([]domain.SetCatalogue{tsp, tsb}, cfg.Seed)
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	e, err := sim.NewEngine(cfg, []domain.SetCatalogue{tsp}, []domain.SetCatalogue{tsb}, prices, policy, rng)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	res, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

func nonBasicCards(cat domain.SetCatalogue) int {
	n := 0
	for _, c := range cat.Cards {
		if !c.IsBasicLand {
			n++
		}
	}
	return n
}

// Time Spiral's binder holds the timeshifted cards its boosters draw, see
// docs/core-features.md#collection-and-binders: the simulation must count the same cards as the
// collection page, or the two disagree about when a binder is complete.
func TestEngine_theBinderCountsTheTimeshiftedCardsTheBoostersDraw(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	want := nonBasicCards(tsp) + len(domain.DrawableCards(tsp, tsb))

	res := timeSpiralRun(t, smallConfig(), sim.NewIdlePolicy())

	if got := res.Days[0].SetProgress["TSP"].Total; got != want {
		t.Errorf("the binder total = %d, want %d: the set's cards plus the drawable timeshifted ones", got, want)
	}
}

func TestEngine_policiesSeeTheTimeshiftedCardsAsPartOfTheBinder(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	rec := &viewRecorder{}

	timeSpiralRun(t, smallConfig(), rec)

	if len(rec.views) == 0 {
		t.Fatal("the policy never ran")
	}
	v := rec.views[0]
	if got, want := v.SetProgress["TSP"].Total, nonBasicCards(tsp)+len(tsb.Cards); got != want {
		t.Errorf("the progress a policy sees totals %d, want %d", got, want)
	}
	var cat domain.SetCatalogue
	for _, c := range v.Catalogues {
		if c.Code == "TSP" {
			cat = c
		}
	}
	for id := range tsb.Cards {
		if _, ok := cat.Cards[id]; !ok {
			t.Errorf("timeshifted card %s is not among the cards a policy collects", id)
		}
	}
	if len(v.Catalogues) != 1 {
		t.Errorf("a policy sees %d catalogues, want only the one openable set: the pool is not a set to open", len(v.Catalogues))
	}
}

func TestEngine_aCollectorWithCoinsCompletesTheWholeBinderTimeshiftedCardsIncluded(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	cfg := smallConfig()
	cfg.RunDays = 30
	cfg.WelcomeBonus = 1_000_000
	cfg.DailyReward = 100_000
	want := nonBasicCards(tsp) + len(domain.DrawableCards(tsp, tsb))

	res := timeSpiralRun(t, cfg, sim.NewCollectorPolicy())

	last := res.Days[len(res.Days)-1].SetProgress["TSP"]
	if last.Total != want || last.Owned != last.Total {
		t.Errorf("after %d days the binder is %d/%d, want it complete at %d: the collector must buy the timeshifted cards too", cfg.RunDays, last.Owned, last.Total, want)
	}
	if got := res.Summary["days_to_100pct.tsp"]; got > float64(cfg.RunDays) {
		t.Errorf("days_to_100pct.tsp = %v, want a day within the %d-day run", got, cfg.RunDays)
	}
}
