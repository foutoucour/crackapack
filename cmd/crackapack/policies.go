package main

import "github.com/foutoucour/crackapack/internal/sim"

// buildPolicies returns the map of registered player policies.
func buildPolicies() map[string]func() sim.PlayerPolicy {
	return map[string]func() sim.PlayerPolicy{
		"collector": func() sim.PlayerPolicy { return sim.NewCollectorPolicy() },
		"cash-out":  func() sim.PlayerPolicy { return sim.NewCashOutPolicy() },
		"idle":      func() sim.PlayerPolicy { return sim.NewIdlePolicy() },
		"trader":    func() sim.PlayerPolicy { return sim.NewTraderPolicy() },
	}
}
