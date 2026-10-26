package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

const (
	// marketPath is the market page, where the market's forms come back to when they did not start
	// from a slot of a binder.
	marketPath = "/market"
	// fromMarket is the value of the "from" field of a form that was on the market page.
	fromMarket = "market"
	// backToMarketLabel is the text of the way back from a refusal to the market page.
	backToMarketLabel = "Back to the market"
)

// backView is where a form of the market sends the player once it is done: the market page, or the
// slot of a card in a binder.
type backView struct {
	Market bool
	Set    string
	Card   domain.CardID
	Filter binderFilter
}

func (b backView) href() string {
	if b.Market {
		return marketPath
	}
	return binderHref(b.Set, b.Filter, b.Card)
}

// query is the fields that make a link carry b, as the forms of the market do with hidden fields.
func (b backView) query() url.Values {
	if b.Market {
		return url.Values{"from": {fromMarket}}
	}
	q := b.Filter.values()
	q.Set("set", b.Set)
	q.Set("card", string(b.Card))
	return q
}

// backOf reads where a form or a link of the market came from. Without a set it is the market page.
func backOf(r *http.Request) backView {
	set := r.FormValue("set")
	if set == "" || r.FormValue("from") == fromMarket {
		return backView{Market: true}
	}
	return backView{Set: set, Card: domain.CardID(r.FormValue("card")), Filter: binderFilterOf(r)}
}

// backLabelOf is the text of the link back to href.
func backLabelOf(href string) string {
	if href == marketPath {
		return backToMarketLabel
	}
	return backLabel
}

// marketNavView is the link to the market page in the header, with the answers waiting for the
// player.
type marketNavView struct {
	Waiting int
	// Label says what the number is to a screen reader.
	Label string
}

func newMarketNavView(waiting int) *marketNavView {
	nav := &marketNavView{Waiting: waiting, Label: "answers waiting"}
	if waiting == 1 {
		nav.Label = "answer waiting"
	}
	return nav
}

// marketIndex is the market as the slots of a binder look it up: by card and finish. A nil one is
// a game without a market.
type marketIndex struct {
	now      time.Time
	requests map[domain.CardKey]play.Request
	listings map[domain.CardKey]play.Listing
	// reserved is the copies of a card the market holds.
	reserved func(domain.CardID, domain.Finish) int
	// onShelf tells whether an NPC that trades has a card.
	onShelf func(domain.CardID, domain.Finish) bool
}

func newMarketIndex(v play.MarketView, reserved func(domain.CardID, domain.Finish) int, onShelf func(domain.CardID, domain.Finish) bool) *marketIndex {
	idx := &marketIndex{
		now:      v.Now,
		requests: make(map[domain.CardKey]play.Request, len(v.Requests)),
		listings: make(map[domain.CardKey]play.Listing, len(v.Listings)),
		reserved: reserved,
		onShelf:  onShelf,
	}
	for _, r := range v.Requests {
		idx.requests[domain.CardKey{ID: r.Card.ID, Finish: r.Finish}] = r
	}
	for _, l := range v.Listings {
		idx.listings[domain.CardKey{ID: l.Card.ID, Finish: l.Finish}] = l
	}
	return idx
}

// huntView is the market's side of a slot the player can still fill: the request the player made
// for its card, or the button to make one.
type huntView struct {
	cardRef
	Request *requestView
	// OnShelf is true when an NPC that trades has the card, so that asking around finds someone.
	OnShelf bool
}

// marketSaleView is the market's side of one finish of a held card: the listing, or the form to
// make one.
type marketSaleView struct {
	cardRef
	Foil    bool
	Listing *listingView
	// Held is the copies held for a counter, when the card is not listed.
	Held int
	// Price is the asking price the form starts from, the Counter's selling price, and 0 for none.
	Price domain.Coins
}

// hunt fills the market's side of the slot of the card of ref.
func (idx *marketIndex) hunt(ref cardRef) *huntView {
	hunt := &huntView{cardRef: ref, OnShelf: idx.onShelf(ref.ID, ref.finish())}
	if r, ok := idx.requests[domain.CardKey{ID: ref.ID, Finish: ref.finish()}]; ok {
		view := idx.requestView(r, ref, backView{Set: ref.Set, Card: ref.ID, Filter: ref.Filter})
		hunt.Request = &view
	}
	return hunt
}

// sale fills the market's side of the finish of a held card, which the Counter sells at asks.
func (idx *marketIndex) sale(ref cardRef, foil bool, asks domain.Coins) marketSaleView {
	sale := marketSaleView{cardRef: ref, Foil: foil, Price: asks}
	key := domain.CardKey{ID: ref.ID, Finish: ref.finish()}
	if l, ok := idx.listings[key]; ok {
		view := idx.listingView(l, ref, backView{Set: ref.Set, Card: ref.ID, Filter: ref.Filter})
		sale.Listing = &view
		return sale
	}
	sale.Held = idx.reserved(ref.ID, ref.finish())
	return sale
}

