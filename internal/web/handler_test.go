package web_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/images"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
	"github.com/foutoucour/crackapack/internal/web"
)

const testSeed = 11

// fakeImages is an images.Source over a fixed set of pictures.
type fakeImages struct {
	pictures map[string]images.Image
	err      error
}

func (f fakeImages) Image(_ context.Context, id string) (images.Image, error) {
	if f.err != nil {
		return images.Image{}, f.err
	}
	if img, ok := f.pictures[id]; ok {
		return img, nil
	}
	return images.Image{}, images.ErrNotFound
}

// withScryfallIDs gives every card of cat a valid Scryfall ID, and returns the ID of each card.
func withScryfallIDs(cat domain.SetCatalogue) map[domain.CardID]string {
	ids := make(map[domain.CardID]string, len(cat.Cards))
	n := 0
	for id, card := range cat.Cards {
		n++
		card.ScryfallID = fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
		cat.Cards[id] = card
		ids[id] = card.ScryfallID
	}
	return ids
}

// newServer serves a fresh game on sets. Its client does not follow redirects, so a test can
// assert on the redirect itself.
func newServer(t *testing.T, sets ...domain.SetCatalogue) (*httptest.Server, *http.Client) {
	t.Helper()
	srv, c, _ := newLoggedServer(t, fakeImages{}, sets...)
	return srv, c
}

// newLoggedServer is newServer plus the buffer the handler writes its log to.
func newLoggedServer(t *testing.T, pics images.Source, sets ...domain.SetCatalogue) (*httptest.Server, *http.Client, *bytes.Buffer) {
	t.Helper()
	return serve(t, newGame(t, sets...), pics)
}

// newGame is a fresh game on sets.
func newGame(t *testing.T, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{
			domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
		},
		rand.New(rand.NewPCG(testSeed, testSeed)),
	)
	game, err := play.NewGame(sets, nil, opener, playtest.Rules(sets...), playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	return game
}

// serve serves game, with a client that does not follow redirects and the buffer the handler logs to.
func serve(t *testing.T, game *play.Game, pics images.Source) (*httptest.Server, *http.Client, *bytes.Buffer) {
	t.Helper()
	return serveWithIcons(t, game, pics, fstest.MapFS{})
}

// serveWithIcons is serve over a catalogue directory that holds icons.
func serveWithIcons(t *testing.T, game *play.Game, pics images.Source, icons fs.FS) (*httptest.Server, *http.Client, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	srv := httptest.NewServer(web.NewHandler(game, pics, icons, slog.New(slog.NewTextHandler(&logs, nil))))
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return srv, client, &logs
}

func get(t *testing.T, c *http.Client, url string) (*http.Response, string) {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp, readBody(t, resp)
}

