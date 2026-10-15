package config

import (
	"fmt"
	"maps"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// validateArchetypeTraits checks the traits and trait parameters of the archetype at key, such as
// "npcs.archetypes.stingy". It returns an error naming the offending key.
func validateArchetypeTraits(key string, a ArchetypeConfig) error {
	return checkTraits(key, a, domain.TraitNames())
}

// checkTraits validates the trait list and trait parameters of an archetype. knownNames is the
// sorted list of names the registry accepts; tests pass a custom list to avoid importing the
// real registry.
//
// Checks in order:
//  1. Duplicate name in traits.
//  2. Any trait_params entry whose name is not in traits.
//  3. Any param value < 0.
//  4. Each trait name must be known.
func checkTraits(key string, a ArchetypeConfig, knownNames []string) error {
	// 1. Duplicate names.
	seen := make(map[string]bool, len(a.Traits))
	for _, name := range a.Traits {
		if seen[name] {
			return fmt.Errorf("config: %s.traits has duplicate trait name %q", key, name)
		}
		seen[name] = true
	}

	// 2. trait_params entries whose name is not in traits.
	traitSet := seen // reuse: seen is already the set of trait names
	for _, pname := range slices.Sorted(maps.Keys(a.TraitParams)) {
		if !traitSet[pname] {
			return fmt.Errorf("config: %s.trait_params has %q which is not in traits", key, pname)
		}
	}

	// 3. Negative param values (iterate in stable order).
	for _, tname := range slices.Sorted(maps.Keys(a.TraitParams)) {
		params := a.TraitParams[tname]
		for _, pname := range slices.Sorted(maps.Keys(params)) {
			if params[pname] < 0 {
				return fmt.Errorf("config: %s.trait_params.%s.%s must be >= 0, got %d", key, tname, pname, params[pname])
			}
		}
	}

	// 4. Unknown trait names (check each with its index for the error message).
	known := make(map[string]bool, len(knownNames))
	for _, n := range knownNames {
		known[n] = true
	}
	for i, name := range a.Traits {
		if !known[name] {
			return fmt.Errorf("config: %s.traits[%d] %q is not a registered trait, known traits: %v", key, i, name, knownNames)
		}
	}
	return nil
}

// validateMarginCap checks that NPCConfig.MaxMarginPct is not negative.
func validateMarginCap(n NPCConfig) error {
	if n.MaxMarginPct < 0 {
		return fmt.Errorf("config: npcs.max_margin_pct must be >= 0, got %d", n.MaxMarginPct)
	}
	return nil
}
