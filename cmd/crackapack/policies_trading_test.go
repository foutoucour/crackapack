package main

import "testing"

func TestBuildPolicies_includesTheTradingPolicies(t *testing.T) {
	t.Parallel()
	policies := buildPolicies()

	for _, name := range []string{"trader", "exploiter"} {
		build, ok := policies[name]
		if !ok {
			t.Errorf("policy %q is not registered", name)
			continue
		}
		if build() == nil {
			t.Errorf("policy %q builds nil", name)
		}
	}
}
