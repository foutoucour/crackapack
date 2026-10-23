package web_test

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// newServerWithPool serves a game on set whose boosters draw cards from pool, as Time Spiral's do.
func newServerWithPool(t *testing.T, set, pool domain.SetCatalogue) (*httptest.Server, *http.Client) {
	t.Helper()
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{
			domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
		},
		rand.New(rand.NewPCG(testSeed, testSeed)),
	)
	game, err := play.NewGame([]domain.SetCatalogue{set}, []domain.SetCatalogue{pool}, opener, playtest.Rules(set), playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	srv, client, _ := serve(t, game, fakeImages{})
	return srv, client
}

// slotsShown counts the slots of the binder, owned and empty, in body.
func slotsShown(body string) int {
	return strings.Count(body, "data-card ") + strings.Count(body, `class="slot empty"`)
}

func TestCollection_filtersByRarityAndKeepsTheWholeBindersProgress(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	tests := []struct {
		rarity string
		want   int
	}{
		{"common", 11},
		{"uncommon", 4},
		{"rare", 1},
	}
	for _, tc := range tests {
		resp, body := get(t, c, srv.URL+"/collection?rarity="+tc.rarity)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("rarity=%s: status %d, want 200", tc.rarity, resp.StatusCode)
		}
		if got := slotsShown(body); got != tc.want {
			t.Errorf("rarity=%s shows %d slots, want %d", tc.rarity, got, tc.want)
		}
		if !strings.Contains(body, `max="16"`) {
			t.Errorf("rarity=%s: the progress bar must still be the whole binder's (max 16):\n%s", tc.rarity, body)
		}
	}
}

func TestCollection_theRarityListIsTheFilterAndMarksTheOneInForce(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	srv, c := newServerWithPool(t, tsp, tsb)

	_, all := get(t, c, srv.URL+"/collection?set="+tsp.Code)
	_, rare := get(t, c, srv.URL+"/collection?set="+tsp.Code+"&rarity=rare")

	for _, want := range []string{`href="/collection?set=TSP"`, `href="/collection?rarity=rare&amp;set=TSP"`, `href="/collection?rarity=common&amp;set=TSP"`} {
		if !strings.Contains(rare, want) {
			t.Errorf("the filter lacks a link %q:\n%s", want, rare)
		}
	}
	if strings.Count(all, `aria-current="true"`) != 1 || !strings.Contains(all, `aria-current="true">All`) {
		t.Errorf("with no filter, only All must be marked as the one in force:\n%s", all)
	}
	if strings.Count(rare, `aria-current="true"`) != 1 || !strings.Contains(rare, `aria-current="true">Rare`) {
		t.Errorf("with rarity=rare, only Rare must be marked:\n%s", rare)
	}
}

func TestCollection_aRarityTheBinderDoesNotHaveIsABadRequest(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	for _, rarity := range []string{"mythic", "legendary", "<script>"} {
		resp, body := get(t, c, srv.URL+"/collection?rarity="+url.QueryEscape(rarity))

		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(strings.ToLower(body), "unknown rarity") {
			t.Errorf("rarity=%s = %d, want 400 saying unknown rarity:\n%s", rarity, resp.StatusCode, body)
		}
		if strings.Contains(body, "<script>") {
			t.Errorf("the rarity was echoed unescaped:\n%s", body)
		}
	}
}

func TestCollection_theTimeshiftedCardsHaveASectionAndCountInTheBinder(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	srv, c := newServerWithPool(t, tsp, tsb)

	_, body := get(t, c, srv.URL+"/collection")

	nonBasic := 0
	for _, card := range tsp.Cards {
		if !card.IsBasicLand {
			nonBasic++
		}
	}
	if want := fmt.Sprintf(`max="%d"`, nonBasic+len(tsb.Cards)); !strings.Contains(body, want) {
		t.Errorf("progress bar lacks %s: the set's cards plus the timeshifted ones:\n%s", want, body)
	}
	_, section, found := strings.Cut(body, "<h2>"+tsb.Name)
	if !found {
		t.Fatalf("no section named %q:\n%s", tsb.Name, body)
	}
	if got := strings.Count(section, `class="slot empty"`); got != len(tsb.Cards) {
		t.Errorf("the timeshifted section has %d empty slots, want %d", got, len(tsb.Cards))
	}
	if !strings.Contains(body, "Special") {
		t.Errorf("the special rarity is not in the rarity list:\n%s", body)
	}
}

