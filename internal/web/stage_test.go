package web_test

import (
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// stagedServer serves a game on three sets cut into one stage each: A, whose one card a booster
// gives, then B and C. A's first booster unlocks B.
func stagedServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	sets := []domain.SetCatalogue{playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1), playtest.TinySet("C", 30, 1)}
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()},
		rand.New(rand.NewPCG(testSeed, testSeed)),
	)
	game, err := play.NewGame(sets, nil, opener, playtest.Rules(sets...), playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := game.Stage(play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 2, StartSets: 1}, play.NewStream(1)); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	srv, c, _ := serve(t, game, fakeImages{})
	return srv, c
}

func openButton(code string) string { return `name="set" value="` + code + `"` }

func TestHome_groupsTheSetsByStageAndGreysTheLockedOnesWithoutAButton(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	_, body := get(t, c, srv.URL+"/")

	for _, want := range []string{"Stage 1", "Stage 2", "Stage 3", openButton("A"), "Set B", "Set C"} {
		if !strings.Contains(body, want) {
			t.Errorf("home page lacks %q:\n%s", want, body)
		}
	}
	for _, code := range []string{"B", "C"} {
		if strings.Contains(body, openButton(code)) {
			t.Errorf("home page has an open button for the locked set %s", code)
		}
	}
	if got := len(regexp.MustCompile(`class="locked"`).FindAllString(body, -1)); got != 2 {
		t.Errorf("home page marks %d sets locked, want 2:\n%s", got, body)
	}
}

func TestHome_aGameWithoutStagesShowsNoStage(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1))

	_, body := get(t, c, srv.URL+"/")

	if strings.Contains(body, "Stage") || strings.Contains(body, `class="locked"`) {
		t.Errorf("a game without stages shows a stage or a lock:\n%s", body)
	}
	if !strings.Contains(body, openButton("B")) {
		t.Errorf("home page lacks the open button of B:\n%s", body)
	}
}

func TestOpen_aLockedSetIsAConflictThatSaysItIsLocked(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	resp := openPack(t, c, srv.URL, "C")
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusConflict || !strings.Contains(body, "locked") {
		t.Errorf("POST /open of a locked set = %d:\n%s\nwant 409 saying it is locked", resp.StatusCode, body)
	}
}

func TestPack_saysWhichSetsTheBoosterUnlockedAndHomeOpensThem(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	openPack(t, c, srv.URL, "A")
	_, pack := get(t, c, srv.URL+"/pack")
	_, home := get(t, c, srv.URL+"/")

	if !strings.Contains(pack, "Unlocked") || !strings.Contains(pack, "Set B") {
		t.Errorf("pack page does not say B was unlocked:\n%s", pack)
	}
	if !strings.Contains(home, openButton("B")) {
		t.Errorf("home page lacks the open button of the unlocked B:\n%s", home)
	}
	openPack(t, c, srv.URL, "A")
	if _, again := get(t, c, srv.URL+"/pack"); strings.Contains(again, "Unlocked") {
		t.Errorf("a booster that unlocked nothing says it did:\n%s", again)
	}
}

func TestCollection_marksTheLockedSetsAndStillShowsTheirBinder(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	resp, body := get(t, c, srv.URL+"/collection?set=C")

	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "<h1>Set C") {
		t.Fatalf("GET the binder of a locked set = %d, want 200 and its binder:\n%s", resp.StatusCode, body)
	}
	if got := strings.Count(body, `class="locked"`); got != 2 {
		t.Errorf("the switcher marks %d binders locked, want 2:\n%s", got, body)
	}
}
