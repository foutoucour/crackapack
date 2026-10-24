package domain

import (
	"cmp"
	"slices"
)

// MilestoneKind is what a milestone asks the player to complete in a binder.
type MilestoneKind string

// The kinds of milestone, see [Milestone].
const (
	MilestoneRarity MilestoneKind = "rarity"
	MilestoneColour MilestoneKind = "colour"
	MilestoneBinder MilestoneKind = "binder"
)

// MilestoneRarities lists the rarities that have a milestone, from the easiest to the hardest. The
// special rarity of Time Spiral's timeshifted cards has none.
func MilestoneRarities() []Rarity {
	return []Rarity{RarityCommon, RarityUncommon, RarityRare, RarityMythic}
}

// Milestone is a goal of one set's binder: every card of a rarity, every card of a colour group, or
// every card. Basic lands are in no binder count, so they are in no milestone. A milestone is
// comparable, so it is a map key.
type Milestone struct {
	Set  string
	Kind MilestoneKind
	// Group is the rarity or the colour group a milestone of that kind names, and empty for the
	// whole binder.
	Group string
}

// Includes reports whether card is one of the cards the milestone asks for.
func (m Milestone) Includes(card Card) bool {
	if card.IsBasicLand {
		return false
	}
	switch m.Kind {
	case MilestoneRarity:
		return string(card.Rarity) == m.Group
	case MilestoneColour:
		return string(ColourGroupOf(card)) == m.Group
	case MilestoneBinder:
		return true
	}
	return false
}

// Cards returns the cards of binder that the milestone asks for, ordered by ID.
func (m Milestone) Cards(binder SetCatalogue) []Card {
	var cards []Card
	for _, card := range binder.Cards {
		if m.Includes(card) {
			cards = append(cards, card)
		}
	}
	slices.SortFunc(cards, func(a, b Card) int { return cmp.Compare(a.ID, b.ID) })
	return cards
}

// MilestonesOf lists the milestones of binder: a rarity or a colour group that the binder has no
// card in has none, since it could never be completed. They come in the order a binder shows them,
// the rarities from common up, the colour groups, and the whole binder last.
func MilestonesOf(binder SetCatalogue) []Milestone {
	progress := NewCollection().BinderProgress(binder)
	var out []Milestone
	for _, rarity := range MilestoneRarities() {
		if progress.ByRarity[rarity].Total > 0 {
			out = append(out, Milestone{Set: binder.Code, Kind: MilestoneRarity, Group: string(rarity)})
		}
	}
	for _, group := range ColourGroups() {
		if progress.ByColour[group].Total > 0 {
			out = append(out, Milestone{Set: binder.Code, Kind: MilestoneColour, Group: string(group)})
		}
	}
	if progress.Total > 0 {
		out = append(out, Milestone{Set: binder.Code, Kind: MilestoneBinder})
	}
	return out
}
