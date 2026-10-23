package web_test

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const saveFailedNotice = "could not be saved"

// savedGame is a game on the Ravnica test set whose saves go to store.
func savedGame(t *testing.T, store play.Store) *play.Game {
	t.Helper()
	rav := domaintest.NewRavnicaCatalogue()
	stream := play.NewStream(testSeed)
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()},
		rand.New(stream),
	)
	game, err := play.NewGame([]domain.SetCatalogue{rav}, nil, opener, playtest.Rules(rav), playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := game.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	return game
}

func TestSaveFailure_isToldOnEveryPageAndTheOpeningStillGoesThrough(t *testing.T) {
	t.Parallel()
	store := &playtest.FlakyStore{}
	store.Fail(errors.New("disk full"))
	srv, c, _ := serve(t, savedGame(t, store), fakeImages{})

	resp := openPack(t, c, srv.URL, "RAV")
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /open = %d, want 303: a save that fails must not fail the opening", resp.StatusCode)
	}

	for _, path := range []string{"/", "/pack", "/collection"} {
		_, body := get(t, c, srv.URL+path)
		if !strings.Contains(body, saveFailedNotice) {
			t.Errorf("GET %s lacks the %q notice:\n%s", path, saveFailedNotice, body)
		}
		if strings.Contains(body, "disk full") {
			t.Errorf("GET %s shows the error of the disk to the player:\n%s", path, body)
		}
	}

	store.Fail(nil)
	openPack(t, c, srv.URL, "RAV")

	_, body := get(t, c, srv.URL+"/")
	if strings.Contains(body, saveFailedNotice) {
		t.Errorf("the notice stays after a save went through:\n%s", body)
	}
}
