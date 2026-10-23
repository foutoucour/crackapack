package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

const (
	// confirmed is the value of the field a sale carries once the player has agreed to empty a slot.
	confirmed = "yes"
	// backLabel is the text of the way back from a refusal to the binder.
	backLabel = "Back to the binder"
)

// noticeBox keeps what the last purchase or sale tells until the binder page it redirects to shows
// it once. The game has one player, so one notice is enough.
type noticeBox struct {
	mu     sync.Mutex
	notice noticeView
}

func (b *noticeBox) put(n noticeView) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.notice = n
}

// take returns the notice and forgets it.
func (b *noticeBox) take() noticeView {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := b.notice
	b.notice = noticeView{}
	return n
}

// binderHref is the address of the binder of set, filtered on rarity when it is not empty, at the
// slot of card when it is not empty. A set it does not name leads to the first binder.
func binderHref(set, rarity string, card domain.CardID) string {
	if set == "" {
		return "/collection"
	}
	href := collectionHref(set, rarity)
	if card != "" {
		href += "#" + (&url.URL{Fragment: slotAnchor(card)}).EscapedFragment()
	}
	return href
}

func slotAnchor(card domain.CardID) string { return "c-" + string(card) }

// cardRefOf reads the card, its finish and the binder to come back to from a form of the Counter.
func cardRefOf(r *http.Request) (cardRef, bool) {
	ref := cardRef{Set: r.PostFormValue("set"), ID: domain.CardID(r.PostFormValue("card")), Finish: r.PostFormValue("finish"), Filter: r.PostFormValue("rarity")}
	switch domain.Finish(ref.Finish) {
	case domain.FinishNonFoil, domain.FinishFoil:
		return ref, ref.ID != ""
	default:
		return ref, false
	}
}

func (c cardRef) finish() domain.Finish { return domain.Finish(c.Finish) }

func (c cardRef) back() string { return binderHref(c.Set, c.Filter, c.ID) }

// describe names the card of c for a notice: its name, and "(foil)" for a foil.
func (h *handler) describe(c cardRef) string {
	card, _ := h.game.Card(c.ID)
	if c.finish() == domain.FinishFoil {
		return card.Name + " (foil)"
	}
	return card.Name
}

func (h *handler) buy(w http.ResponseWriter, r *http.Request) {
	ref, ok := cardRefOf(r)
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown card: buy from a slot of a binder.", binderHref(ref.Set, ref.Filter, ""))
		return
	}
	purchase, err := h.game.Buy(ref.ID, ref.finish())
	if err != nil {
		h.counterFailed(w, err, ref)
		return
	}
	h.notices.put(noticeView{Text: fmt.Sprintf("Bought %s for %s.", h.describe(ref), coinsText(purchase.Paid)), Unlocked: purchase.Unlocked})
	http.Redirect(w, r, ref.back(), http.StatusSeeOther)
}

// sell sells qty copies of a card. A sale that would leave the slot empty asks first, on a page of
// its own whose form sells with confirm set, so that no script is needed.
func (h *handler) sell(w http.ResponseWriter, r *http.Request) {
	ref, ok := cardRefOf(r)
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown card: sell from a slot of a binder.", binderHref(ref.Set, ref.Filter, ""))
		return
	}
	qty, err := strconv.Atoi(r.PostFormValue("qty"))
	if err != nil || qty < 1 {
		h.refuse(w, http.StatusBadRequest, "Pick a quantity of at least 1.", ref.back())
		return
	}
	if r.PostFormValue("confirm") != confirmed && h.game.SaleEmptiesSlot(ref.ID, ref.finish(), qty) {
		h.confirmSale(w, ref, qty)
		return
	}
	paid, err := h.game.Sell(ref.ID, ref.finish(), qty)
	if err != nil {
		h.counterFailed(w, err, ref)
		return
	}
	h.notices.put(noticeView{Text: fmt.Sprintf("Sold %d × %s for %s.", qty, h.describe(ref), coinsText(paid))})
	http.Redirect(w, r, ref.back(), http.StatusSeeOther)
}

