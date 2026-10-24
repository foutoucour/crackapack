package config

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// MilestonesConfig is what a milestone of a binder pays, see docs/core-features.md#milestones. Each
// reward is energy, in percent of energy.reserve_cap, so that a reward follows the bar when the bar
// changes: with a reserve of 10, 50 pays 5. A reward of 0 turns that milestone's energy off, and
// the milestone still locks its cards.
type MilestonesConfig struct {
	// CommonPct, UncommonPct, RarePct and MythicPct are what owning every card of that rarity of a
	// set pays (default 50, 100, 200 and 400).
	CommonPct   domain.Percent `yaml:"common_pct"`
	UncommonPct domain.Percent `yaml:"uncommon_pct"`
	RarePct     domain.Percent `yaml:"rare_pct"`
	MythicPct   domain.Percent `yaml:"mythic_pct"`
	// ColourPct is what owning every card of a colour group of a set pays (default 50).
	ColourPct domain.Percent `yaml:"colour_pct"`
}

// rewards names each reward by its key, in the order a configuration lists them.
func (m MilestonesConfig) rewards() []struct {
	key     string
	percent domain.Percent
} {
	return []struct {
		key     string
		percent domain.Percent
	}{
		{"milestones.common_pct", m.CommonPct},
		{"milestones.uncommon_pct", m.UncommonPct},
		{"milestones.rare_pct", m.RarePct},
		{"milestones.mythic_pct", m.MythicPct},
		{"milestones.colour_pct", m.ColourPct},
	}
}

// RarityPercents is the reward of each rarity that has a milestone, by rarity.
func (m MilestonesConfig) RarityPercents() map[domain.Rarity]domain.Percent {
	return map[domain.Rarity]domain.Percent{
		domain.RarityCommon: m.CommonPct, domain.RarityUncommon: m.UncommonPct,
		domain.RarityRare: m.RarePct, domain.RarityMythic: m.MythicPct,
	}
}

// validateMilestones checks the lower bounds of the milestones block; validateBounds checks the
// upper ones.
func validateMilestones(m MilestonesConfig) error {
	for _, r := range m.rewards() {
		if r.percent < 0 {
			return fmt.Errorf("config: %s must be >= 0, got %d", r.key, r.percent)
		}
	}
	return nil
}