func TestCollection_filteringOnSpecialShowsOnlyTheTimeshiftedSection(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	srv, c := newServerWithPool(t, tsp, tsb)

	_, body := get(t, c, srv.URL+"/collection?rarity=special")

	if got := slotsShown(body); got != len(tsb.Cards) {
		t.Errorf("shows %d slots, want the %d timeshifted ones", got, len(tsb.Cards))
	}
	if !strings.Contains(body, "<h2>"+tsb.Name) {
		t.Errorf("the timeshifted section is missing:\n%s", body)
	}
}

func TestCollection_filteringOnCommonHidesTheTimeshiftedSection(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	srv, c := newServerWithPool(t, tsp, tsb)

	_, body := get(t, c, srv.URL+"/collection?rarity=common")

	if strings.Contains(body, "<h2>"+tsb.Name) {
		t.Errorf("a section with no slot left is still shown:\n%s", body)
	}
}

// coloursSet is six commons, one a gold card, one colourless, one a land and two white.
func coloursSet() domain.SetCatalogue {
	cat := playtest.TinySet("M", 6, 6)
	for id, colours := range map[domain.CardID][]string{"M-00": {"W"}, "M-01": {"U", "B"}, "M-04": {"W"}, "M-05": {"G"}} {
		card := cat.Cards[id]
		card.Colors = colours
		cat.Cards[id] = card
	}
	land := cat.Cards["M-03"]
	land.IsLand = true
	cat.Cards["M-03"] = land
	return cat
}

func TestCollection_showsTheProgressByColourWithTextLabelsAndLeavesOutTheEmptyGroups(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, coloursSet())

	_, empty := get(t, c, srv.URL+"/collection")
	openPack(t, c, srv.URL, "M")
	_, full := get(t, c, srv.URL+"/collection")

	for _, want := range []string{
		`aria-label="Progress by colour"`,
		`<li>White <strong>0 / 2</strong></li>`, `<li>Green <strong>0 / 1</strong></li>`, `<li>Gold <strong>0 / 1</strong></li>`,
		`<li>Colourless <strong>0 / 1</strong></li>`, `<li>Land <strong>0 / 1</strong></li>`,
	} {
		if !strings.Contains(empty, want) {
			t.Errorf("the binder lacks %q:\n%s", want, empty)
		}
	}
	for _, absent := range []string{"Blue", "Black", "Red"} {
		if strings.Contains(empty, "<li>"+absent+" ") {
			t.Errorf("the binder lists %s, which no card of the set is", absent)
		}
	}
	if !strings.Contains(full, `<li>White <strong>2 / 2</strong></li>`) {
		t.Errorf("after a booster of the whole binder White is not 2 / 2:\n%s", full)
	}
}

func TestCollection_showsTheOverviewAtTheTopOfThePage(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	_, before := get(t, c, srv.URL+"/collection")
	openPack(t, c, srv.URL, "A")
	_, after := get(t, c, srv.URL+"/collection")

	for body, want := range map[string][]string{
		before: {`<dt>Cards held</dt><dd>0 / 1</dd>`, `<dt>Binders complete</dt><dd>0</dd>`, `<dt>Sets unlocked</dt><dd>1 / 3</dd>`},
		after:  {`<dt>Cards held</dt><dd>1 / 3</dd>`, `<dt>Binders complete</dt><dd>1</dd>`, `<dt>Sets unlocked</dt><dd>2 / 3</dd>`},
	} {
		for _, w := range want {
			if !strings.Contains(body, w) {
				t.Errorf("the overview lacks %q:\n%s", w, body)
			}
		}
		if overview, nav := strings.Index(body, `aria-label="Collection overview"`), strings.Index(body, `<nav class="binders"`); overview < 0 || overview > nav {
			t.Errorf("the overview is not above the binders:\n%s", body)
		}
	}
}

func TestCollection_marksOnlyTheBinderThatPassedTheUnlockThreshold(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)
	openPack(t, c, srv.URL, "A")

	_, passed := get(t, c, srv.URL+"/collection?set=A")
	_, other := get(t, c, srv.URL+"/collection?set=B")

	if !strings.Contains(passed, "Unlock threshold reached") {
		t.Errorf("the binder of A, which unlocked B, is not marked:\n%s", passed)
	}
	if strings.Contains(other, "Unlock threshold reached") {
		t.Errorf("the binder of B is marked, and it has not reached the threshold:\n%s", other)
	}
}
