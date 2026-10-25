package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// reservedMessage tells why a card cannot be sold or given: the market holds it.
const reservedMessage = "This card is held for the market, for a listing or a counter: unlist it or wait for the NPC's reply, then try again."

// lockedCopyMessage tells why a card cannot be sold or given: a milestone locks the copy.
const lockedCopyMessage = "A milestone locks this copy: you earned it by holding the card, so you keep it. Only the copies above it can be sold or traded."

// refusal is what the player is told when the market refuses an action, with the status of it.
type refusal struct {
	status  int
	message string
}

// marketRefusal returns what to tell the player about err, and false for an error that the market
// does not refuse by design. ErrNotEnoughCoins is told by the handler, which knows the balance.
func marketRefusal(err error) (refusal, bool) {
	refusals := []struct {
		err error
		refusal
	}{
		{play.ErrNoMarket, refusal{http.StatusNotFound, "This game has no market."}},
		{play.ErrNotOnMarket, refusal{http.StatusNotFound, "This is no longer on the market: it was answered, taken back or it lapsed."}},
		{play.ErrUnknownCard, refusal{http.StatusBadRequest, "Unknown card: pick one from a binder."}},
		{play.ErrLocked, refusal{http.StatusConflict, "This card is in a locked set: fill a binder of an open set to unlock more."}},
		{play.ErrNobodyHasIt, refusal{http.StatusConflict, "No NPC has this card to trade right now."}},
		{play.ErrAlreadyAsked, refusal{http.StatusConflict, "You are already asking around for this card."}},
		{play.ErrAlreadyListed, refusal{http.StatusConflict, "This card is on the market already."}},
		{play.ErrReserved, refusal{http.StatusConflict, reservedMessage}},
		{play.ErrCopyLocked, refusal{http.StatusConflict, lockedCopyMessage}},
		{play.ErrNotHeld, refusal{http.StatusConflict, "You do not hold this card, or not as many copies of it."}},
		{play.ErrInvalidPrice, refusal{http.StatusBadRequest, "A price is at least 1 coin."}},
		{play.ErrCoinsOnly, refusal{http.StatusBadRequest, "A counter to an offer asks for coins, with no card."}},
		{play.ErrNothingOffered, refusal{http.StatusBadRequest, "Offer coins or cards in your counter."}},
		{play.ErrNegative, refusal{http.StatusBadRequest, "A counter has no negative coins or copies."}},
		{play.ErrTooManyCards, refusal{http.StatusBadRequest, "Too many cards: a trade holds only a few cards on a side."}},
		{play.ErrAwaitingReply, refusal{http.StatusConflict, "You countered this answer: wait for the NPC's reply."}},
		{play.ErrGone, refusal{http.StatusConflict, "The NPC no longer has the cards, so the answer is over."}},
	}
	for _, r := range refusals {
		if errors.Is(err, r.err) {
			return r.refusal, true
		}
	}
	return refusal{}, false
}

// marketFailed answers a refusal of the market with the status that says what went wrong, and a way
// back to where the player came from. The game changed nothing, but for an answer that ended
// because the NPC no longer had its cards.
func (h *handler) marketFailed(w http.ResponseWriter, err error, back string) {
	if errors.Is(err, play.ErrNotEnoughCoins) {
		h.refuse(w, http.StatusConflict, fmt.Sprintf("Not enough coins: you have %d. Sell cards to the Counter to earn more.", h.game.Balance()), back)
		return
	}
	if errors.Is(err, play.ErrReserved) {
		h.refuseReserved(w, back)
		return
	}
	if r, ok := marketRefusal(err); ok {
		h.refuse(w, r.status, r.message, back)
		return
	}
	h.playFailed(w, err)
}

// refuseReserved tells that the market holds the card, with links to the page the player came from
// and to the market, where they can free it.
func (h *handler) refuseReserved(w http.ResponseWriter, back string) {
	h.render(w, http.StatusConflict, "error", errorView{
		Message: reservedMessage, BackHref: back, BackLabel: backLabelOf(back),
		AltHref: marketPath, AltLabel: "Go to the market",
	})
}

// marketIDOf reads the ID of a request, a listing or an answer from field.
func marketIDOf(r *http.Request, field string) (domain.MarketID, bool) {
	n, err := strconv.Atoi(r.FormValue(field))
	return domain.MarketID(n), err == nil && n > 0
}

