package web_test

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
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
	if got := panel(all, "Filter by rarity"); strings.Count(got, `aria-current="true"`) != 1 || !strings.Contains(got, `aria-current="true">All rarities`) {
		t.Errorf("with no filter, only All rarities must be marked as the one in force:\n%s", got)
	}
	if got := panel(rare, "Filter by rarity"); strings.Count(got, `aria-current="true"`) != 1 || !strings.Contains(got, `aria-current="true">Rare`) {
		t.Errorf("with rarity=rare, only Rare must be marked:\n%s", got)
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

// panel is the part of body that is the list labelled label, up to the end of its nav.
func panel(body, label string) string {
	_, rest, _ := strings.Cut(body, `aria-label="`+label+`"`)
	got, _, _ := strings.Cut(rest, "</nav>")
	return got
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

	colours := panel(empty, "Filter by colour")
	for _, want := range []string{"White <strong>0 / 2</strong>", "Green <strong>0 / 1</strong>", "Gold <strong>0 / 1</strong>", "Colourless <strong>0 / 1</strong>", "Land <strong>0 / 1</strong>"} {
		if !strings.Contains(colours, want) {
			t.Errorf("the colour list lacks %q:\n%s", want, colours)
		}
	}
	for _, absent := range []string{"Blue", "Black", "Red"} {
		if strings.Contains(colours, absent+" <strong>") {
			t.Errorf("the colour list shows %s, which no card of the set is", absent)
		}
	}
	if got := panel(full, "Filter by colour"); !strings.Contains(got, "White <strong>2 / 2</strong>") {
		t.Errorf("after a booster of the whole binder White is not 2 / 2:\n%s", got)
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

// mixedSet is coloursSet with its first white card an uncommon, so that rarity and colour cut the
// binder in different ways: white is one common and one uncommon, and the rest are commons.
func mixedSet() domain.SetCatalogue {
	cat := coloursSet()
	card := cat.Cards["M-00"]
	card.Rarity = domain.RarityUncommon
	cat.Cards["M-00"] = card
	return cat
}

func TestCollection_filtersByColourAndKeepsTheWholeBindersProgress(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, coloursSet())

	tests := []struct {
		colour string
		want   int
	}{
		{"W", 2},
		{"G", 1},
		{"gold", 1},
		{"colourless", 1},
		{"land", 1},
	}
	for _, tc := range tests {
		resp, body := get(t, c, srv.URL+"/collection?colour="+tc.colour)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("colour=%s: status %d, want 200", tc.colour, resp.StatusCode)
		}
		if got := slotsShown(body); got != tc.want {
			t.Errorf("colour=%s shows %d slots, want %d", tc.colour, got, tc.want)
		}
		if !strings.Contains(body, `max="6"`) {
			t.Errorf("colour=%s: the progress bar must still be the whole binder's (max 6):\n%s", tc.colour, body)
		}
	}
}

func TestCollection_aColourTheBinderDoesNotHaveIsABadRequest(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, coloursSet())

	for _, colour := range []string{"U", "purple", "<script>"} {
		resp, body := get(t, c, srv.URL+"/collection?colour="+url.QueryEscape(colour))

		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(strings.ToLower(body), "unknown colour") {
			t.Errorf("colour=%s = %d, want 400 saying unknown colour:\n%s", colour, resp.StatusCode, body)
		}
		if strings.Contains(body, "<script>") {
			t.Errorf("the colour was echoed unescaped:\n%s", body)
		}
	}
}

func TestCollection_aRarityAndAColourTogetherKeepTheCardsOfBoth(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, mixedSet())

	tests := []struct {
		query string
		want  int
	}{
		{"rarity=common&colour=W", 1},
		{"rarity=uncommon&colour=W", 1},
		{"rarity=uncommon&colour=G", 0},
		{"rarity=common&colour=gold", 1},
	}
	for _, tc := range tests {
		_, body := get(t, c, srv.URL+"/collection?"+tc.query)

		if got := slotsShown(body); got != tc.want {
			t.Errorf("%s shows %d slots, want %d", tc.query, got, tc.want)
		}
	}
}

