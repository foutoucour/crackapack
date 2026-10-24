package web

import (
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

type homeView struct {
	// Staged is true in a game whose sets open up in stages: each stage has a heading of its own.
	Staged bool
	Stages []stageView
}

// stageView is one stage of the home page: the sets the player can open, as tiles, and the locked
// ones as a short list, so that a hundred sets stay a page one can scan.
type stageView struct {
	Number int
	Open   []tileView
	Locked []play.Pack
}

// tileView is a set the player can open, with its icon in the colour of the rarest card held.
type tileView struct {
	play.Pack
	Icon iconView
}

// Size is the number of sets in the stage.
func (s stageView) Size() int { return len(s.Open) + len(s.Locked) }

// newHomeView groups packs by stage, keeping their order. A game without stages has one group.
// binders tell the rarest card held of each set, which colours its icon.
func newHomeView(packs []play.Pack, binders []play.BinderSummary) homeView {
	rarest := make(map[string]string, len(binders))
	for _, b := range binders {
		rarest[b.SetCode] = rarestHeld(b.Progress)
	}
	var view homeView
	for _, p := range packs {
		if n := len(view.Stages); n == 0 || view.Stages[n-1].Number != p.Stage {
			view.Stages = append(view.Stages, stageView{Number: p.Stage})
		}
		stage := &view.Stages[len(view.Stages)-1]
		if p.Locked {
			stage.Locked = append(stage.Locked, p)
		} else {
			stage.Open = append(stage.Open, tileView{Pack: p, Icon: newIconView(p.IconCode, rarest[p.SetCode])})
		}
		view.Staged = view.Staged || p.Stage > 0
	}
	return view
}

type errorView struct {
	Message string
	// BackHref and BackLabel are the way out of the page, the home page when they are empty.
	BackHref  string
	BackLabel string
	// AltHref and AltLabel are a second way out, such as the market page, empty when there is none.
	AltHref  string
	AltLabel string
}

type packView struct {
	SetName string
	SetCode string
	Cards   []cardView
	// Left is the welcome boosters of this set the player still holds.
	Left int
	// Cost is the energy another booster of the set costs once the welcome ones are gone, and Wait
	// the time until the player can open one, 0 when they can now.
	Cost int
	Wait time.Duration
	// Unlocked are the sets the booster unlocked.
	Unlocked []play.UnlockedSet
}

type cardView struct {
	Name   string
	Rarity string
	Colour string
	// ImageID is the card's Scryfall ID, empty when it has none and the card shows as text.
	ImageID string
	Foil    bool
	New     bool
	// Icon is the icon of the card's set in the colour of its rarity.
	Icon iconView
}

func newPackView(opening play.Opening, packs []play.Pack) packView {
	view := packView{SetName: opening.SetName, SetCode: opening.SetCode, Cards: make([]cardView, len(opening.Cards)), Unlocked: opening.Unlocked}
	for _, p := range packs {
		if p.SetCode == opening.SetCode {
			view.Left, view.Cost, view.Wait = p.Count, p.Cost, p.Wait
		}
	}
	for i, c := range opening.Cards {
		view.Cards[i] = cardView{
			Name:    c.Card.Name,
			Rarity:  string(c.Card.Rarity),
			Colour:  colourClass(c.Card.Colors),
			ImageID: c.Card.ScryfallID,
			Foil:    c.Finish == domain.FinishFoil,
			New:     c.New,
			Icon:    newIconView(opening.IconCode, string(c.Card.Rarity)),
		}
	}
	return view
}

// colourClass names the CSS frame of a card: "colourless" for no colour, the colour's name for one,
// and "gold" for several.
func colourClass(colours []string) string {
	if len(colours) > 1 {
		return "gold"
	}
	if len(colours) == 0 {
		return "colourless"
	}
	switch colours[0] {
	case "W":
		return "white"
	case "U":
		return "blue"
	case "B":
		return "black"
	case "R":
		return "red"
	case "G":
		return "green"
	default:
		return "colourless"
	}
}
