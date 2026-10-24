package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// huntForm is cardForm for the card of the slot a hunt is started from.
func huntForm(card string) url.Values { return cardForm("S", card, "nonfoil") }

func (s counterShop) hunt(t *testing.T, card string) {
	t.Helper()
	if resp, body := s.post(t, "/hunt", huntForm(card)); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /hunt %s = %d:\n%s", card, resp.StatusCode, body)
	}
}

// huntBadge is the count of the Hunted link of the header, 0 without a badge.
func huntBadge(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`<a href="/hunted">Hunted(?: <span class="badge">(\d+)<span class="sr-only"> new at the Counter</span></span>)?</a>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the header has no link to /hunted:\n%s", body)
	}
	if m[1] == "" {
		return "0"
	}
	return m[1]
}

func TestHuntedSlot_anEmptySlotOffersToHuntAndAHuntedOneToStop(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	before := slot(t, shop.get(t, "/collection?set=S"), neverCard)
	shop.hunt(t, neverCard)
	after := slot(t, shop.get(t, "/collection?set=S"), neverCard)

	mustContain(t, "an empty slot", before, `action="/hunt"`, `name="card" value="S-02"`, ">Hunt S-02<")
	mustNotContain(t, "an empty slot", before, `action="/unhunt"`)
	mustContain(t, "a hunted slot", after, `action="/unhunt"`, "Stop hunting")
	mustNotContain(t, "a hunted slot", after, `action="/hunt"`)
}

func TestHuntedSlot_aHeldCardCannotBeHunted(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 1)

	got := slot(t, shop.get(t, "/collection?set=S"), cheapCard)

	mustNotContain(t, "a held slot", got, `action="/hunt"`)
}

func TestHunt_startsTheHuntAndComesBackToTheSlot(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	form := huntForm(neverCard)
	form.Set("rarity", "uncommon")

	resp, _ := shop.post(t, "/hunt", form)

	redirectsTo(t, "POST /hunt", resp, "/collection?rarity=uncommon&set=S#c-S-02")
	mustContain(t, "the binder", shop.get(t, "/collection?set=S"), "Hunting S-02")
}

func TestHunt_refusalsShowAMessageAndChangeNothing(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 1)
	tests := []struct {
		name   string
		form   url.Values
		status int
		want   string
	}{
		{"a card the player holds", huntForm(cheapCard), http.StatusConflict, "already hold"},
		{"a card of no catalogue", huntForm("nope"), http.StatusBadRequest, "Unknown card"},
		{"no card at all", cardForm("S", "", "nonfoil"), http.StatusBadRequest, "Unknown card"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := shop.post(t, "/hunt", tt.form)
			if resp.StatusCode != tt.status {
				t.Errorf("status = %d, want %d:\n%s", resp.StatusCode, tt.status, body)
			}
			mustContain(t, "the refusal", body, tt.want)
		})
	}
	if hunted, err := shop.game.Hunted(); err != nil || len(hunted) != 0 {
		t.Errorf("hunted = %+v, %v after the refusals, want nothing", hunted, err)
	}
}

func TestUnhunt_stopsTheHuntAndComesBackToWhereItWasAskedFrom(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.hunt(t, neverCard)
	shop.hunt(t, unpricedKey)

	fromSlot, _ := shop.post(t, "/unhunt", huntForm(neverCard))
	fromPage := huntForm(unpricedKey)
	fromPage.Set("from", "hunted")
	fromHunted, _ := shop.post(t, "/unhunt", fromPage)

	redirectsTo(t, "POST /unhunt from a slot", fromSlot, "/collection?set=S#c-S-02")
	redirectsTo(t, "POST /unhunt from the page", fromHunted, "/hunted")
	if hunted, _ := shop.game.Hunted(); len(hunted) != 0 {
		t.Errorf("hunted = %+v, want nothing", hunted)
	}
}

func TestHuntedPage_listsTheHuntsWithWhereTheyStand(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.hunt(t, cheapCard)
	shop.hunt(t, neverCard)

	got := shop.get(t, "/hunted")

	mustContain(t, "the page", got, "<h1>Hunted cards</h1>", "S-00", "S-02", `action="/unhunt"`, `name="from" value="hunted"`)
	mustContain(t, "the card at the Counter", got, "At the Counter for ◎120", `<span class="tag new">New</span>`)
	mustContain(t, "the card the NPCs hold", got, "1 offer waiting", `href="/market"`)
	mustContain(t, "the card nobody has", got, "Not at the Counter")
}

func TestHuntedPage_hasAnEmptyState(t *testing.T) {
	t.Parallel()

	got := newCounterShop(t).get(t, "/hunted")

	mustContain(t, "the page", got, "You are not hunting any card", `href="/collection"`)
}

func TestHuntedPage_aGameWithoutACounterHasNone(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	resp, _ := get(t, c, srv.URL+"/hunted")

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /hunted = %d, want 404 in a game with no Counter", resp.StatusCode)
	}
}

func TestHeader_theHuntedLinkCountsWhatIsNewAtTheCounterUntilThePlayerSeesIt(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.hunt(t, cheapCard)
	shop.hunt(t, neverCard)

	before := huntBadge(t, shop.get(t, "/collection?set=S"))
	shop.get(t, "/hunted")
	after := huntBadge(t, shop.get(t, "/collection?set=S"))

	if before != "1" || after != "0" {
		t.Errorf("the badge says %s then %s, want 1 (the card at the Counter) then none once the page showed it", before, after)
	}
}

func TestHeader_marksThePageThePlayerIsOn(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 1)
	nav := regexp.MustCompile(`(?s)<nav class="site-nav".*?</nav>`)
	current := regexp.MustCompile(`<a href="([^"]+)" aria-current="page">`)
	tests := []struct{ path, want string }{
		{"/", "/"},
		{"/pack", "/"},
		{"/collection?set=S", "/collection"},
		{"/hunted", "/hunted"},
		{"/market", "/market"},
	}

	for _, tt := range tests {
		got := current.FindAllStringSubmatch(nav.FindString(shop.get(t, tt.path)), -1)

		if len(got) != 1 || got[0][1] != tt.want {
			t.Errorf("%s marks %v as the current page, want only %s", tt.path, got, tt.want)
		}
	}
}

func TestHeader_noHuntedLinkInAGameWithoutACounter(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, domaintest.NewRavnicaCatalogue())

	_, body := get(t, c, srv.URL+"/")

	mustNotContain(t, "the home page", body, `href="/hunted"`)
}
