package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// milestoneServer serves a game on one set of two commons, whose booster holds both, that pays
// milestones: one booster completes the common rarity, the colourless group and the binder.
func milestoneServer(t *testing.T) (base string, c *http.Client) {
	t.Helper()
	set := playtest.TinySet("S", 2, 2)
	rules := playtest.Rules(set)
	rules.Milestones = &play.MilestoneRules{Rarity: map[domain.Rarity]domain.Percent{domain.RarityCommon: 50}, Colour: 50}
	game, err := play.NewGame([]domain.SetCatalogue{set}, nil, newOpener(), rules, playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	srv, client, _ := serve(t, game, fakeImages{})
	return srv.URL, client
}

func TestMilestones_aPageTellsTheMilestonesEarnedOnce(t *testing.T) {
	t.Parallel()
	base, c := milestoneServer(t)
	openPack(t, c, base, "S")

	_, first := get(t, c, base+"/collection")
	_, second := get(t, c, base+"/collection")

	for _, want := range []string{"Milestone reached", "every common card", "5 energy", "locked"} {
		if !strings.Contains(first, want) {
			t.Errorf("the page after the opening lacks %q", want)
		}
	}
	if strings.Contains(second, "Milestone reached") {
		t.Error("the next page tells the milestones again")
	}
}

func TestMilestones_theCollectionListsTheGoalsOfTheBinder(t *testing.T) {
	t.Parallel()
	base, c := milestoneServer(t)

	_, before := get(t, c, base+"/collection")
	openPack(t, c, base, "S")
	_, after := get(t, c, base+"/collection")

	for _, want := range []string{"Milestones", "Every common card", "Every colourless card", "The whole set"} {
		if !strings.Contains(before, want) {
			t.Errorf("the collection lacks the goal %q", want)
		}
	}
	if strings.Count(after, `class="milestone done"`) != 3 {
		t.Errorf("the collection marks %d goals done, want 3", strings.Count(after, `class="milestone done"`))
	}
}

func TestMilestones_sellingTheLockedCopyIsRefusedWithTheReason(t *testing.T) {
	t.Parallel()
	set := counterSet()
	rules := playtest.Rules(set)
	rules.Milestones = &play.MilestoneRules{Colour: 50}
	game, err := play.NewGame([]domain.SetCatalogue{set}, nil, newOpener(), rules, playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := game.OpenCounter(counterRules(), play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	srv, c, _ := serve(t, game, fakeImages{})
	if _, err := game.Open("S"); err != nil {
		t.Fatalf("Open: %v", err)
	}

	form := sellForm(cheapCard, "1")
	form.Set("confirm", "yes")
	resp, err := c.PostForm(srv.URL+"/sell", form)
	if err != nil {
		t.Fatalf("POST /sell: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusConflict || !strings.Contains(body, "locks this copy") {
		t.Errorf("selling the only copy = %d, want 409 saying that a milestone locks it:\n%s", resp.StatusCode, body)
	}
}

func TestMilestones_aLockedCardCannotBeSoldFromTheCollection(t *testing.T) {
	t.Parallel()
	base, c := milestoneServer(t)
	openPack(t, c, base, "S")

	_, body := get(t, c, base+"/collection")

	if !strings.Contains(body, "locked by a milestone") {
		t.Error("the slots do not say that a copy is locked")
	}
}
