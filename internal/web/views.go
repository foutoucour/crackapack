package web

import (
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

type homeView struct {
	Packs []play.Pack
}

type errorView struct {
	Message string
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
}

type cardView struct {
	Name   string
	Rarity string
	Colour string
	// ImageID is the card's Scryfall ID, empty when it has none and the card shows as text.
	ImageID string
	Foil    bool
	New     bool
}

func newPackView(opening play.Opening, packs []play.Pack) packView {
	view := packView{SetName: opening.SetName, SetCode: opening.SetCode, Cards: make([]cardView, len(opening.Cards))}
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
