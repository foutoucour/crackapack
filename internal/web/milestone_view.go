package web

import (
	"fmt"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// colourName is how the page names a colour group of a binder: the group's own name, but for the
// five colours, which are written as letters.
func colourName(group domain.ColourGroup) string {
	switch group {
	case domain.ColourWhite:
		return "white"
	case domain.ColourBlue:
		return "blue"
	case domain.ColourBlack:
		return "black"
	case domain.ColourRed:
		return "red"
	case domain.ColourGreen:
		return "green"
	case domain.ColourGold, domain.ColourColourless, domain.ColourLand:
		return string(group)
	}
	return string(group)
}

// goalName is what a milestone asks for, as a phrase that follows "every" or opens a sentence.
func goalName(m domain.Milestone) string {
	switch m.Kind {
	case domain.MilestoneRarity:
		return m.Group + " card"
	case domain.MilestoneColour:
		return colourName(domain.ColourGroup(m.Group)) + " card"
	case domain.MilestoneBinder:
		return "card of the set"
	}
	return "card"
}

// earnedGoal says what a milestone that the player has earned is and what it gave, as the words of
// the check mark that stands for it in the filter row of its cards.
func earnedGoal(s play.MilestoneStatus) string {
	var text strings.Builder
	switch {
	case s.Milestone.Kind == domain.MilestoneBinder:
		text.WriteString("Milestone reached: the whole set, which unlocks sets.")
	case s.Reward > 0:
		fmt.Fprintf(&text, "Milestone reached: every %s, which pays %d energy.", goalName(s.Milestone), s.Reward)
	default:
		fmt.Fprintf(&text, "Milestone reached: every %s, which pays no energy.", goalName(s.Milestone))
	}
	text.WriteString(" One copy of each of these cards is locked: you keep it.")
	return text.String()
}

// markMilestones puts a check mark on the filter row of each goal of the binder that the player has
// earned. A goal that is not earned yet leaves its row bare.
func (v *collectionView) markMilestones(statuses []play.MilestoneStatus) {
	for _, s := range statuses {
		if !s.Done {
			continue
		}
		for _, panel := range []*filterPanel{&v.Rarities, &v.Colours} {
			for i := range panel.Rows {
				if row := &panel.Rows[i]; row.Goal.Kind == s.Milestone.Kind && row.Goal.Group == s.Milestone.Group {
					row.Earned = earnedGoal(s)
				}
			}
		}
	}
}

// milestoneNotice tells the player of a milestone just earned.
type milestoneNotice struct {
	Text     string
	Unlocked []play.UnlockedSet
}

// newMilestoneNotice words what a milestone gave. The copy it locks is said every time, since it is
// what the player cannot take back.
func newMilestoneNotice(e play.EarnedMilestone) milestoneNotice {
	var text strings.Builder
	what := "every " + goalName(e.Milestone)
	if e.Milestone.Kind == domain.MilestoneBinder {
		what = "the whole set"
	}
	fmt.Fprintf(&text, "Milestone reached in %s: %s.", e.SetName, what)
	if e.Energy > 0 {
		fmt.Fprintf(&text, " +%d energy.", e.Energy)
	}
	text.WriteString(" One copy of each of these cards is now locked: you keep it.")
	return milestoneNotice{Text: text.String(), Unlocked: e.Unlocked}
}

// takeMilestones is the notices of the milestones earned since the last page, each told once.
func (h *handler) takeMilestones() []milestoneNotice {
	var notices []milestoneNotice
	for _, e := range h.game.TakeMilestones() {
		notices = append(notices, newMilestoneNotice(e))
	}
	return notices
}
