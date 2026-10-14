package domain_test

// booster_layouts_test.go: statistical and structural tests for the Time Spiral (TSP/TSB)
// and Planar Chaos (PLC) booster layouts. Both layouts are expressed entirely through
// BoosterOptions and named Sheets — no new generator type is needed.
//
// All statistical tests use 100 000 packs with a fixed seed. Tolerance justification is
// stated per assertion; all thresholds are ≥ 3σ so that spurious failure probability < 0.3%.

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// ---------------------------------------------------------------------------
// Time Spiral — structural tests
// ---------------------------------------------------------------------------

// TestClassicGenerator_timeSpiralReturns15Cards verifies that every TSP booster
// contains exactly 15 cards across both booster options.
func TestClassicGenerator_timeSpiralReturns15Cards(t *testing.T) {
	t.Parallel()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	gen := domain.NewClassicGenerator()
	rng := newRNG(20)

	for i := range 100 {
		b, err := gen.Generate(tsp, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}
		if len(b.Cards) != 15 {
			t.Errorf("pack %d: len(Cards) = %d, want 15", i, len(b.Cards))
		}
	}
}

// TestClassicGenerator_timeSpiralTimeshiftedSlotInEveryPack verifies that every TSP
// booster contains exactly one non-foil timeshifted card (identified by the tsp-tsb-
// ID prefix). A foil pack may additionally carry a foil tsp-tsb- card from the foil
// sheet; that does not count toward the mandatory non-foil slot.
func TestClassicGenerator_timeSpiralTimeshiftedSlotInEveryPack(t *testing.T) {
	t.Parallel()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	gen := domain.NewClassicGenerator()
	rng := newRNG(21)

	for i := range 500 {
		b, err := gen.Generate(tsp, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}
		nonFoilTimeshifted := 0
		for _, ck := range b.Cards {
			if ck.Finish == domain.FinishNonFoil && strings.HasPrefix(string(ck.ID), "tsp-tsb-") {
				nonFoilTimeshifted++
			}
		}
		if nonFoilTimeshifted != 1 {
			t.Errorf("pack %d: non-foil timeshifted count = %d, want 1", i, nonFoilTimeshifted)
		}
	}
}

// ---------------------------------------------------------------------------
// Time Spiral — statistical tests (100 000 packs, seed 22)
//
// Tolerance justification (n = 100 000):
//   foil-pack rate p = 0.225, σ = sqrt(0.225×0.775/100000) ≈ 0.00132 → ±0.5% ≈ 3.8σ
//
// Foil rarity fractions (n ≈ 22 500 foil packs):
//   p = 0.50 (common),      σ ≈ 0.00333 → ±1.0% ≈ 3.0σ
//   p = 0.25 (uncommon),    σ ≈ 0.00289 → ±1.0% ≈ 3.5σ
//   p = 0.125 (rare),       σ ≈ 0.00220 → ±1.0% ≈ 4.5σ
//   p = 0.125 (timeshifted),σ ≈ 0.00220 → ±1.0% ≈ 4.5σ
// ---------------------------------------------------------------------------

// TestClassicGenerator_statisticsTimeSpiral runs 100 000 TSP packs and asserts:
//  1. foil-pack rate = 22.5% ± 0.5%
//  2. every pack has exactly 1 non-foil timeshifted card
//  3. all five Magic colours are present among non-foil common-sheet draws in every pack
//  4. the foil card's rarity distribution matches 50/25/12.5/12.5 within ±1%
func TestClassicGenerator_statisticsTimeSpiral(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	gen := domain.NewClassicGenerator()
	rng := newRNG(22)

	var foilPacks int
	foilByGroup := make(map[string]int) // "common", "uncommon", "rare", "timeshifted"

	for i := range statPacks {
		b, err := gen.Generate(tsp, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}

		// --- timeshifted slot: exactly 1 non-foil tsp-tsb- card ---
		nonFoilTS := 0
		for _, ck := range b.Cards {
			if ck.Finish == domain.FinishNonFoil && strings.HasPrefix(string(ck.ID), "tsp-tsb-") {
				nonFoilTS++
			}
		}
		if nonFoilTS != 1 {
			t.Errorf("pack %d: non-foil timeshifted count = %d, want 1", i, nonFoilTS)
		}

		// --- colour balance: non-foil common-sheet draws must cover all 5 colours ---
		colours := make(map[string]bool, 5)
		for _, ck := range b.Cards {
			if ck.Finish != domain.FinishNonFoil {
				continue
			}
			c, ok := tsp.Cards[ck.ID]
			if !ok || c.Rarity != domain.RarityCommon {
				continue
			}
			for _, col := range c.Colors {
				colours[col] = true
			}
		}
		for _, want := range [5]string{"W", "U", "B", "R", "G"} {
			if !colours[want] {
				t.Errorf("pack %d: colour %q missing from non-foil common draws", i, want)
			}
		}

		// --- foil rate and foil rarity distribution ---
		for _, ck := range b.Cards {
			if ck.Finish != domain.FinishFoil {
				continue
			}
			foilPacks++
			switch {
			case strings.HasPrefix(string(ck.ID), "tsp-tsb-"):
				foilByGroup["timeshifted"]++
			default:
				// Look up in TSP cards; TSB cards were already handled above.
				if _, ok := tsb.Cards[ck.ID]; ok {
					foilByGroup["timeshifted"]++
					continue
				}
				c, ok := tsp.Cards[ck.ID]
				if !ok {
					continue
				}
				switch c.Rarity {
				case domain.RarityCommon:
					foilByGroup["common"]++
				case domain.RarityUncommon:
					foilByGroup["uncommon"]++
				case domain.RarityRare, domain.RarityMythic:
					foilByGroup["rare"]++
				case domain.RaritySpecial:
					// TSP has no special-rarity cards in its main set; unreachable
					// in this fixture, but required for exhaustive switch coverage.
					foilByGroup["timeshifted"]++
				}
			}
		}
	}

	const tolFoilRate = 0.005 // ±0.5%
	const tolRarity = 0.010   // ±1.0%

	assertRate(t, "TSP foil-pack rate", float64(foilPacks)/statPacks, 0.225, tolFoilRate)

	if foilPacks > 0 {
		n := float64(foilPacks)
		assertRate(t, "TSP foil common fraction", float64(foilByGroup["common"])/n, 0.500, tolRarity)
		assertRate(t, "TSP foil uncommon fraction", float64(foilByGroup["uncommon"])/n, 0.250, tolRarity)
		assertRate(t, "TSP foil rare fraction", float64(foilByGroup["rare"])/n, 0.125, tolRarity)
		assertRate(t, "TSP foil timeshifted fraction", float64(foilByGroup["timeshifted"])/n, 0.125, tolRarity)
	}
}

