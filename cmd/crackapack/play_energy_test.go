package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var costPattern = regexp.MustCompile(`(?s)<h2>([^<]*)</h2>\s*<p class="count">(\d+) energy</p>`)

// tileCosts reads the energy each tile of the home page asks for, by set name.
func tileCosts(t *testing.T, home string) map[string]string {
	t.Helper()
	costs := make(map[string]string)
	for _, m := range costPattern.FindAllStringSubmatch(home, -1) {
		costs[m[1]] = m[2]
	}
	return costs
}

// spendWelcomeBoosters opens every free welcome booster of the demo, five of each set.
func spendWelcomeBoosters(t *testing.T, base string) {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, set := range []string{"DEM", "DSP"} {
		for range 5 {
			resp, err := client.PostForm(base+"/open", url.Values{"set": {set}})
			if err != nil {
				t.Fatalf("POST /open %s: %v", set, err)
			}
			_ = readAll(t, resp)
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("a welcome booster of %s was refused: %d", set, resp.StatusCode)
			}
		}
	}
}

func getBody(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return readAll(t, resp)
}

func TestRunPlay_theHeaderShowsTheEnergyOfAFreshSession(t *testing.T) {
	t.Parallel()
	base := servePlay(t, demoPlayArgs(t))

	home := getBody(t, base+"/")

	for _, want := range []string{"<meter", "10 / 10", "Welcome gift"} {
		if !strings.Contains(home, want) {
			t.Errorf("home page lacks %q:\n%s", want, home)
		}
	}
}

// The demo prices make its Time Spiral-style set worth more than its Ravnica-style one, so the
// boosters cost different energy. If the demo data changes these change with it, which is the point
// of pinning them: the costs must come from the prices and the median of the catalogue.
func TestRunPlay_theDemoPricesGiveEachSetItsCost(t *testing.T) {
	t.Parallel()
	base := servePlay(t, demoPlayArgs(t))
	spendWelcomeBoosters(t, base)

	costs := tileCosts(t, getBody(t, base+"/"))

	want := map[string]string{"Demo Set (Ravnica-style)": "5", "Demo Spiral Set (Time Spiral-style)": "8"}
	for set, cost := range want {
		if costs[set] != cost {
			t.Errorf("%s costs %q energy, want %s (all: %v)", set, costs[set], cost, costs)
		}
	}
}

func TestRunPlay_withoutPricesEverySetCostsTheMiddleTier(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	a.pricesFS, a.pricesName = nil, ""
	base := servePlay(t, a)
	spendWelcomeBoosters(t, base)

	costs := tileCosts(t, getBody(t, base+"/"))

	if len(costs) != 2 {
		t.Fatalf("read %d costs from the tiles, want 2: %v", len(costs), costs)
	}
	for set, cost := range costs {
		if cost != "5" {
			t.Errorf("%s costs %s energy with no prices, want the x1 tier, 5", set, cost)
		}
	}
}

func TestRunPlay_aBoosterBeyondTheEnergyIsRefusedAndSaysWhy(t *testing.T) {
	t.Parallel()
	base := servePlay(t, demoPlayArgs(t))
	spendWelcomeBoosters(t, base)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// A reserve of 10 cannot pay for more than three of the dearest boosters, a tier of 3 at the
	// cheapest: the energy runs out well within ten openings.
	var refused string
	for range 10 {
		resp, err := client.PostForm(base+"/open", url.Values{"set": {"DEM"}})
		if err != nil {
			t.Fatalf("POST /open: %v", err)
		}
		body := readAll(t, resp)
		if resp.StatusCode == http.StatusConflict {
			refused = body
			break
		}
	}

	if refused == "" {
		t.Fatal("ten boosters in a row were all opened: the energy never ran out")
	}
	for _, want := range []string{"Not enough energy", "costs", "Enough in"} {
		if !strings.Contains(refused, want) {
			t.Errorf("the refusal lacks %q:\n%s", want, refused)
		}
	}
}

func TestRunPlay_aPricesFileThatCannotBeReadIsAnErrorNamingIt(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	a.pricesName = "missing-prices.json"
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	a.listener = ln
	a.stdout, a.stderr = io.Discard, io.Discard

	err = runPlay(context.Background(), a)

	if err == nil || !strings.Contains(err.Error(), "missing-prices.json") {
		t.Fatalf("runPlay error = %v, want one naming missing-prices.json", err)
	}
}