func TestCollection_theColourListIsAFilterThatKeepsTheRarityInForce(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, mixedSet())

	_, all := get(t, c, srv.URL+"/collection")
	_, both := get(t, c, srv.URL+"/collection?rarity=common&colour=W")

	colours := panel(both, "Filter by colour")
	for _, want := range []string{
		`href="/collection?rarity=common&amp;set=M"`, // White is in force, so its link clears it, and All colours does too
		`href="/collection?colour=G&amp;rarity=common&amp;set=M"`,
		`aria-current="true">White`,
	} {
		if !strings.Contains(colours, want) {
			t.Errorf("the colour list lacks %q:\n%s", want, colours)
		}
	}
	if got := strings.Count(colours, `href="/collection?rarity=common&amp;set=M"`); got != 2 {
		t.Errorf("%d links clear the colour, want 2 (All colours and White):\n%s", got, colours)
	}
	rarities := panel(both, "Filter by rarity")
	for _, want := range []string{
		`href="/collection?colour=W&amp;set=M"`, // Common is in force, so its link clears it, and All rarities does too
		`href="/collection?colour=W&amp;rarity=uncommon&amp;set=M"`,
		`aria-current="true">Common`,
	} {
		if !strings.Contains(rarities, want) {
			t.Errorf("the rarity list lacks %q:\n%s", want, rarities)
		}
	}
	if got := strings.Count(rarities, `href="/collection?colour=W&amp;set=M"`); got != 2 {
		t.Errorf("%d links clear the rarity, want 2 (All rarities and Common):\n%s", got, rarities)
	}
	if got := panel(all, "Filter by colour"); !strings.Contains(got, `aria-current="true">All colours`) || strings.Count(got, `aria-current="true"`) != 1 {
		t.Errorf("with no colour in force, only All colours must be marked:\n%s", got)
	}
}

// binderList is the list of binders of a collection page, from its nav to the end of it.
func binderList(body string) string { return panel(body, "Binders") }

func TestCollection_theBinderListShowsTheUnlockedSetsAndFoldsTheLockedOnesAway(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	_, body := get(t, c, srv.URL+"/collection?set=A")

	list := binderList(body)
	folded := strings.Index(list, `<details class="locked-binders">`)
	if folded < 0 {
		t.Fatalf("the locked sets are not in a section that starts closed:\n%s", list)
	}
	if !strings.Contains(list, "Locked sets (2)") {
		t.Errorf("the closed section does not say how many sets it holds:\n%s", list)
	}
	if a := strings.Index(list, `href="/collection?set=A"`); a < 0 || a > folded {
		t.Errorf("the unlocked set A is not listed above the locked ones:\n%s", list)
	}
	for _, code := range []string{"B", "C"} {
		if at := strings.Index(list, `href="/collection?set=`+code+`"`); at < folded {
			t.Errorf("the locked set %s is listed outside the closed section:\n%s", code, list)
		}
	}
}

func TestCollection_theClosedSectionOpensWhenTheBinderInViewIsLocked(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	_, body := get(t, c, srv.URL+"/collection?set=C")

	if list := binderList(body); !strings.Contains(list, `<details class="locked-binders" open>`) {
		t.Errorf("the binder in view is locked and its set is hidden in a closed section:\n%s", list)
	}
}

func TestCollection_aGameWithNoLockedSetHasNoClosedSection(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1))

	_, body := get(t, c, srv.URL+"/collection")

	if list := binderList(body); strings.Contains(list, "locked-binders") {
		t.Errorf("a section of locked sets is shown when no set is locked:\n%s", list)
	}
}

func TestCollection_everyUnlockedSetFillsItsEntryUpToWhatIsHeldAndALockedOneDoesNot(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)
	openPack(t, c, srv.URL, "A")

	_, body := get(t, c, srv.URL+"/collection?set=A")

	list := binderList(body)
	open, locked, _ := strings.Cut(list, `<details class="locked-binders"`)
	if !strings.Contains(open, `<progress value="1" max="1" aria-hidden="true"></progress>`) || !strings.Contains(open, "1 / 1") {
		t.Errorf("the entry of A, which is complete, is not filled:\n%s", open)
	}
	if strings.Contains(locked, "<progress") {
		t.Errorf("an entry of a locked set is filled:\n%s", locked)
	}
}