func openPack(t *testing.T, c *http.Client, base, set string) *http.Response {
	t.Helper()
	resp, err := c.PostForm(base+"/open", url.Values{"set": {set}})
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestHome_listsEachSetWithItsPackCountAndAnOpenButton(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	srv, c := newServer(t, rav, tsp)

	resp, body := get(t, c, srv.URL+"/")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	for _, want := range []string{rav.Name, tsp.Name, `name="set" value="` + rav.Code + `"`, "5 boosters"} {
		if !strings.Contains(body, want) {
			t.Errorf("home page lacks %q:\n%s", want, body)
		}
	}
}

func TestOpen_redirectsToThePackPageWhichShowsTheCards(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)

	resp := openPack(t, c, srv.URL, rav.Code)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/pack" {
		t.Fatalf("POST /open = %d to %q, want 303 to /pack", resp.StatusCode, resp.Header.Get("Location"))
	}

	page, body := get(t, c, srv.URL+"/pack")
	if page.StatusCode != http.StatusOK {
		t.Fatalf("GET /pack status = %d, want 200", page.StatusCode)
	}
	for _, want := range []string{"Ravnica Common", "Ravnica Uncommon", "Ravnica Rare"} {
		if !strings.Contains(body, want) {
			t.Errorf("pack page lacks a %q card:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "New") {
		t.Errorf("the first booster must flag new cards:\n%s", body)
	}
}

func TestPack_refreshingDoesNotSpendAnotherBooster(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	get(t, c, srv.URL+"/pack")
	get(t, c, srv.URL+"/pack")

	_, home := get(t, c, srv.URL+"/")
	if !strings.Contains(home, "9 boosters") {
		t.Errorf("want 9 boosters left after one opening and two refreshes:\n%s", home)
	}
}

func TestPack_withNothingOpenedYetRedirectsHome(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	resp, _ := get(t, c, srv.URL+"/pack")

	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("GET /pack = %d to %q, want 303 to /", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestOpen_failuresUseTheirStatusAndExplainThemselves(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	for range play.WelcomeBoosters {
		openPack(t, c, srv.URL, rav.Code)
	}

	tests := []struct {
		name       string
		form       url.Values
		wantStatus int
		wantText   string
	}{
		{"unknown set", url.Values{"set": {"NOPE"}}, http.StatusBadRequest, "unknown set"},
		{"missing set", url.Values{}, http.StatusBadRequest, "unknown set"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := c.PostForm(srv.URL+"/open", tc.form)
			if err != nil {
				t.Fatalf("POST /open: %v", err)
			}
			body := readBody(t, resp)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if !strings.Contains(strings.ToLower(body), tc.wantText) {
				t.Errorf("body lacks %q:\n%s", tc.wantText, body)
			}
		})
	}
}

func TestOpen_rejectsAnotherSitePostingTheForm(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/open", strings.NewReader(url.Values{"set": {rav.Code}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	_ = readBody(t, resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
	_, home := get(t, c, srv.URL+"/")
	if !strings.Contains(home, "10 boosters") {
		t.Errorf("a refused request must not spend a booster:\n%s", home)
	}
}

func TestPack_escapesCardNames(t *testing.T) {
	t.Parallel()
	const hostile = `<script>alert("x")</script>`
	rav := domaintest.NewRavnicaCatalogue()
	for id, card := range rav.Cards {
		card.Name = hostile
		rav.Cards[id] = card
	}
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/pack")

	if strings.Contains(body, hostile) {
		t.Errorf("card name rendered unescaped:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("escaped card name not found:\n%s", body)
	}
}

func TestResponses_carrySecurityHeaders(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	resp, _ := get(t, c, srv.URL+"/")

	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
	}
	for header, wantValue := range want {
		if got := resp.Header.Get(header); !strings.Contains(got, wantValue) {
			t.Errorf("%s = %q, want it to contain %q", header, got, wantValue)
		}
	}
	// The server speaks plain HTTP on localhost: HSTS there would pin the browser's localhost to HTTPS.
	if got := resp.Header.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q, want it absent on a plain-HTTP localhost server", got)
	}
}

// The card back is drawn in-house, see docs/core-features.md#legal-and-data: it must be a plain
// SVG, with nothing that runs and nothing fetched from elsewhere, since the pages' policy only
// allows this origin.
func TestCardBack_isAnInHouseSVGWithNothingThatRunsOrLoads(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	resp, body := get(t, c, srv.URL+"/static/card-back.svg")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/card-back.svg = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "image/svg+xml") {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
	var doc struct {
		XMLName xml.Name
		ViewBox string `xml:"viewBox,attr"`
	}
	if err := xml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("the card back is not well-formed XML: %v", err)
	}
	if doc.XMLName.Local != "svg" || doc.ViewBox != "0 0 488 680" {
		t.Errorf("root = <%s viewBox=%q>, want <svg viewBox=\"0 0 488 680\"> so it fits a card", doc.XMLName.Local, doc.ViewBox)
	}
	for _, forbidden := range []string{"<script", "<image", "<foreignObject", "href=", "http://www.w3.org/1999/xlink", "url(http"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the card back contains %q", forbidden)
		}
	}
}

