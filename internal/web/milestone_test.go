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

	for _, want := range []string{"Milestone reached in S", "every common card", "5 energy", "locked"} {
		if !strings.Contains(first, want) {
			t.Errorf("the page after the opening lacks %q", want)
		}
	}
	if strings.Contains(second, "Milestone reached in S") {
		t.Error("the next page tells the milestones again")
	}
}

// filterRow is the entry of a filter panel whose label starts with label.
func filterRow(t *testing.T, body, panelLabel, label string) string {
	t.Helper()
	for _, row := range strings.Split(panel(body, panelLabel), "<li>")[1:] {
		if strings.Contains(row, ">"+label+" <strong>") {
			row, _, _ = strings.Cut(row, "</li>")
			return row
		}
	}
	t.Fatalf("the panel %q has no row %q:\n%s", panelLabel, label, body)
	return ""
}

func TestMilestones_aGoalEarnedIsACheckMarkInItsFilterRowAndNoGoalNotYetEarnedIs(t *testing.T) {
	t.Parallel()
	base, c := milestoneServer(t)

	_, before := get(t, c, base+"/collection")
	openPack(t, c, base, "S")
	_, after := get(t, c, base+"/collection")

	if strings.Contains(before, "milestone-mark") {
		t.Errorf("a goal not earned yet has a check mark:\n%s", before)
	}
	tests := []struct {
		panel, row string
		want       []string
	}{
		{"Filter by rarity", "Common", []string{"every common card", "5 energy"}},
		{"Filter by colour", "Colourless", []string{"every colourless card", "5 energy"}},
		{"Filter by rarity", "All rarities", []string{"the whole set", "unlocks sets"}},
	}
	for _, tt := range tests {
		row := filterRow(t, after, tt.panel, tt.row)
		mustContain(t, "the row "+tt.row, row, `class="milestone-mark"`, `role="img"`, "Milestone reached")
		mustContain(t, "the row "+tt.row, row, tt.want...)
	}
	mustNotContain(t, "the collection", after, `class="milestones"`, "Every common card")
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
