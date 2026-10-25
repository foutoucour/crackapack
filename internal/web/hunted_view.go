package web

import (
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// huntedPath is the page of the cards the player hunts, and the value of the form field "from"
// that sends a form back to it.
const (
	huntedPath = "/hunted"
	fromHunted = "hunted"
)

// huntedMark is the hunt of an empty slot: the button that starts it, or the one that stops it.
// It is not the market's huntView, which is asking around for the card once.
type huntedMark struct {
	cardRef
	On bool
}

// huntedNavView is the link to the hunted cards in the header, with the cards that came to the
// Counter since the player last looked.
type huntedNavView struct {
	New int
}

// huntedPageView is the page of the hunted cards.
type huntedPageView struct {
	Cards  []huntedCardView
	Notice noticeView
}

// huntedCardView is one hunted card and where its hunt stands. The Counter's stock is never told
// but by this: that a copy came, not how many.
type huntedCardView struct {
	cardRef
	Name      string
	Foil      bool
	AtCounter bool
	Price     domain.Coins
	New       bool
	Offers    int
	// Href is the card's slot in its binder, where it can be bought, and empty when no unlocked
	// binder holds it.
	Href  string
	Thumb cardThumb
}

func newHuntedPageView(hunted []play.Hunted, setOf setFinder) huntedPageView {
	view := huntedPageView{Cards: make([]huntedCardView, len(hunted))}
	for i, h := range hunted {
		view.Cards[i] = huntedCardView{
			cardRef: cardRef{Set: h.SetCode, ID: h.Card.ID, Finish: string(h.Finish)},
			Name:    h.Card.Name, Foil: h.Finish == domain.FinishFoil,
			AtCounter: h.AtCounter, Price: h.BuyPrice, New: h.New, Offers: h.Offers,
			// A card is hunted while the player does not hold it.
			Thumb: newCardThumb(h.Card, false, setOf),
		}
		if h.SetCode != "" {
			view.Cards[i].Href = binderHref(h.SetCode, binderFilter{}, h.Card.ID)
		}
	}
	return view
}

func newHuntedNavView(hunted []play.Hunted) *huntedNavView {
	nav := &huntedNavView{}
	for _, h := range hunted {
		if h.New {
			nav.New++
		}
	}
	return nav
}

// huntedKeys is the cards hunted, to mark their slots.
func huntedKeys(hunted []play.Hunted) map[domain.CardKey]bool {
	keys := make(map[domain.CardKey]bool, len(hunted))
	for _, h := range hunted {
		keys[domain.CardKey{ID: h.Card.ID, Finish: h.Finish}] = true
	}
	return keys
}
