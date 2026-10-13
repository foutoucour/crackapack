package config

import (
	"fmt"
	"maps"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// validateNPCs checks the NPC block. It returns an error naming the offending key on the first
// failure. Archetypes are checked in name order so the reported key is stable.
func validateNPCs(n NPCConfig) error {
	if n.Population < 0 {
		return fmt.Errorf("config: npcs.population must be >= 0, got %d", n.Population)
	}
	var totalShare int
	for _, name := range slices.Sorted(maps.Keys(n.Archetypes)) {
		a := n.Archetypes[name]
		key := "npcs.archetypes." + name
		switch {
		case a.Share < 0:
			return fmt.Errorf("config: %s.share must be >= 0, got %d", key, a.Share)
		case a.MinBoostersPerDay < 0:
			return fmt.Errorf("config: %s.min_boosters_per_day must be >= 0, got %d", key, a.MinBoostersPerDay)
		case a.MaxBoostersPerDay < a.MinBoostersPerDay:
			return fmt.Errorf("config: %s.max_boosters_per_day (%d) must be >= min_boosters_per_day (%d)", key, a.MaxBoostersPerDay, a.MinBoostersPerDay)
		case a.KeepCopies < 1 || a.KeepCopies > domain.PlaysetSize:
			return fmt.Errorf("config: %s.keep_copies must be in [1, %d], got %d", key, domain.PlaysetSize, a.KeepCopies)
		}
		totalShare += a.Share
	}
	if n.Population > 0 && totalShare == 0 {
		return fmt.Errorf("config: npcs.archetypes needs at least one archetype with a share above zero to build %d NPCs", n.Population)
	}
	return nil
}
