package main

// conversion_sim_test.go runs the three coin top-up variants on the synthetic
// demo dataset and prints a comparison table. The numbers feed docs/simulation.md.
//
// Run with:
//
//	go test -v -run TestCoinTopupVariants_SimReport ./cmd/crackapack

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// TestCoinTopupVariants_SimReport runs the three coin top-up variants plus a baseline on the
// demo dataset with simplified settings (no NPCs, no trades, no counter budget) to isolate the
// coin top-up effect. Use -v to see the table.
//
// Two scenarios:
//
//  1. High-fee (p=300, 60 days): coins_per_energy matches the demo config; booster fees are
//     enormous relative to daily income, so no boosters open in this window. The comparison
//     shows how each variant affects pace and idle coin pile.
//
//  2. Low-fee stall (p=10, 30 days): the plain collector stalls; the converting collector
//     should accumulate enough for the RAV fee (≈8280 coins at p=10) and open at least one
//     booster before the window closes.
func TestCoinTopupVariants_SimReport(t *testing.T) {
	t.Parallel()

	ds := demo.Build()

	type variantCase struct {
		name   string
		topup  config.EnergyCoinTopupConfig
		player string
	}

	type row struct {
		name       string
		boosters   float64
		topupCoins float64
		idleCoins  float64
	}

	runScenario := func(t *testing.T, label string, coinsPerEnergy int, runDays int, variants []variantCase) {
		t.Helper()
		var rows []row

		for _, tc := range variants {
			in, err := loadInputs(simulateArgs{
				configReader: strings.NewReader(ds.ConfigYAML),
				catFS:        ds.Files,
				setCodes:     ds.SetCodes,
				suppCodes:    ds.SupplementaryCodes,
				pricesFS:     ds.Files,
				pricesName:   "prices.json",
				player:       tc.player,
				policies:     buildPolicies(),
				newOutput:    discardOutput,
				stdout:       nil,
			})
			if err != nil {
				t.Fatalf("loadInputs for %q: %v", tc.name, err)
			}

			cfg := in.cfg
			cfg.RunDays = runDays
			cfg.CoinRate = 200
			cfg.Seed = 42
			cfg.WelcomeBonus = 200
			cfg.DailyReward = 100
			cfg.Energy.CoinsPerEnergy = domain.Coins(coinsPerEnergy)
			cfg.Energy.CoinTopup = tc.topup
			// Eliminate NPCs, trades, and counter budget so only energy income and
			// daily reward affect pace and idle coins.
			cfg.NPCs.Population = 0
			cfg.NPCs.Archetypes = nil
			cfg.Trades.Enabled = false
			cfg.CounterBudget.Enabled = false
			cfg.CounterBudget.Initial = 0
			cfg.CounterBudget.DailyFloat = 0
			cfg.CounterSupply.Mode = "none"
			// A small initial stock lets the collector buy cards but keeps it simple.
			cfg.InitialStock = map[domain.Rarity]int{
				domain.RarityCommon:   5,
				domain.RarityUncommon: 2,
				domain.RarityRare:     1,
			}

			rng := rand.New(rand.NewPCG(uint64(cfg.Seed), uint64(cfg.Seed)))
			e, err := sim.NewEngine(cfg, in.catalogues, in.supplementary, in.prices, in.policyFn(), rng,
				sim.WithReferenceCatalogues(in.reference))
			if err != nil {
				t.Fatalf("NewEngine for %q: %v", tc.name, err)
			}
			result, err := e.Run()
			if err != nil {
				t.Fatalf("Run for %q: %v", tc.name, err)
			}

			s := result.Summary
			// boosters_opened is per-day in DayMetrics, not in the Summary map.
			var boosters float64
			for _, d := range result.Days {
				boosters += float64(d.BoostersOpened)
			}
			rows = append(rows, row{
				name:       tc.name,
				boosters:   boosters,
				topupCoins: s["coin_topup_coins"],
				idleCoins:  s["final_balance"],
			})
		}

		t.Log("")
		t.Logf("%s (demo dataset, %d days, coin_rate=200, p=%d, no NPCs/trades):", label, runDays, coinsPerEnergy)
		t.Log("──────────────────────────────────────────────────────────────────────────────────")
		t.Logf("%-42s  %10s  %16s  %12s", "variant", "boosters", "coin_topup_coins", "idle_coins")
		t.Log("──────────────────────────────────────────────────────────────────────────────────")
		for _, r := range rows {
			t.Logf("%-42s  %10.0f  %16.0f  %12.0f", r.name, r.boosters, r.topupCoins, r.idleCoins)
		}
		t.Log("──────────────────────────────────────────────────────────────────────────────────")
	}

	// Table 1: high-fee scenario matching the demo's coins_per_energy=300.
	runScenario(t, "High-fee scenario", 300, 60, []variantCase{
		{
			name:   "baseline (no top-up, collector)",
			topup:  config.EnergyCoinTopupConfig{Enabled: false},
			player: "collector",
		},
		{
			name: "min_energy (min_energy_units=1)",
			topup: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantMinEnergy, MinEnergyUnits: 1, EscalationStep: 1,
			},
			player: "converting-collector",
		},
		{
			name: "escalating (step=30)",
			topup: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantEscalating, MinEnergyUnits: 1, EscalationStep: 30,
			},
			player: "converting-collector",
		},
		{
			name: "both (min_energy_units=1, step=30)",
			topup: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantBoth, MinEnergyUnits: 1, EscalationStep: 30,
			},
			player: "converting-collector",
		},
		{
			name: "energy-first (min_energy_units=1)",
			topup: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantMinEnergy, MinEnergyUnits: 1, EscalationStep: 1,
			},
			player: "energy-first",
		},
	})

	// Table 2: low-fee stall scenario at p=10.
	runScenario(t, "Low-fee stall scenario", 10, 30, []variantCase{
		{
			name:   "baseline (no top-up, collector)",
			topup:  config.EnergyCoinTopupConfig{Enabled: false},
			player: "collector",
		},
		{
			name: "min_energy (min_energy_units=1)",
			topup: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantMinEnergy, MinEnergyUnits: 1, EscalationStep: 1,
			},
			player: "converting-collector",
		},
		{
			name: "energy-first (min_energy_units=1)",
			topup: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantMinEnergy, MinEnergyUnits: 1, EscalationStep: 1,
			},
			player: "energy-first",
		},
	})
}