// ---------------------------------------------------------------------------
// Planar Chaos — structural tests
// ---------------------------------------------------------------------------

// TestClassicGenerator_planarChaosReturns15Cards verifies that every PLC booster
// contains exactly 15 cards across both booster options.
func TestClassicGenerator_planarChaosReturns15Cards(t *testing.T) {
	t.Parallel()
	plc := domaintest.NewPlanarChaosCatalogue()
	gen := domain.NewClassicGenerator()
	rng := newRNG(23)

	for i := range 100 {
		b, err := gen.Generate(plc, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}
		if len(b.Cards) != 15 {
			t.Errorf("pack %d: len(Cards) = %d, want 15", i, len(b.Cards))
		}
	}
}

// ---------------------------------------------------------------------------
// Planar Chaos — statistical tests (100 000 packs, seed 24)
//
// Tolerance justification (n = 100 000):
//   foil-pack rate p = 0.225, σ ≈ 0.00132 → ±0.5% ≈ 3.8σ
//   CS-slot rare fraction p = 0.25, σ ≈ 0.00137 → ±0.5% ≈ 3.6σ
//
// Foil rarity fractions (n ≈ 22 500 foil packs):
//   p = 0.60 (common),   σ ≈ 0.00326 → ±1.0% ≈ 3.1σ
//   p = 0.25 (uncommon), σ ≈ 0.00289 → ±1.0% ≈ 3.5σ
//   p = 0.15 (rare),     σ ≈ 0.00238 → ±1.0% ≈ 4.2σ
// ---------------------------------------------------------------------------

// TestClassicGenerator_statisticsPlanarChaos runs 100 000 PLC packs and asserts:
//  1. foil-pack rate = 22.5% ± 0.5%
//  2. colorshifted-slot rare fraction = 25% ± 0.5%  (aggregate weight 30:10 → 25%)
//  3. foil rarity distribution = 60/25/15 within ±1%
func TestClassicGenerator_statisticsPlanarChaos(t *testing.T) {
	t.Parallel()
	plc := domaintest.NewPlanarChaosCatalogue()
	gen := domain.NewClassicGenerator()
	rng := newRNG(24)

	var foilPacks int
	var csSlotRares int // packs where the colorshifted-slot drew a rare
	foilByRarity := make(map[domain.Rarity]int)

	for i := range statPacks {
		b, err := gen.Generate(plc, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}

		// Identify the colorshifted-slot card: a non-foil card whose ID starts with
		// "plc-cs-u-" or "plc-cs-r-" (colorshifted uncommon or rare).
		for _, ck := range b.Cards {
			if ck.Finish == domain.FinishFoil {
				continue
			}
			c, ok := plc.Cards[ck.ID]
			if !ok {
				continue
			}
			id := string(ck.ID)
			if strings.HasPrefix(id, "plc-cs-u-") || strings.HasPrefix(id, "plc-cs-r-") {
				if c.Rarity == domain.RarityRare {
					csSlotRares++
				}
				// Only one colorshifted-slot card per pack; break after finding it.
				// (Regular uncommons plc-u-* are not in the colorshifted-slot sheet.)
				break
			}
		}

		for _, ck := range b.Cards {
			if ck.Finish != domain.FinishFoil {
				continue
			}
			foilPacks++
			c, ok := plc.Cards[ck.ID]
			if ok {
				foilByRarity[c.Rarity]++
			}
		}
	}

	const tolFoilRate = 0.005 // ±0.5%
	const tolCSRare = 0.005   // ±0.5%
	const tolRarity = 0.010   // ±1.0%

	assertRate(t, "PLC foil-pack rate", float64(foilPacks)/statPacks, 0.225, tolFoilRate)
	assertRate(t, "PLC colorshifted-slot rare fraction", float64(csSlotRares)/statPacks, 0.25, tolCSRare)

	if foilPacks > 0 {
		n := float64(foilPacks)
		assertRate(t, "PLC foil common fraction", float64(foilByRarity[domain.RarityCommon])/n, 0.60, tolRarity)
		assertRate(t, "PLC foil uncommon fraction", float64(foilByRarity[domain.RarityUncommon])/n, 0.25, tolRarity)
		assertRate(t, "PLC foil rare fraction", float64(foilByRarity[domain.RarityRare])/n, 0.15, tolRarity)
	}
}
