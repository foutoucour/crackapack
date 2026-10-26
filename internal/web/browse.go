package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// browse shows every card the NPCs hold on their shelves, a page of tiles at a time.
func (h *handler) browse(w http.ResponseWriter, r *http.Request) {
	sales, err := h.game.ForSale()
	if err != nil {
		h.marketFailed(w, err, "/")
		return
	}
	view := newBrowseView(sales, browseFilterOf(r), h.game.SetOf)
	view.Notice = h.notices.take()
	view.Tabs = h.marketTabs(tabBrowse)
	h.render(w, http.StatusOK, "market_browse", view)
}

// marketCard shows the NPCs that hold one card, each with its price, and what the player can do
// with it: buy now, or offer less.
func (h *handler) marketCard(w http.ResponseWriter, r *http.Request) {
	filter := browseFilterOf(r)
	ref := cardRef{ID: domain.CardID(r.FormValue("card")), Finish: r.FormValue("finish")}
	card, ok := h.game.Card(ref.ID)
	if !ok || !validFinish(ref.finish()) {
		h.refuse(w, http.StatusBadRequest, "Unknown card: pick one from the market.", browseHref(filter))
		return
	}
	sales, err := h.game.ForSale()
	if err != nil {
		h.marketFailed(w, err, "/")
		return
	}
	view := newMarketCardView(card, ref.finish(), sales, filter, h.game.SetOf)
	view.Notice = h.notices.take()
	view.Tabs = h.marketTabs(tabBrowse)
	h.render(w, http.StatusOK, "market_card", view)
}

func validFinish(f domain.Finish) bool { return f == domain.FinishNonFoil || f == domain.FinishFoil }

// shelfForm reads what a form of a card's page names: the card, the NPC, and the filter to come back
// with. It has answered the request itself, and returns false, for a form that names none.
func (h *handler) shelfForm(w http.ResponseWriter, r *http.Request) (ref cardRef, npc int, filter browseFilter, ok bool) {
	filter = browseFilterOf(r)
	ref = cardRef{ID: domain.CardID(r.PostFormValue("card")), Finish: r.PostFormValue("finish")}
	if _, known := h.game.Card(ref.ID); !known || !validFinish(ref.finish()) {
		h.refuse(w, http.StatusBadRequest, "Unknown card: pick one from the market.", browseHref(filter))
		return ref, 0, filter, false
	}
	npc, err := strconv.Atoi(r.PostFormValue("npc"))
	if err != nil || npc < 0 {
		h.refuse(w, http.StatusBadRequest, "Unknown NPC: pick one from the card's page.", marketCardHref(ref, filter))
		return ref, 0, filter, false
	}
	return ref, npc, filter, true
}

// buyNow buys a card from one NPC at its asking price, and comes back to the card's page.
func (h *handler) buyNow(w http.ResponseWriter, r *http.Request) {
	ref, npc, filter, ok := h.shelfForm(w, r)
	if !ok {
		return
	}
	back := marketCardHref(ref, filter)
	who := h.sellerLabel(ref, npc)
	purchase, err := h.game.BuyNow(npc, ref.ID, ref.finish())
	if err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{
		Text:     fmt.Sprintf("Bought %s from %s for %s.", h.describe(ref), who, coinsText(purchase.Paid)),
		Unlocked: purchase.Unlocked,
	})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// offerTo offers less than an NPC's price for its card, and comes back to the card's page.
func (h *handler) offerTo(w http.ResponseWriter, r *http.Request) {
	ref, npc, filter, ok := h.shelfForm(w, r)
	if !ok {
		return
	}
	back := marketCardHref(ref, filter)
	coins, ok := wholeNumber(r.PostFormValue("coins"), 64)
	if !ok || r.PostFormValue("coins") == "" {
		h.refuse(w, http.StatusBadRequest, "Give your offer in whole coins.", back)
		return
	}
	who := h.sellerLabel(ref, npc)
	if _, err := h.game.OfferTo(npc, ref.ID, ref.finish(), domain.Coins(coins)); err != nil {
		h.marketFailed(w, err, back)
		return
	}
	h.notices.put(noticeView{Text: fmt.Sprintf("Offered %s for %s to %s: it replies after its delay. Follow it in My trades.", coinsText(domain.Coins(coins)), h.describe(ref), who)})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// sellerLabel is who NPC npc is among those that hold the card, as the player sees it, for a notice.
func (h *handler) sellerLabel(ref cardRef, npc int) string {
	if sales, err := h.game.ForSale(); err == nil {
		for _, s := range sales {
			if s.Card.ID != ref.ID || s.Finish != ref.finish() {
				continue
			}
			for _, seller := range s.Sellers {
				if seller.NPC == npc {
					return sellerLabelOf(seller)
				}
			}
		}
	}
	return fmt.Sprintf("NPC %d", npc+1)
}

// marketTabs is the tabs of the market pages, with the answers waiting for the player.
func (h *handler) marketTabs(active string) marketTabsView {
	v, err := h.game.Market()
	if err != nil && !errors.Is(err, play.ErrNoMarket) {
		h.log.Error("market tabs", "error", err)
	}
	return newMarketTabsView(active, v.Waiting)
}
