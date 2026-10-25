package web

import (
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// cardThumb is a card on a page of the market or of the hunt: its picture, grey while the player
// does not hold it, linked to its slot in the binder of its set, with the name and the icon of the set.
type cardThumb struct {
	Name    string
	ImageID string
	Href    string
	Held    bool
	SetName string
	Icon    iconView
}

// setFinder says which set's binder shows a card, as play.Game.SetOf does.
type setFinder func(domain.CardID) (play.CardSet, bool)

func newCardThumb(card domain.Card, held bool, setOf setFinder) cardThumb {
	set, _ := setOf(card.ID)
	return cardThumb{
		Name:    card.Name,
		ImageID: card.ScryfallID,
		Href:    binderHref(set.Code, binderFilter{}, card.ID),
		Held:    held,
		SetName: set.Name,
		Icon:    newIconView(set.IconCode, string(card.Rarity)),
	}
}
