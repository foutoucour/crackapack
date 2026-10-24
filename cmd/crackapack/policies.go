package main

import "github.com/foutoucour/crackapack/internal/sim"

// buildPolicies returns the map of registered player policies.
func buildPolicies() map[string]func() sim.PlayerPolicy {
	return map[string]func() sim.PlayerPolicy{
		"boosters-only":        func() sim.PlayerPolicy { return sim.NewBoostersOnlyPolicy() },
		"collector":            func() sim.PlayerPolicy { return sim.NewCollectorPolicy() },
		"converting-collector": func() sim.PlayerPolicy { return sim.NewConvertingCollectorPolicy() },
		"master-collector":     func() sim.PlayerPolicy { return sim.NewMasterCollectorPolicy() },
		"energy-first":         func() sim.PlayerPolicy { return sim.NewEnergyFirstPolicy() },
		"cash-out":             func() sim.PlayerPolicy { return sim.NewCashOutPolicy() },
		"idle":                 func() sim.PlayerPolicy { return sim.NewIdlePolicy() },
		"trader":               func() sim.PlayerPolicy { return sim.NewTraderPolicy() },
		"market-trader":        func() sim.PlayerPolicy { return sim.NewMarketTraderPolicy() },
		"exploiter":            func() sim.PlayerPolicy { return sim.NewExploiterPolicy() },
	}
}
