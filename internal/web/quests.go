package web

import (
	"errors"
	"net/http"

	"github.com/foutoucour/crackapack/internal/play"
)

// quests shows the daily and weekly quests with how far the player is. Showing them counts what the
// player did, since the game does that when it is asked.
func (h *handler) quests(w http.ResponseWriter, _ *http.Request) {
	board, err := h.game.Quests()
	if errors.Is(err, play.ErrNoQuests) {
		h.fail(w, http.StatusNotFound, "This game has no quests.")
		return
	}
	if err != nil {
		h.playFailed(w, err)
		return
	}
	h.render(w, http.StatusOK, "quests", newQuestsPageView(board))
}

// questsNav is the header's link to the quests, nil in a game without them.
func (h *handler) questsNav() *questsNavView {
	if _, err := h.game.Quests(); err != nil {
		if !errors.Is(err, play.ErrNoQuests) {
			h.log.Error("quests header", "error", err)
		}
		return nil
	}
	return &questsNavView{}
}
