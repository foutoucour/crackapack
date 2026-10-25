package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

var (
	cardPictures    = regexp.MustCompile(`<img src="(/card-image/[^"]+)"`)
	preloadedImages = regexp.MustCompile(`<link rel="preload" as="image" href="(/card-image/[^"]+)">`)
)

func TestPack_preloadsThePicturesOfTheFirstTheSecondAndTheLastCard(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	withScryfallIDs(rav)
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/pack")

	pictures := cardPictures.FindAllStringSubmatch(body, -1)
	if len(pictures) < 3 {
		t.Fatalf("the booster has %d pictures, the test needs at least three:\n%s", len(pictures), body)
	}
	var want []string
	for _, i := range []int{0, 1, len(pictures) - 1} {
		want = append(want, pictures[i][1])
	}
	var got []string
	for _, m := range preloadedImages.FindAllStringSubmatch(body, -1) {
		got = append(got, m[1])
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("preloaded %v, want the first, the second and the last picture %v", got, want)
	}
	if strings.Index(body, "rel=\"preload\"") > strings.Index(body, "data-booster") {
		t.Error("the pictures are preloaded after the booster that shows them")
	}
}

func TestPack_aNewRareHasTheEffectOfItsOwnAndOtherCardsHaveNone(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/pack")

	rares := strings.Count(body, `data-rarity="rare"`)
	if rares == 0 {
		t.Fatalf("the booster holds no rare, the test covers nothing:\n%s", body)
	}
	if got := strings.Count(body, " new-rare"); got != rares {
		t.Errorf("%d cards carry new-rare, want the %d new rares of the first booster", got, rares)
	}
	if strings.Contains(body, " new-common") || strings.Contains(body, " new-uncommon") {
		t.Errorf("a card that is not a rare or a mythic has an effect:\n%s", body)
	}
}

func TestPack_everyCardOfAGameWithACounterShowsItsValueInCoins(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	if _, err := shop.game.Open("S"); err != nil {
		t.Fatalf("Open: %v", err)
	}

	body := shop.get(t, "/pack")

	// The four cards are worth $1, $2.50, $1 and nothing, at 100 coins a dollar. The card worth
	// nothing has no price to show.
	if got := strings.Count(body, `class="price"`); got != 3 {
		t.Errorf("%d prices on the page, want 3:\n%s", got, body)
	}
	mustContain(t, "the pack page", body, "◎100", "◎250")
}

func TestPack_aGameWithoutACounterShowsNoPrice(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)

	_, body := get(t, c, srv.URL+"/pack")

	if strings.Contains(body, `class="price"`) {
		t.Errorf("a game without coins shows a price:\n%s", body)
	}
}

func TestPack_theTitleCarriesTheIconOfTheSetWhenItHasOne(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		iconCode string
		wantIcon bool
	}{
		{"a set with an icon", "RAV", true},
		{"a set without one", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rav := ravWithIcon()
			rav.IconCode = tt.iconCode
			srv, c := iconServer(t, ravIcons(), rav)
			openPack(t, c, srv.URL, rav.Code)

			_, body := get(t, c, srv.URL+"/pack")

			_, rest, _ := strings.Cut(body, "<h1>")
			title, _, found := strings.Cut(rest, "</h1>")
			if !found || !strings.Contains(title, rav.Name) {
				t.Fatalf("the title does not name the set %q: %q", rav.Name, title)
			}
			if got := strings.Contains(title, `class="title-icon set-icon-rav"`); got != tt.wantIcon {
				t.Errorf("icon in the title = %v, want %v: %q", got, tt.wantIcon, title)
			}
		})
	}
}

func TestPack_theButtonsSitOnTheIconOfTheSet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		iconCode string
		wantIcon bool
	}{
		{"a set with an icon", "RAV", true},
		{"a set without one", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rav := ravWithIcon()
			rav.IconCode = tt.iconCode
			srv, c := iconServer(t, ravIcons(), rav)
			openPack(t, c, srv.URL, rav.Code)

			_, body := get(t, c, srv.URL+"/pack")

			_, actions, found := strings.Cut(body, `class="actions"`)
			if !found {
				t.Fatalf("no actions on the pack page:\n%s", body)
			}
			actions, _, _ = strings.Cut(actions, "</nav>")
			if got := strings.Contains(actions, `class="bar-icon set-icon-rav"`); got != tt.wantIcon {
				t.Errorf("icon behind the buttons = %v, want %v:\n%s", got, tt.wantIcon, actions)
			}
		})
	}
}

func TestPack_comesBackWithTheCardsAlreadyTurned(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	openPack(t, c, srv.URL, rav.Code)
	_, fresh := get(t, c, srv.URL+"/pack")
	mustContain(t, "the pack page of a booster just opened", fresh, `data-revealed="0"`)

	resp, err := c.PostForm(srv.URL+"/pack/revealed", url.Values{"count": {"2"}})
	if err != nil {
		t.Fatalf("POST /pack/revealed: %v", err)
	}
	_ = readBody(t, resp)
	_, back := get(t, c, srv.URL+"/pack")

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("POST /pack/revealed = %d, want 204", resp.StatusCode)
	}
	mustContain(t, "the pack page after two cards were turned", back, `data-revealed="2"`)
}

func TestPackRevealed_refusesACountThatIsNotOneOfTheBooster(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c := newServer(t, rav)
	post := func(count string) int {
		resp, err := c.PostForm(srv.URL+"/pack/revealed", url.Values{"count": {count}})
		if err != nil {
			t.Fatalf("POST /pack/revealed: %v", err)
		}
		_ = readBody(t, resp)
		return resp.StatusCode
	}
	if got := post("1"); got != http.StatusConflict {
		t.Errorf("a count before any booster was opened = %d, want 409", got)
	}
	openPack(t, c, srv.URL, rav.Code)

	for _, count := range []string{"", "two", "-1", "999"} {
		if got := post(count); got != http.StatusBadRequest {
			t.Errorf("count %q = %d, want 400", count, got)
		}
	}
}
