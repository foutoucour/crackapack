package web_test

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/images"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>`

// iconServer serves a game on sets, with a source that knows the icon of RAV only.
func iconServer(t *testing.T, icons images.Source, sets ...domain.SetCatalogue) (*httptest.Server, *http.Client) {
	t.Helper()
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()},
		rand.New(rand.NewPCG(testSeed, testSeed)),
	)
	game, err := play.NewGame(sets, nil, opener, playtest.Rules(sets...), playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	srv, c, _ := serveWithIcons(t, game, fakeImages{}, icons)
	return srv, c
}

func ravWithIcon() domain.SetCatalogue {
	rav := domaintest.NewRavnicaCatalogue()
	rav.IconCode = "RAV"
	return rav
}

func ravIcons() fakeImages {
	return fakeImages{pictures: map[string]images.Image{"RAV": {ContentType: "image/svg+xml", Data: []byte(testSVG)}}}
}

func TestSetIcon_servesTheSVGWithACacheHeader(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	resp, body := get(t, c, srv.URL+"/set-icon/RAV")

	if resp.StatusCode != http.StatusOK || body != testSVG {
		t.Fatalf("status %d body %q, want 200 and the icon", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
	if resp.Header.Get("Cache-Control") == "" {
		t.Error("Cache-Control missing: the browser would ask again for every page")
	}
}

func TestSetIcon_aSetWithoutAPictureGetsTheBuiltInIconSoTheButtonIsNeverBlank(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		icons      fakeImages
		wantLogged bool
	}{
		{"icon unknown to the source", fakeImages{}, false},
		{"source down", fakeImages{err: errors.New("offline")}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opener := domain.NewOpener(
				map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()},
				rand.New(rand.NewPCG(testSeed, testSeed)),
			)
			sets := []domain.SetCatalogue{ravWithIcon()}
			game, err := play.NewGame(sets, nil, opener, playtest.Rules(sets...), playtest.NewClock().Now)
			if err != nil {
				t.Fatalf("NewGame: %v", err)
			}
			srv, c, logs := serveWithIcons(t, game, fakeImages{}, tc.icons)

			resp, body := get(t, c, srv.URL+"/set-icon/RAV")

			if resp.StatusCode != http.StatusOK || !strings.HasPrefix(body, "<svg") {
				t.Fatalf("status %d body %q, want 200 and an SVG", resp.StatusCode, body)
			}
			if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
				t.Errorf("Content-Type = %q, want image/svg+xml", got)
			}
			if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
				t.Errorf("Cache-Control = %q: a stand-in must not stay in the browser once the real icon can be had", cc)
			}
			if got := strings.Contains(logs.String(), "offline"); got != tc.wantLogged {
				t.Errorf("error logged = %v, want %v:\n%s", got, tc.wantLogged, logs.String())
			}
		})
	}
}

func TestSetIcon_aBadCodeIsABadRequest(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	resp, _ := get(t, c, srv.URL+"/set-icon/not-a-code!")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSetIconsStylesheet_masksEachSetIconByClassAndSkipsSetsWithoutOne(t *testing.T) {
	t.Parallel()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	tsp.IconCode = ""
	srv, c := iconServer(t, ravIcons(), ravWithIcon(), tsp)

	resp, body := get(t, c, srv.URL+"/set-icons.css")

	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/css") {
		t.Fatalf("status %d type %q, want 200 text/css", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, want := range []string{".set-icon-rav", `url("/set-icon/RAV")`} {
		if !strings.Contains(body, want) {
			t.Errorf("stylesheet lacks %q:\n%s", want, body)
		}
	}
	if strings.Count(body, "mask-image") != 1 {
		t.Errorf("stylesheet has %d rules, want 1: the set without an icon has none:\n%s", strings.Count(body, "mask-image"), body)
	}
}

func TestPages_linkTheSetIconsStylesheet(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	_, body := get(t, c, srv.URL+"/")

	if !strings.Contains(body, `href="/set-icons.css"`) {
		t.Errorf("the page does not link the set icons stylesheet:\n%s", body)
	}
}

func TestHome_theOpenButtonOfASetCarriesItsIconInCommonBlackUntilARareIsHeld(t *testing.T) {
	t.Parallel()
	rav := ravWithIcon()
	srv, c := iconServer(t, ravIcons(), rav)

	_, before := get(t, c, srv.URL+"/")
	openPack(t, c, srv.URL, rav.Code)
	_, after := get(t, c, srv.URL+"/")

	if !strings.Contains(before, `class="set-icon set-icon-rav rarity-common"`) {
		t.Errorf("before any booster the icon is not common:\n%s", before)
	}
	if strings.Contains(after, `class="set-icon set-icon-rav rarity-common"`) {
		t.Errorf("a booster always gives a rare or a mythic, yet the icon is still common:\n%s", after)
	}
	if !strings.Contains(after, `class="set-icon set-icon-rav rarity-rare"`) && !strings.Contains(after, `class="set-icon set-icon-rav rarity-mythic"`) {
		t.Errorf("the icon is neither rare nor mythic after a booster:\n%s", after)
	}
}

func TestIcon_sitsOnAChipOfTheRaritySoItReadsOnAnySurface(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	_, body := get(t, c, srv.URL+"/")

	want := `<span class="set-icon-chip rarity-common" aria-hidden="true"><span class="set-icon set-icon-rav rarity-common"></span></span>`
	if !strings.Contains(body, want) {
		t.Errorf("the icon is not on a chip, want %s in:\n%s", want, body)
	}
}

func TestHome_aSetWithoutAnIconCodeShowsNoIcon(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), domaintest.NewRavnicaCatalogue())

	_, body := get(t, c, srv.URL+"/")

	if strings.Contains(body, `class="set-icon `) {
		t.Errorf("an icon is shown for a set that has no code:\n%s", body)
	}
}

