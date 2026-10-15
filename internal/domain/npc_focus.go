package domain

import (
	"fmt"
	"strings"
)

// FocusSetMode controls which set an NPC of an archetype opens.
// The zero value FocusSetNone means no restriction: the NPC picks a random set each booster.
// See ADR-0026.
type FocusSetMode int

const (
	// FocusSetNone is the zero value: the NPC picks a random set each time it opens a booster.
	// This is the behaviour that corresponds to the old bool false in config.
	FocusSetNone FocusSetMode = iota
	// FocusSetRandom draws one set at population generation; the NPC opens only that set for
	// all subsequent boosters. This corresponds to the old bool true in config.
	FocusSetRandom
	// FocusSetOldest resolves to the openable set with the earliest release date among the sets
	// the population is generated from. See ADR-0026.
	FocusSetOldest
)

// oldestSet returns the code of the set with the earliest ReleaseDate among sets.
// All sets must carry a non-empty date: any set with an empty ReleaseDate causes an error
// that lists all undated codes in the order they appear in sets. Ties go to the first set
// in the given order (strict Before comparison).
func oldestSet(sets []SetCatalogue) (string, error) {
	// Collect all undated codes first so we can name them all in one error.
	var undated []string
	for _, cat := range sets {
		if cat.ReleaseDate == "" {
			undated = append(undated, cat.Code)
		}
	}
	if len(undated) > 0 {
		return "", fmt.Errorf("focus_set oldest needs a release date for every set, missing: %s",
			strings.Join(undated, ", "))
	}

	oldest := sets[0]
	oldestTime, err := parseReleaseDate(oldest.ReleaseDate)
	if err != nil {
		return "", fmt.Errorf("set %q: %w", oldest.Code, err)
	}
	for _, cat := range sets[1:] {
		t, err := parseReleaseDate(cat.ReleaseDate)
		if err != nil {
			return "", fmt.Errorf("set %q: %w", cat.Code, err)
		}
		if t.Before(oldestTime) {
			oldest = cat
			oldestTime = t
		}
	}
	return oldest.Code, nil
}
