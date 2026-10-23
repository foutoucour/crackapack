package domain

import (
	"fmt"
	"slices"
)

// ValidColors returns the Magic colours an archetype may list, as MTGJSON writes them.
func ValidColors() []string { return []string{"W", "U", "B", "R", "G"} }

// ColourGroup is the group a card falls in when a binder is counted by colour: one of the five
// colours, gold for a card of several, colourless, or land. Every card is in exactly one group.
type ColourGroup string

// The colour groups, see [ColourGroups].
const (
	ColourWhite      ColourGroup = "W"
	ColourBlue       ColourGroup = "U"
	ColourBlack      ColourGroup = "B"
	ColourRed        ColourGroup = "R"
	ColourGreen      ColourGroup = "G"
	ColourGold       ColourGroup = "gold"
	ColourColourless ColourGroup = "colourless"
	ColourLand       ColourGroup = "land"
)

// ColourGroups lists the colour groups in the order a binder shows them.
func ColourGroups() []ColourGroup {
	return []ColourGroup{ColourWhite, ColourBlue, ColourBlack, ColourRed, ColourGreen, ColourGold, ColourColourless, ColourLand}
}

// ColourGroupOf returns the group of c, from the MTGJSON "colors" of the one face of the card that a
// binder holds. The loader keeps the faces a booster sheet draws, which is the a side of a split or
// double-faced card, so the b side is not in a binder and a split card is classified by its a half
// only. A non-basic land is a land whatever its colours, so that Dryad Arbor counts with the lands.
// Otherwise a card with two colours or more is gold, and one with none is colourless, which includes
// devoid cards and colour indicators that MTGJSON leaves out of "colors". A card of one colour is in
// that colour's group. Data the loader would have refused is colourless.
func ColourGroupOf(c Card) ColourGroup {
	switch {
	case c.IsLand:
		return ColourLand
	case len(c.Colors) > 1:
		return ColourGold
	case len(c.Colors) == 0:
		return ColourColourless
	}
	if slices.Contains(ValidColors(), c.Colors[0]) {
		return ColourGroup(c.Colors[0])
	}
	return ColourColourless
}

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
