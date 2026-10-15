package config

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// validateArchetypeColors checks the colours of the archetype at key, such as
// "npcs.archetypes.stingy". It rejects a colour that is not one of W, U, B, R, G and a colour
// listed more than once.
func validateArchetypeColors(key string, a ArchetypeConfig) error {
	valid := make(map[string]bool, len(domain.ValidColors()))
	for _, c := range domain.ValidColors() {
		valid[c] = true
	}
	seen := make(map[string]bool, len(a.Colors))
	for _, c := range a.Colors {
		if !valid[c] {
			return fmt.Errorf("config: %s.colors: unknown colour %q, want one of W, U, B, R, G", key, c)
		}
		if seen[c] {
			return fmt.Errorf("config: %s.colors: colour %q is listed twice", key, c)
		}
		seen[c] = true
	}
	return nil
}