// The stylesheet layers the downloaded back over the in-house one: a layer that fails to load
// paints nothing, so the SVG shows through when the picture is not there (offline, first launch).
func TestStylesheet_paintsTheDownloadedCardBackOverTheInHouseOne(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	_, css := get(t, c, srv.URL+"/static/style.css")

	downloaded := strings.Index(css, `url("/card-image/`+images.CardBackID+`")`)
	inHouse := strings.Index(css, `url("/static/card-back.svg")`)
	if downloaded < 0 || inHouse < 0 {
		t.Fatalf("the stylesheet lacks a back layer: downloaded at %d, in-house at %d", downloaded, inHouse)
	}
	if downloaded > inHouse {
		t.Error("the downloaded back must be the top layer, listed before the in-house one")
	}
}

func TestStaticAssets_areServedAndHealthzAnswers(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	for path, wantType := range map[string]string{
		"/static/style.css": "text/css",
		"/static/reveal.js": "javascript",
		"/healthz":          "",
	} {
		resp, _ := get(t, c, srv.URL+path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); !strings.Contains(got, wantType) {
			t.Errorf("GET %s Content-Type = %q, want %q", path, got, wantType)
		}
	}
}

func TestOpen_aBrokenCatalogueIsAServerErrorThatIsLoggedAndNotShownToThePlayer(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	rav.BoosterOptions = nil
	srv, c, logs := newLoggedServer(t, fakeImages{}, rav)

	resp, err := c.PostForm(srv.URL+"/open", url.Values{"set": {rav.Code}})
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
	if strings.Contains(body, "booster options") {
		t.Errorf("the internal error leaked to the page:\n%s", body)
	}
	if !strings.Contains(logs.String(), "booster options") {
		t.Errorf("the internal error was not logged:\n%s", logs.String())
	}
}

func TestPack_showsThePictureOfACardThatHasAScryfallIDAndNoneOtherwise(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	withScryfallIDs(rav)
	// The rare is the one card without an id; every Ravnica booster of the test catalogue holds it.
	const rareID = domain.CardID("rav-r-000")
	rare := rav.Cards[rareID]
	rare.ScryfallID = ""
	rav.Cards[rareID] = rare
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/pack")

	cards, pictures := strings.Count(body, "data-card "), strings.Count(body, "<img ")
	if pictures != cards-1 {
		t.Errorf("%d pictures for %d cards, want one fewer picture than cards:\n%s", pictures, cards, body)
	}
	if !strings.Contains(body, `<img src="/card-image/00000000-0000-4000-8000-`) {
		t.Errorf("a picture does not point at /card-image/<scryfall id>:\n%s", body)
	}
	if !strings.Contains(body, `alt="Ravnica Common`) {
		t.Errorf("a picture lacks the card name as its alt text:\n%s", body)
	}
}

// The reveal shows one card at a time and hides the others, and a browser does not load a lazy
// picture that is hidden. Each next card would then wait for its download when it turns up.
func TestPack_picturesAreNotLazyBecauseTheRevealHidesThem(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	withScryfallIDs(rav)
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/pack")

	if !strings.Contains(body, "<img ") {
		t.Fatalf("the page has no picture, the test covers nothing:\n%s", body)
	}
	if strings.Contains(body, `loading="lazy"`) {
		t.Errorf("a picture is lazy-loaded:\n%s", body)
	}
}

