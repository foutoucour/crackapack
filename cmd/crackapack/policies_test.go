package main

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

// TestBuildPolicies_registeredPolicies verifies that "collector", "energy-first", "cash-out" and
// "idle" are registered and each constructs a non-nil PlayerPolicy.
func TestBuildPolicies_registeredPolicies(t *testing.T) {
	t.Parallel()

	policies := buildPolicies()

	for _, name := range []string{"collector", "energy-first", "cash-out", "idle", "master-collector"} {
		fn, ok := policies[name]
		if !ok {
			t.Errorf("policy %q not registered", name)
			continue
		}
		p := fn()
		if p == nil {
			t.Errorf("policy %q constructor returned nil", name)
		}
	}
}

// Compile-time check: every policy implements sim.PlayerPolicy.
var _ sim.PlayerPolicy = sim.NewCollectorPolicy()
var _ sim.PlayerPolicy = sim.NewCashOutPolicy()
var _ sim.PlayerPolicy = sim.NewIdlePolicy()
