package web

import (
	"errors"
	"net/http"

	"github.com/foutoucour/crackapack/internal/play"
)

// hunt starts hunting the card of a slot, and comes back to the slot.
func (h *handler) hunt(w http.ResponseWriter, r *http.Request) {
	ref, ok := cardRefOf(r)
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown card: hunt from a slot of a binder.", binderHref(ref.Set, ref.Filter, ""))
		return
	}
	if err := h.game.Hunt(ref.ID, ref.finish()); err != nil {
		h.huntFailed(w, err, ref)
		return
	}
	h.notices.put(noticeView{Text: "Hunting " + h.describe(ref) + "."})
	http.Redirect(w, r, ref.back(), http.StatusSeeOther)
}

// unhunt stops hunting a card, and comes back to the slot or to the hunted cards, where the form was.
func (h *handler) unhunt(w http.ResponseWriter, r *http.Request) {
	ref, ok := cardRefOf(r)
	if !ok {
		h.refuse(w, http.StatusBadRequest, "Unknown card: stop hunting from a slot of a binder or from the hunted cards.", binderHref(ref.Set, ref.Filter, ""))
		return
	}
	if err := h.game.Unhunt(ref.ID, ref.finish()); err != nil {
		h.huntFailed(w, err, ref)
		return
	}
	h.notices.put(noticeView{Text: "Stopped hunting " + h.describe(ref) + "."})
	if r.PostFormValue("from") == fromHunted {
		http.Redirect(w, r, huntedPath, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, ref.back(), http.StatusSeeOther)
}

// hunted shows the cards the player hunts. Showing the copies that came to the Counter is telling
// the player of them, so they are no longer new once the page is built.
func (h *handler) hunted(w http.ResponseWriter, _ *http.Request) {
	hunted, err := h.game.Hunted()
	if err != nil {
		h.huntFailed(w, err, cardRef{})
		return
	}
	view := newHuntedPageView(hunted)
	view.Notice = h.notices.take()
	if err := h.game.SeeHunted(); err != nil {
		h.huntFailed(w, err, cardRef{})
		return
	}
	h.render(w, http.StatusOK, "hunted", view)
}

// huntedNav is the header's link to the hunted cards, nil in a game without a Counter.
func (h *handler) huntedNav() *huntedNavView {
	hunted, err := h.game.Hunted()
	if err != nil {
		if !errors.Is(err, play.ErrNoCounter) {
			h.log.Error("hunted header", "error", err)
		}
		return nil
	}
	return newHuntedNavView(hunted)
}

// huntFailed answers a refusal of a hunt, with a way back to the slot the player came from. The
// refusals it shares with the Counter are told by counterFailed.
func (h *handler) huntFailed(w http.ResponseWriter, err error, ref cardRef) {
	switch {
	case errors.Is(err, play.ErrHeld):
		h.refuse(w, http.StatusConflict, "You already hold this card: there is nothing to hunt.", ref.back())
	case errors.Is(err, play.ErrTooManyHunted):
		h.refuse(w, http.StatusConflict, "You hunt as many cards as the list holds: stop hunting one first.", ref.back())
	default:
		h.counterFailed(w, err, ref)
	}
}