// Without JavaScript the booster page shows every card face up, and the reveal script then turns it
// into one face-down card at a time. Until it has, the cards must not be seen, or they flash. The
// pages CSP forbids inline scripts, so a small blocking script in the head flags that scripting is
// on, and the stylesheet hides the booster until the reveal has taken over. A booster whose script
// never runs must still show up, which a delayed animation takes care of.
func TestPack_hidesTheCardsUntilTheRevealTakesOverButNeverForever(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, page := get(t, c, srv.URL+"/pack")
	flagScript := `<script src="/static/js-flag.js"></script>`
	flag, main := strings.Index(page, flagScript), strings.Index(page, "<main")
	if flag < 0 || main < 0 || flag > main {
		t.Fatalf("the blocking script must be in the page before its content (script at %d, main at %d):\n%s", flag, main, page)
	}

	resp, script := get(t, c, srv.URL+"/static/js-flag.js")
	if resp.StatusCode != http.StatusOK || !strings.Contains(script, "classList.add('js')") {
		t.Errorf("GET /static/js-flag.js = %d, want a script that flags the page as scripted:\n%s", resp.StatusCode, script)
	}

	_, css := get(t, c, srv.URL+"/static/style.css")
	rule := strings.Index(css, ".js .booster:not(.staged)")
	if rule < 0 {
		t.Fatalf("the stylesheet does not hide a booster that is not staged yet:\n%s", css)
	}
	hidden := css[rule:]
	if end := strings.Index(hidden, "}"); end >= 0 {
		hidden = hidden[:end]
	}
	if !strings.Contains(hidden, "visibility: hidden") || !strings.Contains(hidden, "animation:") {
		t.Errorf("the rule must hide the booster and give it a fallback animation, got: %s", hidden)
	}
	if !strings.Contains(css, "@keyframes show-booster") {
		t.Error("the fallback animation that shows the booster again is not defined")
	}
}

func TestCardImage_servesThePictureWithACacheHeader(t *testing.T) {
	t.Parallel()
	const id = "00000000-0000-4000-8000-000000000001"
	pics := fakeImages{pictures: map[string]images.Image{id: {ContentType: "image/jpeg", Data: []byte("jpegdata")}}}
	srv, c, _ := newLoggedServer(t, pics, domaintest.NewRavnicaCatalogue())

	resp, body := get(t, c, srv.URL+"/card-image/"+id)

	if resp.StatusCode != http.StatusOK || body != "jpegdata" {
		t.Fatalf("status %d body %q, want 200 and the picture", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if resp.Header.Get("Cache-Control") == "" {
		t.Error("Cache-Control missing: the browser would ask again for every card")
	}
}

func TestCardImage_failuresUseTheirStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		path       string
		err        error
		wantStatus int
		wantLogged bool
	}{
		{"not a Scryfall id", "/card-image/not-an-id", nil, http.StatusBadRequest, false},
		{"unknown picture", "/card-image/00000000-0000-4000-8000-000000000002", nil, http.StatusNotFound, false},
		{"source down", "/card-image/00000000-0000-4000-8000-000000000003", errors.New("offline"), http.StatusBadGateway, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, c, logs := newLoggedServer(t, fakeImages{err: tc.err}, domaintest.NewRavnicaCatalogue())

			resp, _ := get(t, c, srv.URL+tc.path)

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if got := strings.Contains(logs.String(), "offline"); got != tc.wantLogged {
				t.Errorf("error logged = %v, want %v:\n%s", got, tc.wantLogged, logs.String())
			}
		})
	}
}

func TestPages_carryTheFanContentNotice(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	_, body := get(t, c, srv.URL+"/")

	for _, want := range []string{"unofficial Fan Content", "Not approved/endorsed by Wizards", "Wizards of the Coast"} {
		if !strings.Contains(body, want) {
			t.Errorf("footer lacks %q:\n%s", want, body)
		}
	}
}

