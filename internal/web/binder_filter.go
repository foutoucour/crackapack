package web

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// The choices of the show filter, which keeps the cards the player can do something with.
const (
	// showCounter keeps the cards the Counter has in stock for the player to buy.
	showCounter = "counter"
	// showTrade keeps the cards an NPC has to trade, which the player can ask around for.
	showTrade = "trade"
	// showDuplicates keeps the cards held above a playset, the copies to sell or to trade.
	showDuplicates = "duplicates"
)

// binderFilter keeps part of a binder on its page: the cards of one rarity, of one colour group, the
// ones that Show names, or any mix of these. The zero value keeps every card. It travels through the
// forms of the Counter and the market, so that a player comes back to the binder as they left it.
type binderFilter struct {
	Rarity string
	Colour string
	Show   string
}

// showChoice is one entry of the show filter: the value of its field, and its label.
type showChoice struct {
	Value string
	Label string
}

// showChoices lists what the show filter offers in a game that has a Counter and a market, or not.
// The first, with no value, shows every card.
func showChoices(counter, market bool) []showChoice {
	choices := []showChoice{{"", "All cards"}, {showDuplicates, "Duplicates"}}
	if counter {
		choices = append(choices, showChoice{showCounter, "At the Counter"})
	}
	if market {
		choices = append(choices, showChoice{showTrade, "Available to trade"})
	}
	return choices
}

// offers reports whether the game that has a Counter and a market, or not, offers show.
func offers(show string, counter, market bool) bool {
	return slices.ContainsFunc(showChoices(counter, market), func(c showChoice) bool { return c.Value == show })
}

// binderFilterOf reads the filter of a page, a link or a form.
func binderFilterOf(r *http.Request) binderFilter {
	return binderFilter{Rarity: r.FormValue("rarity"), Colour: r.FormValue("colour"), Show: r.FormValue("show")}
}

// keeps reports whether card is in the part of the binder that f shows.
func (f binderFilter) keeps(card domain.Card) bool {
	if f.Rarity != "" && string(card.Rarity) != f.Rarity {
		return false
	}
	return f.Colour == "" || string(domain.ColourGroupOf(card)) == f.Colour
}

// keepsSlot reports whether slot is in the part of the binder that the show filter of f picks. It
// reads the slot, since what the Counter and the market offer comes with it.
func (f binderFilter) keepsSlot(slot slotView) bool {
	switch f.Show {
	case "":
		return true
	case showCounter:
		return slot.Buy.Buyable
	case showTrade:
		return slot.Hunt != nil && slot.Hunt.OnShelf
	case showDuplicates:
		return slot.Regular+slot.Foil > domain.PlaysetSize
	default:
		return false
	}
}

// values is f as the fields of a query or a form, which has none for a part that is not filtered.
func (f binderFilter) values() url.Values {
	q := url.Values{}
	if f.Rarity != "" {
		q.Set("rarity", f.Rarity)
	}
	if f.Colour != "" {
		q.Set("colour", f.Colour)
	}
	if f.Show != "" {
		q.Set("show", f.Show)
	}
	return q
}

// toggleRarity is f with rarity picked, or with no rarity when it is the one in force.
func (f binderFilter) toggleRarity(rarity string) binderFilter {
	if f.Rarity == rarity {
		rarity = ""
	}
	f.Rarity = rarity
	return f
}

// toggleColour is f with colour picked, or with no colour when it is the one in force.
func (f binderFilter) toggleColour(colour string) binderFilter {
	if f.Colour == colour {
		colour = ""
	}
	f.Colour = colour
	return f
}

// toggleShow is f with show picked, or with no show when it is the one in force.
func (f binderFilter) toggleShow(show string) binderFilter {
	if f.Show == show {
		show = ""
	}
	f.Show = show
	return f
}
