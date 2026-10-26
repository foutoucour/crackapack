package web

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

const (
	// browsePageSize is how many cards a page of the market shows: the shelves hold a card of every
	// set that is open, and a page of all of them would weigh as much as the largest binder.
	browsePageSize = 48
	// marketCardPath is the page of one card on the market.
	marketCardPath = "/market/card"
	// The tabs of the market pages.
	tabBrowse = "browse"
	tabTrades = "trades"
)

// browseFilter keeps part of what the NPCs sell: the cards the player hunts or lacks, of one colour
// group, rarity or set, whose name has a word, on one page. The zero value keeps every card. It
// travels through the links and forms of the market's pages, so that the player comes back to the
// cards as they left them.
type browseFilter struct {
	Hunted, Missing        bool
	Colour, Rarity, Set, Q string
	Page                   int
}

// browseFilterOf reads the filter of a page, a link or a form.
func browseFilterOf(r *http.Request) browseFilter {
	page, err := strconv.Atoi(r.FormValue("page"))
	if err != nil || page < 1 {
		page = 1
	}
	return browseFilter{
		Hunted: r.FormValue("hunted") == "1", Missing: r.FormValue("missing") == "1",
		Colour: r.FormValue("colour"), Rarity: r.FormValue("rarity"), Set: r.FormValue("set"),
		Q: strings.TrimSpace(r.FormValue("q")), Page: page,
	}
}

// keeps reports whether the card of sale is in the part of the shelves that f shows.
func (f browseFilter) keeps(sale play.ForSale) bool {
	switch {
	case f.Hunted && !sale.Hunted, f.Missing && !sale.Missing:
		return false
	case f.Rarity != "" && string(sale.Card.Rarity) != f.Rarity:
		return false
	case f.Colour != "" && string(domain.ColourGroupOf(sale.Card)) != f.Colour:
		return false
	case f.Set != "" && sale.SetCode != f.Set:
		return false
	}
	return f.Q == "" || strings.Contains(strings.ToLower(sale.Card.Name), strings.ToLower(f.Q))
}

// values is f as the fields of a query or a form, which has none for a part that is not filtered.
func (f browseFilter) values() url.Values {
	q := url.Values{}
	if f.Hunted {
		q.Set("hunted", "1")
	}
	if f.Missing {
		q.Set("missing", "1")
	}
	for name, value := range map[string]string{"colour": f.Colour, "rarity": f.Rarity, "set": f.Set, "q": f.Q} {
		if value != "" {
			q.Set(name, value)
		}
	}
	if f.Page > 1 {
		q.Set("page", strconv.Itoa(f.Page))
	}
	return q
}

// fieldView is a hidden field of a form.
type fieldView struct{ Name, Value string }

// Fields is f as the hidden fields that make a form of the market come back to the same cards.
func (f browseFilter) Fields() []fieldView {
	var fields []fieldView
	for name, values := range f.values() {
		fields = append(fields, fieldView{Name: name, Value: values[0]})
	}
	slices.SortFunc(fields, func(a, b fieldView) int { return strings.Compare(a.Name, b.Name) })
	return fields
}

func (f browseFilter) onPage(page int) browseFilter {
	f.Page = page
	return f
}

// browseHref is the market page with the cards f keeps.
func browseHref(f browseFilter) string {
	if q := f.values().Encode(); q != "" {
		return marketPath + "?" + q
	}
	return marketPath
}

// marketCardHref is the page of the card of ref, which links back to the cards f keeps.
func marketCardHref(ref cardRef, f browseFilter) string {
	q := f.values()
	q.Set("card", string(ref.ID))
	q.Set("finish", ref.Finish)
	return marketCardPath + "?" + q.Encode()
}

// marketTabsView is the tabs of the market pages: the cards the NPCs sell, and the trades the player
// started, with the answers waiting for the player.
type marketTabsView struct {
	Browse, Trades bool
	Waiting        int
	// Label says what the number is to a screen reader.
	Label string
}

func newMarketTabsView(active string, waiting int) marketTabsView {
	nav := newMarketNavView(waiting)
	return marketTabsView{Browse: active == tabBrowse, Trades: active == tabTrades, Waiting: waiting, Label: nav.Label}
}

// choiceView is an option of a select.
type choiceView struct {
	Value, Label string
	Selected     bool
}

// saleTile is a card the NPCs hold, as a tile of the market: its picture, the lowest price and the
// number of NPCs that hold it. A click opens the card's page.
type saleTile struct {
	Name            string
	Thumb           cardThumb
	Href            string
	From            domain.Coins
	Sellers         string
	Hunted, Missing bool
}

// browseView is the market page.
type browseView struct {
	Notice noticeView
	Tabs   marketTabsView
	Filter browseFilter
	// Sets, Colours and Rarities are the choices of the filter's selects.
	Sets, Colours, Rarities []choiceView
	Tiles                   []saleTile
	// Total is the cards the filter keeps, over all the pages.
	Total int
	// Prev and Next are the links to the other pages, empty when there is none.
	Prev, Next string
	// Filtered is true when the filter keeps part of the cards, so that an empty page says why.
	Filtered bool
}