func TestCollection_theBinderTabsCarryTheIconOfTheirSet(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	_, body := get(t, c, srv.URL+"/collection")

	nav := body[strings.Index(body, `aria-label="Binders"`):]
	nav = nav[:strings.Index(nav, "</nav>")]
	if !strings.Contains(nav, `class="set-icon set-icon-rav rarity-`) {
		t.Errorf("the tab of the set has no icon:\n%s", nav)
	}
}

func TestCollection_theHeaderCarriesAFaintCopyOfTheSetIconInTheColourOfTheRarestCardHeld(t *testing.T) {
	t.Parallel()
	rav := ravWithIcon()
	srv, c := iconServer(t, ravIcons(), rav)

	_, empty := get(t, c, srv.URL+"/collection")

	if want := `<span class="set-watermark set-icon-rav rarity-common" aria-hidden="true"></span>`; !strings.Contains(empty, want) {
		t.Errorf("the header lacks %q:\n%s", want, empty)
	}
}

func TestCollection_aSetWithoutAnIconHasNoWatermark(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), domaintest.NewRavnicaCatalogue())

	_, body := get(t, c, srv.URL+"/collection")

	if strings.Contains(body, "set-watermark") {
		t.Errorf("a watermark is drawn for a set that has no icon:\n%s", body)
	}
}

func TestPack_everyCardCarriesTheSetIconInTheColourOfItsRarity(t *testing.T) {
	t.Parallel()
	rav := ravWithIcon()
	srv, c := iconServer(t, ravIcons(), rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/pack")

	cards := strings.Count(body, "data-card ")
	if cards == 0 {
		t.Fatalf("the page shows no card:\n%s", body)
	}
	for _, rarity := range []string{"common", "uncommon", "rare", "mythic"} {
		inCards := strings.Count(body, `data-rarity="`+rarity+`"`)
		withIcon := strings.Count(body, `class="set-icon set-icon-rav rarity-`+rarity+`"`)
		if inCards != withIcon {
			t.Errorf("%s: %d cards but %d icons in that colour", rarity, inCards, withIcon)
		}
	}
}

func TestCollection_aCardOfTheBinderCarriesNoSetIcon(t *testing.T) {
	t.Parallel()
	rav := ravWithIcon()
	srv, c := iconServer(t, ravIcons(), rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/collection?set="+url.QueryEscape(rav.Code))

	binder := body[strings.Index(body, "data-binder"):]
	if strings.Contains(binder, "set-icon") {
		t.Errorf("a card or a slot of the binder carries the set icon:\n%s", binder)
	}
}

func TestStylesheet_drawsTheSetIconInTheColourOfItsRarity(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	_, css := get(t, c, srv.URL+"/static/style.css")

	for _, want := range []string{
		"--rarity-common: #000000;",
		"--rarity-uncommon: #c0c0c0;",
		"--rarity-rare: #d4af37;",
		"--rarity-mythic: #9d4edd;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("the stylesheet lacks %q: black, silver, gold and purple for common to mythic", want)
		}
	}
	for _, rarity := range []string{"common", "uncommon", "rare", "mythic"} {
		rule := ".set-icon.rarity-" + rarity + " { background-color: var(--rarity-" + rarity + ");"
		if !strings.Contains(css, rule) {
			t.Errorf("the stylesheet lacks the rule %q", rule)
		}
	}
	for _, want := range []string{".set-icon-chip {", ".set-icon-chip.rarity-common {"} {
		if !strings.Contains(css, want) {
			t.Errorf("the stylesheet lacks the chip rule %q: a black icon needs a light chip, the other colours a dark one", want)
		}
	}
	if !strings.Contains(css, "mask-size: contain") {
		t.Error("the stylesheet does not fit the masked icon in its box")
	}
}
