package main

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

// TestBuildPolicies_registeredPolicies verifies that both "collector" and
// "cash-out" are registered and each constructs a non-nil PlayerPolicy.
func TestBuildPolicies_registeredPolicies(t *testing.T) {
	t.Parallel()

	policies := buildPolicies()

	for _, name := range []string{"collector", "cash-out"} {
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

// Compile-time check: both policies implement sim.PlayerPolicy.
var _ sim.PlayerPolicy = sim.NewCollectorPolicy()
var _ sim.PlayerPolicy = sim.NewCashOutPolicy()