// slotValue reads the value of the binder's progress bar.
func slotValue(t *testing.T, body string) int {
	t.Helper()
	m := regexp.MustCompile(`<progress value="(\d+)" max="(\d+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no progress bar on the page:\n%s", body)
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func TestCollection_showsTheFirstBinderWithEmptySlotsBeforeAnythingIsOpened(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)

	resp, body := get(t, c, srv.URL+"/collection")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	const slots = 16 // 11 commons, 4 uncommons and 1 rare, no basic land
	for _, want := range []string{rav.Name, `<progress value="0" max="16"`, "0 / 16 cards", "Rare", "0 / 1", "playsets"} {
		if !strings.Contains(body, want) {
			t.Errorf("collection page lacks %q:\n%s", want, body)
		}
	}
	if got := strings.Count(body, `class="slot empty"`); got != slots {
		t.Errorf("%d empty slots, want %d", got, slots)
	}
	if strings.Contains(body, "data-card ") {
		t.Errorf("a card is shown before any booster was opened:\n%s", body)
	}
}

func TestCollection_showsWhatWasOpenedAndLeavesTheRestEmpty(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/collection")

	owned := slotValue(t, body)
	if owned == 0 {
		t.Fatalf("progress is 0 after a booster:\n%s", body)
	}
	if got := strings.Count(body, "data-card "); got != owned {
		t.Errorf("%d cards shown, want the %d the progress bar counts", got, owned)
	}
	if got := strings.Count(body, `class="slot empty"`); got != 16-owned {
		t.Errorf("%d empty slots, want %d", got, 16-owned)
	}
	if !strings.Contains(body, "Ravnica Common") {
		t.Errorf("no owned card named on the page:\n%s", body)
	}
}

func TestCollection_switchesBetweenBinders(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	srv, c := newServer(t, rav, tsp)

	_, first := get(t, c, srv.URL+"/collection")
	_, second := get(t, c, srv.URL+"/collection?set="+tsp.Code)

	for _, body := range []string{first, second} {
		for _, want := range []string{`href="/collection?set=` + rav.Code + `"`, `href="/collection?set=` + tsp.Code + `"`} {
			if !strings.Contains(body, want) {
				t.Errorf("the binder switcher lacks %q:\n%s", want, body)
			}
		}
	}
	if !strings.Contains(first, "<h1>"+rav.Name) || !strings.Contains(second, "<h1>"+tsp.Name) {
		t.Errorf("the page does not show the binder it was asked for")
	}
	if strings.Count(second, `aria-current="page"`) != 1 || !strings.Contains(second, `aria-current="page">`+tsp.Name) {
		t.Errorf("the current binder is not marked in the switcher:\n%s", second)
	}
}

func TestCollection_anUnknownSetIsABadRequest(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	resp, body := get(t, c, srv.URL+"/collection?set=NOPE")

	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(strings.ToLower(body), "unknown set") {
		t.Errorf("GET /collection?set=NOPE = %d, want 400 saying unknown set:\n%s", resp.StatusCode, body)
	}
}

func TestCollection_escapesCardNames(t *testing.T) {
	t.Parallel()
	const hostile = `<script>alert("x")</script>`
	rav := domaintest.NewRavnicaCatalogue()
	for id, card := range rav.Cards {
		card.Name = hostile
		rav.Cards[id] = card
	}
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/collection")

	if strings.Contains(body, hostile) || !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("card name not escaped on the collection page:\n%s", body)
	}
}

// The binder can hold hundreds of cards: its pictures wait until they scroll into view, unlike the
// reveal, where hidden pictures must load at once.
func TestCollection_picturesOfOwnedCardsAreLazy(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	withScryfallIDs(rav)
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/collection")

	pictures := strings.Count(body, `<img src="/card-image/`)
	if pictures == 0 {
		t.Fatalf("no picture on the collection page:\n%s", body)
	}
	if lazy := strings.Count(body, `loading="lazy"`); lazy != pictures {
		t.Errorf("%d of %d pictures are lazy-loaded", lazy, pictures)
	}
}

func TestCollection_everyPageLinksToIt(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	_, body := get(t, c, srv.URL+"/")

	if !strings.Contains(body, `href="/collection"`) {
		t.Errorf("the home page has no link to the collection:\n%s", body)
	}
}
