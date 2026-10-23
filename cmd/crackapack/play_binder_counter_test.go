package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// postBody posts form to path, follows the redirect as a browser does, and returns the page it ends on.
func postBody(t *testing.T, base, path string, form url.Values) string {
	t.Helper()
	resp, err := http.PostForm(base+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return readAll(t, resp)
}

// The demo's binder shows the three states of an empty slot at the Counter: a card in stock with a
// Buy button, one the Counter never had, and, a day after the player sold it, one the Counter had.
// It also asks before a sale empties a slot, as it does for the player.
func TestRunPlay_theDemoBinderShowsEveryStateOfTheCounter(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	a := demoPlayArgs(t)
	a.now = clock.Now
	base := servePlay(t, a)

	binder := getBody(t, base+"/collection?set=DEM")
	for _, want := range []string{`class="coins"`, "<strong>200</strong>", `action="/buy"`, "Out of stock, never seen at the Counter"} {
		if !strings.Contains(binder, want) {
			t.Errorf("a new demo binder lacks %q:\n%s", want, binder)
		}
	}
	if strings.Contains(binder, "last in stock") {
		t.Errorf("a new demo binder has a card the Counter once had:\n%s", binder)
	}

	postBody(t, base, "/open", url.Values{"set": {"DEM"}})
	const uncommons = "/collection?set=DEM&rarity=uncommon"
	sold := regexp.MustCompile(`action="/sell" class="sell">\s*<input type="hidden" name="set" value="DEM"><input type="hidden" name="card" value="([^"]+)">`).FindAllStringSubmatch(getBody(t, base+uncommons), -1)
	if len(sold) == 0 {
		t.Fatal("a booster of the demo gave no uncommon to sell")
	}
	form := func(id string) url.Values {
		return url.Values{"set": {"DEM"}, "card": {id}, "finish": {"nonfoil"}, "qty": {"1"}, "rarity": {"uncommon"}}
	}
	asked := postBody(t, base, "/sell", form(sold[0][1]))
	if !strings.Contains(asked, "last copy") {
		t.Fatalf("selling the only copy did not ask first:\n%s", asked)
	}
	for _, id := range sold {
		confirmed := form(id[1])
		confirmed.Set("confirm", "yes")
		if page := postBody(t, base, "/sell", confirmed); !strings.Contains(page, "Sold 1") {
			t.Fatalf("selling %s did not answer with the sale:\n%s", id[1], page)
		}
	}

	clock.Advance(48 * time.Hour)

	if page := getBody(t, base+uncommons); !strings.Contains(page, "last in stock") {
		t.Errorf("the slots of the cards sold two days ago do not say when the Counter last had them:\n%s", page)
	}
}