func TestCollection_theBinderListCanBeSearchedAndNamesTheSetInView(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	_, body := get(t, c, srv.URL+"/collection?set=A")

	list := binderList(body)
	for _, want := range []string{`<input type="search"`, `aria-label="Search sets"`, `<details class="set-picker" open>`, "<summary>"} {
		if !strings.Contains(list, want) {
			t.Errorf("the binder list lacks %q:\n%s", want, list)
		}
	}
	if _, summary, _ := strings.Cut(list, "<summary>"); !strings.Contains(summary[:strings.Index(summary, "</summary>")], "Set A") {
		t.Errorf("the summary does not name the set in view:\n%s", list)
	}
}

func TestCollection_anEntryOfTheBinderListKeepsTheFullSetNameInItsTitleForWhenTheLineCutsIt(t *testing.T) {
	t.Parallel()
	srv, c := stagedServer(t)

	_, body := get(t, c, srv.URL+"/collection?set=A")

	if want := `<a href="/collection?set=A" title="Set A"`; !strings.Contains(binderList(body), want) {
		t.Errorf("the entry of Set A has no title with its name, want %q in:\n%s", want, binderList(body))
	}
}

func TestCollection_showsThePercentageOfProgressNextToEveryCountOfTheSet(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, playtest.TinySet("P", 4, 1))

	_, before := get(t, c, srv.URL+"/collection")
	openPack(t, c, srv.URL, "P")
	_, after := get(t, c, srv.URL+"/collection")

	for _, want := range []string{`<span class="pct">0%</span>`} {
		if got := strings.Count(before, want); got < 4 {
			t.Errorf("before a booster, %q shows %d times, want it in the header, the list and the rarity and colour panels:\n%s", want, got, before)
		}
	}
	if strings.Contains(before, `class="pct">25%`) {
		t.Errorf("a binder with nothing in it shows 25%%:\n%s", before)
	}
	list, header := binderList(after), after[strings.Index(after, `class="set-header"`):]
	for what, body := range map[string]string{"the list of sets": list, "the header": header, "the rarity panel": panel(after, "Filter by rarity")} {
		if !strings.Contains(body, `class="pct">25%`) {
			t.Errorf("after a booster of one card out of four, %s lacks 25%%:\n%s", what, body)
		}
	}
}

func TestCollection_theShowListIsPartOfTheSetHeader(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, playtest.TinySet("P", 4, 1))

	_, body := get(t, c, srv.URL+"/collection")

	_, fromHeader, found := strings.Cut(body, `class="set-header"`)
	if !found {
		t.Fatalf("the page has no set header:\n%s", body)
	}
	header, _, _ := strings.Cut(fromHeader, "</section>")
	mustContain(t, "the set header", header, `aria-label="`+showPanelLabel+`"`, "All cards", "Duplicates")
	// The title row, then the show list, the rarity panel and the colour panel side by side.
	shows, title, rarity := strings.Index(header, `aria-label="`+showPanelLabel+`"`), strings.Index(header, "<h1>"), strings.Index(header, `aria-label="Filter by rarity"`)
	if title > shows || shows > rarity {
		t.Errorf("the header holds the title at %d, the show list at %d and the rarity panel at %d, want them in that order", title, shows, rarity)
	}
	if playsets := strings.Index(header, `class="playsets"`); playsets > shows {
		t.Errorf("the playsets come after the show list: they belong to the title row")
	}
}

func TestCollection_anEmptySlotShowsThePictureOfItsCardForTheStylesheetToGrey(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	ids := withScryfallIDs(rav)
	srv, c := newServer(t, rav)

	_, body := get(t, c, srv.URL+"/collection")

	empty := regexp.MustCompile(`(?s)<div class="slot empty">.*?</div>`).FindAllString(body, -1)
	if len(empty) != len(rav.Cards) {
		t.Fatalf("%d empty slots, want %d", len(empty), len(rav.Cards))
	}
	for _, slot := range empty {
		mustContain(t, "an empty slot", slot, `<img src="/card-image/`, `class="art"`, `alt=""`, `loading="lazy"`)
	}
	for _, pic := range ids {
		mustContain(t, "the page", body, `/card-image/`+pic)
	}
}
