package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func TestHome_aSetRowHasItsIconAsAWatermarkBehindIt(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	_, body := get(t, c, srv.URL+"/")

	want := `<span class="pack-watermark set-icon-rav" aria-hidden="true"></span>`
	if !strings.Contains(body, want) {
		t.Errorf("the row has no watermark, want %s in:\n%s", want, body)
	}
}

func TestHome_aSetWithoutAnIconCodeHasNoWatermark(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	rav.IconCode = ""
	srv, c := iconServer(t, ravIcons(), rav)

	_, body := get(t, c, srv.URL+"/")

	if strings.Contains(body, "pack-watermark") {
		t.Errorf("a set with no icon code has a watermark:\n%s", body)
	}
}

func TestHome_theTileOfAnUnfinishedBoosterLinksBackToItWithHowFarTheRevealWent(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	pc := domaintest.NewPlanarChaosCatalogue()
	srv, c := newServer(t, rav, pc)
	_, home := get(t, c, srv.URL+"/")
	if strings.Contains(home, `href="/pack"`) {
		t.Fatalf("the home page links to a booster before any was opened:\n%s", home)
	}
	openPack(t, c, srv.URL, rav.Code)
	_, pack := get(t, c, srv.URL+"/pack")
	cards := strings.Count(pack, "data-card ")
	postRevealed(t, c, srv.URL, 2)

	_, home = get(t, c, srv.URL+"/")

	tile := tileOf(t, home, rav.Code)
	mustContain(t, "the tile of the booster left halfway", tile, `href="/pack"`, fmt.Sprintf("2 of %d cards turned", cards))
	if strings.Contains(tile, `action="/open"`) {
		t.Errorf("the tile of the booster left halfway still offers a price to open another:\n%s", tile)
	}
	other := tileOf(t, home, pc.Code)
	if strings.Contains(other, `href="/pack"`) || !strings.Contains(other, `action="/open"`) {
		t.Errorf("the tile of another set must keep its own open button and not link to the booster:\n%s", other)
	}

	postRevealed(t, c, srv.URL, cards)
	_, home = get(t, c, srv.URL+"/")
	if strings.Contains(home, `href="/pack"`) || !strings.Contains(tileOf(t, home, rav.Code), `action="/open"`) {
		t.Errorf("a booster whose cards are all turned must give its tile back its open button:\n%s", home)
	}
}

// tileOf is the HTML of the tile of the set code on the home page.
func tileOf(t *testing.T, home, code string) string {
	t.Helper()
	_, rest, found := strings.Cut(home, `data-pack="`+code+`"`)
	if !found {
		t.Fatalf("no tile for %s:\n%s", code, home)
	}
	tile, _, _ := strings.Cut(rest, "</li>")
	return tile
}

// finishLastPack turns every card of the latest booster, which gives the tile of its set back the
// button that opens another.
func finishLastPack(t *testing.T, c *http.Client, base string) {
	t.Helper()
	_, pack := get(t, c, base+"/pack")
	postRevealed(t, c, base, strings.Count(pack, "data-card "))
}

func postRevealed(t *testing.T, c *http.Client, base string, count int) {
	t.Helper()
	resp, err := c.PostForm(base+"/pack/revealed", url.Values{"count": {strconv.Itoa(count)}})
	if err != nil {
		t.Fatalf("POST /pack/revealed: %v", err)
	}
	_ = readBody(t, resp)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /pack/revealed %d = %d, want 204", count, resp.StatusCode)
	}
}