// requestView is a request of the player: asking around for a card, with the answers so far.
type requestView struct {
	cardRef
	RequestID int
	Name      string
	Status    string
	// Over is true when no answer is left to come or to take: the player can ask again.
	Over    bool
	Answers []answerView
	Back    backView
	// Thumb is the card on the market page, which the other pages do not show.
	Thumb cardThumb
}

func (idx *marketIndex) requestView(r play.Request, ref cardRef, back backView) requestView {
	return requestView{
		cardRef:   ref,
		RequestID: int(r.ID),
		Name:      cardName(r.Card, r.Finish),
		Status:    requestStatus(r.Asked, r.Pending, len(r.Answers)),
		Over:      r.Pending == 0 && len(r.Answers) == 0,
		Answers:   idx.answerViews(r.Answers, true, cardName(r.Card, r.Finish), back),
		Back:      back,
	}
}

// requestStatus says how far the NPCs asked have answered: "3 NPCs asked, no answer yet".
func requestStatus(asked, pending, answers int) string {
	asking := plural(asked, "NPC") + " asked"
	switch {
	case pending > 0 && answers == 0:
		return asking + ", no answer yet"
	case pending > 0:
		return fmt.Sprintf("%s, %d still to answer", asking, pending)
	case answers > 0:
		return asking + ", all have answered"
	default:
		return asking + ", none of the answers is left"
	}
}

// listingView is a card the player put on the market, with the offers so far.
type listingView struct {
	cardRef
	ListingID int
	Name      string
	Price     domain.Coins
	// Status is "No offers yet." while there is none.
	Status  string
	Answers []answerView
	Back    backView
	Thumb   cardThumb
}

func (idx *marketIndex) listingView(l play.Listing, ref cardRef, back backView) listingView {
	view := listingView{
		cardRef:   ref,
		ListingID: int(l.ID),
		Name:      cardName(l.Card, l.Finish),
		Price:     l.Price,
		Answers:   idx.answerViews(l.Answers, false, cardName(l.Card, l.Finish), back),
		Back:      back,
	}
	if len(l.Answers) == 0 {
		view.Status = "No offers yet."
	}
	return view
}

// answerView is an NPC's answer to a request (its asking price) or to a listing (its offer).
type answerView struct {
	AnswerID int
	// Who is the NPC and its personality, and Terms what it asks or offers.
	Who   string
	Terms string
	// Card names the card answered about, for the buttons read by a screen reader.
	Card string
	// Lapses says when the answer is gone unless the player takes it.
	Lapses string
	// Countered says the player's counter and when the NPC replies, and is empty for an answer the
	// player can still accept, counter or decline.
	Countered   string
	CounterHref string
	Back        backView
}

func (idx *marketIndex) answerViews(answers []play.Answer, ask bool, card string, back backView) []answerView {
	out := make([]answerView, len(answers))
	for i, a := range answers {
		verb := "offers"
		if ask {
			verb = "asks"
		}
		out[i] = answerView{
			AnswerID:    int(a.ID),
			Who:         npcLabel(a),
			Terms:       verb + " " + termsText(a.Coins, a.Cards),
			Card:        card,
			CounterHref: counterHref(a.ID, back),
			Back:        back,
		}
		if c := a.Counter; c != nil {
			out[i].Countered = fmt.Sprintf("You countered with %s. %s replies in %s.", termsText(c.Coins, c.Cards), npcName(a), humanDuration(c.ReplyAt.Sub(idx.now)))
		} else {
			out[i].Lapses = "Lapses in " + humanDuration(a.Expires.Sub(idx.now))
		}
	}
	return out
}

func counterHref(id domain.MarketID, back backView) string {
	q := back.query()
	q.Set("answer", fmt.Sprint(int(id)))
	return "/counter-offer?" + q.Encode()
}

// npcName is what the NPC that answered is called, and "NPC 3" for one with no name, numbered from
// 1 as a player counts.
func npcName(a play.Answer) string {
	if a.Name != "" {
		return a.Name
	}
	return fmt.Sprintf("NPC %d", a.NPC+1)
}

// npcLabel is who answered: the NPC and its personality, "Ajani Goldmane (playset hunter)".
func npcLabel(a play.Answer) string {
	if a.Archetype == "" {
		return npcName(a)
	}
	return fmt.Sprintf("%s (%s)", npcName(a), strings.ReplaceAll(a.Archetype, "_", " "))
}

// cardName is the name of a card for the player: "(foil)" after that of a foil.
func cardName(card domain.Card, finish domain.Finish) string {
	if finish == domain.FinishFoil {
		return card.Name + " (foil)"
	}
	return card.Name
}

// termsText says coins and cards together: "83 coins", "Ancestral Recall ×2", "10 coins and Bolt ×1".
func termsText(coins domain.Coins, cards []play.CardLine) string {
	var parts []string
	if coins > 0 {
		parts = append(parts, coinsText(coins))
	}
	for _, l := range cards {
		parts = append(parts, fmt.Sprintf("%s ×%d", cardName(l.Card, l.Finish), l.Qty))
	}
	return strings.Join(parts, " and ")
}

// dealFailedText tells a deal that could not go through, with the NPC and the card.
const dealFailedText = "The deal with %s for %s fell through: a card was gone, or the coins were not there."