// market shows everything open on the market and how the latest answers ended.
func (h *handler) market(w http.ResponseWriter, _ *http.Request) {
	v, err := h.game.Market()
	if err != nil {
		h.marketFailed(w, err, "/")
		return
	}
	view := newMarketView(v, h.game.SetOf)
	view.Notice = h.notices.take()
	h.render(w, http.StatusOK, "market", view)
}

// marketIndex is the market as a binder page looks it up, nil in a game without one.
func (h *handler) marketIndex() (*marketIndex, error) {
	v, err := h.game.Market()
	if errors.Is(err, play.ErrNoMarket) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return newMarketIndex(v, h.game.Reserved, h.game.OnNPCShelf), nil
}

// askAround asks the NPCs that have a card what they want for it, and comes back to the slot.
func (h *handler) askAround(w http.ResponseWriter, r *http.Request) {
	back := backOf(r).href()
	ref, ok := cardRefOf(r)
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown card: ask around from a slot of a binder.", back)
		return
	}
	if _, err := h.game.AskAround(ref.ID, ref.finish()); err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{Text: "Asking around for " + h.describe(ref) + "."})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// withdraw stops asking around for a card.
func (h *handler) withdraw(w http.ResponseWriter, r *http.Request) {
	back := backOf(r).href()
	id, ok := marketIDOf(r, "request")
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown request: stop asking from the slot or the market page.", back)
		return
	}
	v, err := h.game.Market()
	if err != nil {
		h.marketFailed(w, err, back)
		return
	}
	if err := h.game.Withdraw(id); err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{Text: "Stopped asking around for " + requestName(v, id) + "."})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func requestName(v play.MarketView, id domain.MarketID) string {
	for _, r := range v.Requests {
		if r.ID == id {
			return cardName(r.Card, r.Finish)
		}
	}
	return "the card"
}

func listingName(v play.MarketView, id domain.MarketID) string {
	for _, l := range v.Listings {
		if l.ID == id {
			return cardName(l.Card, l.Finish)
		}
	}
	return "the card"
}

// list puts a held card on the market at an asking price.
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	back := backOf(r).href()
	ref, ok := cardRefOf(r)
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown card: list a card from a slot of a binder.", back)
		return
	}
	price, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("price")), 10, 64)
	if err != nil {
		h.refuse(w, http.StatusBadRequest, "Give an asking price in whole coins.", back)
		return
	}
	if _, err := h.game.List(ref.ID, ref.finish(), domain.Coins(price)); err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{Text: fmt.Sprintf("Listed %s for %s.", h.describe(ref), coinsText(domain.Coins(price)))})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// unlist takes a listed card off the market.
func (h *handler) unlist(w http.ResponseWriter, r *http.Request) {
	back := backOf(r).href()
	id, ok := marketIDOf(r, "listing")
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown listing: unlist from the slot or the market page.", back)
		return
	}
	v, err := h.game.Market()
	if err != nil {
		h.marketFailed(w, err, back)
		return
	}
	if err := h.game.Unlist(id); err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{Text: "Took " + listingName(v, id) + " off the market."})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// answerAction reads the answer of a form and the page to come back to, and looks the answer up in
// the market as it stands. It has answered the request itself, and returns false, for an answer
// that is not a number or a market that fails. An answer the market no longer has is for the game
// to refuse, so found is false then.
func (h *handler) answerAction(w http.ResponseWriter, r *http.Request) (id domain.MarketID, found foundAnswer, ok bool) {
	back := backOf(r).href()
	id, ok = marketIDOf(r, "answer")
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown answer: answer from the slot or the market page.", back)
		return 0, foundAnswer{}, false
	}
	v, err := h.game.Market()
	if err != nil {
		h.marketFailed(w, err, back)
		return 0, foundAnswer{}, false
	}
	found, _ = findAnswer(v, id)
	return id, found, true
}

