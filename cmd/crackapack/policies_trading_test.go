package main

import "testing"

func TestBuildPolicies_includesTheTrader(t *testing.T) {
	t.Parallel()
	build, ok := buildPolicies()["trader"]
	if !ok {
		t.Fatal("policy \"trader\" is not registered")
	}
	if build() == nil {
		t.Error("policy \"trader\" builds nil")
	}
}
