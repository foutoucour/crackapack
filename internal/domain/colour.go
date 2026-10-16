package domain

import (
	"fmt"
	"slices"
)

// ValidColors returns the Magic colours an archetype may list, as MTGJSON writes them.
func ValidColors() []string { return []string{"W", "U", "B", "R", "G"} }

// colourSet is a bitmask over ValidColors: bit i is the colour at index i.
// It is order-free and duplicate-free: OR-ing the same bit twice is idempotent.
type colourSet uint8

// allColours is the bitmask for all five Magic colours.
const allColours colourSet = 0b11111

// colourSetOf converts a list of colour letters to a bitmask, ignoring unknown letters.
// The result is the same regardless of order or duplicates.
func colourSetOf(colors []string) colourSet {
	known := ValidColors()
	var s colourSet
	for _, c := range colors {
		if i := slices.Index(known, c); i >= 0 {
			s |= 1 << i
		}
	}
	return s
}

// admits reports whether card c is within the NPC's colour set s.
// A card is in scope when all of its colours are in s. Colourless cards (empty Colors) are in
// scope for everyone, as are cards with no colour data, because an empty card set is a subset
// of every set.
func (s colourSet) admits(c Card) bool {
	return colourSetOf(c.Colors)&^s == 0
}

// filterByColours returns the cards in base whose colours are all within colours, in the same
// order. It allocates a new slice; the caller may cache it.
func filterByColours(base []Card, colours colourSet) []Card {
	out := make([]Card, 0, len(base))
	for _, c := range base {
		if colours.admits(c) {
			out = append(out, c)
		}
	}
	return out
}

// validateColors rejects a colour outside ValidColors and a colour listed twice.
// An empty list is valid. The function is unexported because config uses its own error format.
func validateColors(colors []string) error {
	known := ValidColors()
	seen := make(map[string]bool, len(colors))
	for _, c := range colors {
		if !slices.Contains(known, c) {
			return fmt.Errorf("colour %q is not one of W, U, B, R, G", c)
		}
		if seen[c] {
			return fmt.Errorf("colour %q is listed twice", c)
		}
		seen[c] = true
	}
	return nil
}