func (h *handler) confirmSale(w http.ResponseWriter, ref cardRef, qty int) {
	quote, err := h.game.CounterQuote(ref.ID, ref.finish())
	if err != nil {
		h.counterFailed(w, err, ref)
		return
	}
	card, _ := h.game.Card(ref.ID)
	h.render(w, http.StatusOK, "confirm", confirmView{
		cardRef: ref, Name: card.Name, Foil: ref.finish() == domain.FinishFoil, Qty: qty,
		Total: quote.SellPrice * domain.Coins(qty), BackHref: ref.back(),
	})
}

// sellDuplicates sells the copies above a playset of a binder, for the total the binder page showed.
func (h *handler) sellDuplicates(w http.ResponseWriter, r *http.Request) {
	set, rarity := r.PostFormValue("set"), r.PostFormValue("rarity")
	back := binderHref(set, rarity, "")
	paid, err := h.game.SellDuplicates(set)
	if errors.Is(err, play.ErrNothingToSell) {
		h.refuse(w, http.StatusConflict, "You have no duplicates above a playset in this binder.", back)
		return
	}
	if err != nil {
		h.counterFailed(w, err, cardRef{Set: set, Filter: rarity})
		return
	}
	h.notices.put(noticeView{Text: "Sold your duplicates for " + coinsText(paid) + "."})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// duplicatesOf is what the binder of set offers to sell in bulk. A binder with a duplicate that
// the Counter does not trade offers nothing, since the Counter would refuse the lot.
func (h *handler) duplicatesOf(set, rarity string) (duplicatesView, error) {
	d, err := h.game.Duplicates(set)
	switch {
	case errors.Is(err, play.ErrNotTraded):
		return duplicatesView{}, nil
	case err != nil:
		return duplicatesView{}, err
	}
	return duplicatesView{Offered: true, Set: set, Filter: rarity, Copies: d.Copies, Value: d.Value}, nil
}

// counterFailed answers a refusal of the Counter with the status that says what went wrong, and a
// way back to the slot the player came from. The game changed nothing.
func (h *handler) counterFailed(w http.ResponseWriter, err error, ref cardRef) {
	back := ref.back()
	switch {
	case errors.Is(err, play.ErrNoCounter):
		h.refuse(w, http.StatusNotFound, "This game has no Counter.", back)
	case errors.Is(err, play.ErrUnknownCard):
		h.refuse(w, http.StatusBadRequest, "Unknown card: pick one from a binder.", back)
	case errors.Is(err, play.ErrUnknownSet):
		h.refuse(w, http.StatusBadRequest, "Unknown set: pick one of the sets on the home page.", back)
	case errors.Is(err, play.ErrLocked):
		h.refuse(w, http.StatusConflict, "This card is in a locked set: fill a binder of an open set to unlock more.", back)
	case errors.Is(err, play.ErrNotEnoughCoins):
		h.refuse(w, http.StatusConflict, h.shortOfCoins(ref), back)
	case errors.Is(err, play.ErrOutOfStock):
		h.refuse(w, http.StatusConflict, "The Counter has no copy of this card right now.", back)
	case errors.Is(err, play.ErrNotTraded):
		h.refuse(w, http.StatusConflict, "The Counter does not trade this card.", back)
	case errors.Is(err, play.ErrNothingToSell):
		h.refuse(w, http.StatusConflict, "You do not hold that many copies of this card.", back)
	case errors.Is(err, play.ErrCounterBroke):
		h.refuse(w, http.StatusConflict, "The Counter cannot pay for these cards right now.", back)
	default:
		h.playFailed(w, err)
	}
}

// shortOfCoins says what the card costs and what the player has.
func (h *handler) shortOfCoins(ref cardRef) string {
	quote, err := h.game.CounterQuote(ref.ID, ref.finish())
	if err != nil {
		return "Not enough coins for this card."
	}
	return fmt.Sprintf("Not enough coins: this card costs %d and you have %d. Sell cards to the Counter to earn more.", quote.BuyPrice, h.game.Balance())
}

// refuse shows message with status, and a link back to the binder at href.
func (h *handler) refuse(w http.ResponseWriter, status int, message, href string) {
	h.render(w, status, "error", errorView{Message: message, BackHref: href, BackLabel: backLabel})
}
