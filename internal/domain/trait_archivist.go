package domain

import (
	"fmt"
	"time"
)

// archivistTrait scores wish candidates by how old their set is. Older sets score higher,
// so the NPC's wish list starts with cards from the earliest sets loaded.
//
// The score is the whole days from the card's set release date to the horizon (2100-01-01 UTC).
// A card from 1993 scores ~38000; one from 2023 scores ~28000. The gap spans thousands of
// points per decade, so in a mixed archetype this score dominates any small-score trait.
// Pair with care — calibration story planned separately.
type archivistTrait struct {
	env     TraitEnv
	horizon time.Time
}

// newArchivistTrait builds an archivistTrait. It rejects any parameters (the archivist reads
// none) and requires CardSet and SetReleased to be non-nil in env.
func newArchivistTrait(params map[string]int, env TraitEnv) (Trait, error) {
	for k := range params {
		return nil, fmt.Errorf("archivist takes no parameters, got %q", k)
	}
	if env.CardSet == nil || env.SetReleased == nil {
		return nil, fmt.Errorf("archivist needs the card environment")
	}
	return &archivistTrait{
		env:     env,
		horizon: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	}, nil
}

// Name implements Trait.
func (a *archivistTrait) Name() string { return "archivist" }

// WishScore implements WishScorer. It returns the whole days from the card's set release date
// to the horizon (2100-01-01), so older sets score higher. Returns 0 when the card's set is
// unknown, the set has no release date, the date does not parse, or the date is past the horizon.
func (a *archivistTrait) WishScore(card Card) int {
	setCode, ok := a.env.CardSet(card.ID)
	if !ok {
		return 0
	}
	isoDate, ok := a.env.SetReleased(setCode)
	if !ok {
		return 0
	}
	t, err := parseReleaseDate(isoDate)
	if err != nil {
		return 0
	}
	days := int(a.horizon.Sub(t).Hours() / 24)
	return max(0, days)
}