// resultView is how an answer ended, with when.
type resultView struct {
	Text string
	When string
}

func newResultView(r play.Result, now time.Time) resultView {
	name := cardName(r.Card, r.Finish)
	who := npcLabel(r.Answer)
	// An answer the player countered ended on the terms of the counter.
	coins, cards := r.Answer.Coins, r.Answer.Cards
	if c := r.Answer.Counter; c != nil {
		coins, cards = c.Coins, c.Cards
	}
	ask := r.Kind == domain.AnswerAsk
	var text string
	switch r.Outcome {
	case domain.MarketAccepted:
		if ask {
			text = fmt.Sprintf("Bought %s from %s for %s.", name, who, termsText(coins, cards))
		} else {
			text = fmt.Sprintf("Sold %s to %s for %s.", name, who, termsText(coins, cards))
		}
	case domain.MarketRefused:
		text = fmt.Sprintf("%s refused your counter of %s for %s.", who, termsText(coins, cards), name)
	case domain.MarketExpired:
		if ask {
			text = fmt.Sprintf("The price of %s for %s lapsed.", who, name)
		} else {
			text = fmt.Sprintf("The offer of %s for %s lapsed.", who, name)
		}
	case domain.MarketFailed:
		fallthrough
	default:
		text = fmt.Sprintf(dealFailedText, who, name)
	}
	return resultView{Text: text, When: humanAgo(now.Sub(r.At))}
}

// marketView is the market page.
type marketView struct {
	Notice   noticeView
	Requests []requestView
	Listings []listingView
	Results  []resultView
}

func newMarketView(v play.MarketView, setOf setFinder) marketView {
	idx := newMarketIndex(v, nil, nil)
	back := backView{Market: true}
	var view marketView
	for _, r := range v.Requests {
		request := idx.requestView(r, cardRef{ID: r.Card.ID, Finish: string(r.Finish)}, back)
		// A card is asked around for while the player does not hold it.
		request.Thumb = newCardThumb(r.Card, false, setOf)
		view.Requests = append(view.Requests, request)
	}
	for _, l := range v.Listings {
		listing := idx.listingView(l, cardRef{ID: l.Card.ID, Finish: string(l.Finish)}, back)
		listing.Thumb = newCardThumb(l.Card, true, setOf)
		view.Listings = append(view.Listings, listing)
	}
	for _, r := range v.Results {
		view.Results = append(view.Results, newResultView(r, v.Now))
	}
	return view
}

// counterFormView is the page where the player counters an answer.
type counterFormView struct {
	AnswerID int
	Who      string
	// Summary says what the NPC asks or offers for the card.
	Summary string
	// Ask is true for an asking price, which the player can pay in cards as well as coins.
	Ask bool
	// Coins is the coins the form starts from.
	Coins domain.Coins
	Spare []spareView
	Back  backView
	// BackHref is where the player goes without countering.
	BackHref string
}

// spareView is a card the player can add to a counter, with the most it can add.
type spareView struct {
	Field string
	Text  string
	Max   int
}

// giveField is the name of the field of a counter that holds the copies of key given.
func giveField(key domain.CardKey) string {
	return givePrefix + string(key.ID) + ":" + string(key.Finish)
}

const givePrefix = "give:"

func newCounterFormView(a foundAnswer, spare []play.CardLine, back backView) counterFormView {
	view := counterFormView{
		AnswerID: int(a.ID),
		Who:      npcLabel(a.Answer),
		Ask:      a.Ask,
		Coins:    a.Coins,
		Back:     back,
		BackHref: back.href(),
	}
	if a.Ask {
		view.Summary = fmt.Sprintf("asks %s for %s", termsText(a.Coins, a.Cards), cardName(a.Card, a.Finish))
	} else {
		view.Summary = fmt.Sprintf("offers %s for your %s", termsText(a.Coins, a.Cards), cardName(a.Card, a.Finish))
	}
	for _, l := range spare {
		key := domain.CardKey{ID: l.Card.ID, Finish: l.Finish}
		view.Spare = append(view.Spare, spareView{Field: giveField(key), Text: cardName(l.Card, l.Finish), Max: l.Qty})
	}
	return view
}

// foundAnswer is an answer of the market with the card it is about.
type foundAnswer struct {
	play.Answer
	Card   domain.Card
	Finish domain.Finish
	// Ask is true for the answer to a request, an asking price, and false for an offer.
	Ask bool
}

// findAnswer looks for the answer id in v.
func findAnswer(v play.MarketView, id domain.MarketID) (foundAnswer, bool) {
	for _, r := range v.Requests {
		for _, a := range r.Answers {
			if a.ID == id {
				return foundAnswer{Answer: a, Card: r.Card, Finish: r.Finish, Ask: true}, true
			}
		}
	}
	for _, l := range v.Listings {
		for _, a := range l.Answers {
			if a.ID == id {
				return foundAnswer{Answer: a, Card: l.Card, Finish: l.Finish}, true
			}
		}
	}
	return foundAnswer{}, false
}
