package web

import (
	"net/http"
	"net/url"

	"github.com/foutoucour/crackapack/internal/domain"
)

// binderFilter keeps part of a binder on its page: the cards of one rarity, of one colour group, or
// of both. The zero value keeps every card. It travels through the forms of the Counter and the
// market, so that a player comes back to the binder as they left it.
type binderFilter struct {
	Rarity string
	Colour string
}

// binderFilterOf reads the filter of a page, a link or a form.
func binderFilterOf(r *http.Request) binderFilter {
	return binderFilter{Rarity: r.FormValue("rarity"), Colour: r.FormValue("colour")}
}

// keeps reports whether card is in the part of the binder that f shows.
func (f binderFilter) keeps(card domain.Card) bool {
	if f.Rarity != "" && string(card.Rarity) != f.Rarity {
		return false
	}
	return f.Colour == "" || string(domain.ColourGroupOf(card)) == f.Colour
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
