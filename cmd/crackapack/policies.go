package main

import "github.com/foutoucour/crackapack/internal/sim"

// buildPolicies returns the map of registered player policies. T-011 adds its
// cash-out policy by inserting an entry here.
func buildPolicies() map[string]func() sim.PlayerPolicy {
	return map[string]func() sim.PlayerPolicy{
		"collector": func() sim.PlayerPolicy { return sim.NewCollectorPolicy() },
	}
}