// accept takes an answer at once: it buys the card at the asking price, or sells the listed card
// for the offer.
func (h *handler) accept(w http.ResponseWriter, r *http.Request) {
	id, answer, ok := h.answerAction(w, r)
	if !ok {
		return
	}
	back := backOf(r).href()
	deal, err := h.game.AcceptAnswer(id)
	if err != nil {
		h.marketFailed(w, err, back)
		return
	}
	notice := noticeView{Unlocked: deal.Unlocked}
	if answer.Ask {
		notice.Text = fmt.Sprintf("Bought %s from %s for %s.", cardName(answer.Card, answer.Finish), npcLabel(answer.Answer), termsText(answer.Coins, answer.Cards))
	} else {
		notice.Text = fmt.Sprintf("Sold %s to %s for %s.", cardName(answer.Card, answer.Finish), npcLabel(answer.Answer), termsText(answer.Coins, answer.Cards))
	}
	h.notices.put(notice)
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// decline turns an answer down.
func (h *handler) decline(w http.ResponseWriter, r *http.Request) {
	id, answer, ok := h.answerAction(w, r)
	if !ok {
		return
	}
	back := backOf(r).href()
	if err := h.game.DeclineAnswer(id); err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{Text: fmt.Sprintf("Declined the answer of %s for %s.", npcLabel(answer.Answer), cardName(answer.Card, answer.Finish))})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// counterForm shows the page where the player counters an answer: coins, and on an asking price the
// spare cards too.
func (h *handler) counterForm(w http.ResponseWriter, r *http.Request) {
	back := backOf(r)
	id, answer, ok := h.answerAction(w, r)
	if !ok {
		return
	}
	switch {
	case answer.ID != id:
		h.marketFailed(w, play.ErrNotOnMarket, back.href())
		return
	case answer.Counter != nil:
		h.marketFailed(w, play.ErrAwaitingReply, back.href())
		return
	}
	var spare []play.CardLine
	if answer.Ask {
		var err error
		if spare, err = h.game.Spare(); err != nil {
			h.marketFailed(w, err, back.href())
			return
		}
	}
	h.render(w, http.StatusOK, "market_counter", newCounterFormView(answer, spare, back))
}

// counterAnswer sends the player's counter to an answer. The NPC replies after its delay.
func (h *handler) counterAnswer(w http.ResponseWriter, r *http.Request) {
	id, answer, ok := h.answerAction(w, r)
	if !ok {
		return
	}
	back := backOf(r).href()
	coins, cards, ok := counterTerms(r)
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Give coins and copies as whole numbers.", back)
		return
	}
	if err := h.game.CounterAnswer(id, coins, cards); err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{Text: fmt.Sprintf("Countered the answer of %s for %s: it replies after its delay.", npcLabel(answer.Answer), cardName(answer.Card, answer.Finish))})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// counterTerms reads the coins and the cards of a counter from the form: an empty field is 0, and
// each give:<card>:<finish> field holds the copies of that card given.
func counterTerms(r *http.Request) (domain.Coins, []domain.TradeLine, bool) {
	coins, ok := wholeNumber(r.PostFormValue("coins"), 64)
	if !ok {
		return 0, nil, false
	}
	var lines []domain.TradeLine
	for field, values := range r.PostForm {
		spec, isGive := strings.CutPrefix(field, givePrefix)
		sep := strings.LastIndex(spec, ":")
		if !isGive || sep < 0 || len(values) == 0 {
			continue
		}
		qty, ok := wholeNumber(values[0], 32)
		if !ok {
			return 0, nil, false
		}
		if qty != 0 {
			lines = append(lines, domain.TradeLine{Key: domain.CardKey{ID: domain.CardID(spec[:sep]), Finish: domain.Finish(spec[sep+1:])}, Qty: int(qty)})
		}
	}
	return domain.Coins(coins), lines, true
}

// wholeNumber reads a field that is empty, which is 0, or a whole number that fits bits bits.
func wholeNumber(field string, bits int) (int64, bool) {
	field = strings.TrimSpace(field)
	if field == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(field, 10, bits)
	return n, err == nil
}

// marketNav is the header's link to the market with the answers waiting, nil in a game without a
// market, which has no link.
func (h *handler) marketNav() *marketNavView {
	v, err := h.game.Market()
	if err != nil {
		if !errors.Is(err, play.ErrNoMarket) {
			h.log.Error("market header", "error", err)
		}
		return nil
	}
	return newMarketNavView(v.Waiting)
}