func newBrowseView(sales []play.ForSale, f browseFilter, setOf setFinder) browseView {
	view := browseView{Filter: f, Filtered: f.values().Encode() != "", Sets: setChoices(sales, f.Set, setOf)}
	for _, g := range domain.ColourGroups() {
		view.Colours = append(view.Colours, choiceView{Value: string(g), Label: colourGroupLabel(g), Selected: string(g) == f.Colour})
	}
	for _, r := range []domain.Rarity{domain.RarityCommon, domain.RarityUncommon, domain.RarityRare, domain.RarityMythic} {
		view.Rarities = append(view.Rarities, choiceView{Value: string(r), Label: rarityLabel(r), Selected: string(r) == f.Rarity})
	}
	var kept []play.ForSale
	for _, s := range sales {
		if f.keeps(s) {
			kept = append(kept, s)
		}
	}
	slices.SortStableFunc(kept, func(a, b play.ForSale) int {
		return strings.Compare(cardName(a.Card, a.Finish), cardName(b.Card, b.Finish))
	})
	view.Total = len(kept)
	pages := max(1, (len(kept)+browsePageSize-1)/browsePageSize)
	page := min(f.Page, pages)
	view.Filter.Page = page
	from := (page - 1) * browsePageSize
	for _, s := range kept[from:min(len(kept), from+browsePageSize)] {
		view.Tiles = append(view.Tiles, newSaleTile(s, f.onPage(page), setOf))
	}
	if page > 1 {
		view.Prev = browseHref(f.onPage(page - 1))
	}
	if page < pages {
		view.Next = browseHref(f.onPage(page + 1))
	}
	return view
}

// setChoices are the sets whose cards are on the shelves, by name.
func setChoices(sales []play.ForSale, selected string, setOf setFinder) []choiceView {
	seen := map[string]bool{}
	var choices []choiceView
	for _, s := range sales {
		if s.SetCode == "" || seen[s.SetCode] {
			continue
		}
		seen[s.SetCode] = true
		set, _ := setOf(s.Card.ID)
		choices = append(choices, choiceView{Value: s.SetCode, Label: set.Name, Selected: s.SetCode == selected})
	}
	slices.SortFunc(choices, func(a, b choiceView) int { return strings.Compare(a.Label, b.Label) })
	return choices
}

func newSaleTile(s play.ForSale, f browseFilter, setOf setFinder) saleTile {
	ref := cardRef{ID: s.Card.ID, Finish: string(s.Finish)}
	return saleTile{
		Name:    cardName(s.Card, s.Finish),
		Thumb:   newCardThumb(s.Card, !s.Missing, setOf),
		Href:    marketCardHref(ref, f),
		From:    s.Sellers[0].Price,
		Sellers: plural(len(s.Sellers), "NPC"),
		Hunted:  s.Hunted, Missing: s.Missing,
	}
}

// sellerView is an NPC that holds a card, as the card's page lists it.
type sellerView struct {
	NPC   int
	Who   string
	Price domain.Coins
	Qty   int
	// MaxOffer is the highest price an offer can name: below the asking price.
	MaxOffer domain.Coins
}

func sellerLabelOf(s play.Seller) string { return npcLabelOf(s.Name, s.Archetype, s.NPC) }

func newSellerView(s play.Seller) sellerView {
	return sellerView{NPC: s.NPC, Who: sellerLabelOf(s), Price: s.Price, Qty: s.Qty, MaxOffer: s.Price - 1}
}

// marketCardView is the page of a card on the market.
type marketCardView struct {
	Notice noticeView
	Tabs   marketTabsView
	// CardID and Finish name the card for the forms.
	CardID domain.CardID
	Finish string
	Name   string
	Thumb  cardThumb
	Hunted bool
	Filter browseFilter
	// BackHref is the market page with the cards the player came from.
	BackHref string
	Sellers  []sellerView
}

func newMarketCardView(card domain.Card, finish domain.Finish, sales []play.ForSale, f browseFilter, setOf setFinder) marketCardView {
	ref := cardRef{ID: card.ID, Finish: string(finish)}
	view := marketCardView{CardID: card.ID, Finish: ref.Finish, Name: cardName(card, finish), Filter: f, BackHref: browseHref(f)}
	view.Thumb = newCardThumb(card, true, setOf)
	for _, s := range sales {
		if s.Card.ID != card.ID || s.Finish != finish {
			continue
		}
		view.Hunted = s.Hunted
		view.Thumb = newCardThumb(card, !s.Missing, setOf)
		for _, seller := range s.Sellers {
			view.Sellers = append(view.Sellers, newSellerView(seller))
		}
	}
	return view
}
