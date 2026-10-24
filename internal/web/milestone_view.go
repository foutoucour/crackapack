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

// milestoneView is one goal of the binder on the collection page.
type milestoneView struct {
	Name   string
	Done   bool
	Reward int
	// Unlocks is true for the whole set, which opens sets instead of paying energy.
	Unlocks bool
}

func newMilestoneViews(statuses []play.MilestoneStatus) []milestoneView {
	views := make([]milestoneView, len(statuses))
	for i, s := range statuses {
		name := "Every " + goalName(s.Milestone)
		if s.Milestone.Kind == domain.MilestoneBinder {
			name = "The whole set"
		}
		views[i] = milestoneView{Name: name, Done: s.Done, Reward: s.Reward, Unlocks: s.Milestone.Kind == domain.MilestoneBinder}
	}
	return views
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
