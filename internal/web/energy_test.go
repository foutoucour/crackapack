package web_test

import (
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// energyServer serves a session on the Ravnica test set whose boosters cost cost energy, on a
// clock the test moves.
func energyServer(t *testing.T, cost int) (*httptest.Server, *http.Client, *playtest.Clock, string) {
	t.Helper()
	rav := domaintest.NewRavnicaCatalogue()
	rules := playtest.Rules(rav)
	rules.Costs[rav.Code] = cost
	clock := playtest.NewClock()
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{
			domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
		},
		rand.New(rand.NewPCG(testSeed, testSeed)),
	)
	game, err := play.NewGame([]domain.SetCatalogue{rav}, nil, opener, rules, clock.Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	srv, client, _ := serve(t, game, fakeImages{})
	return srv, client, clock, rav.Code
}

// spendWelcome opens the welcome boosters, which are free.
func spendWelcome(t *testing.T, c *http.Client, base, code string) {
	t.Helper()
	for range play.WelcomeBoosters {
		openPack(t, c, base, code)
	}
}

var gaugePattern = regexp.MustCompile(`<meter[^>]*value="(\d+)"[^>]*max="(\d+)"`)

// gauge reads the energy meter of a page.
func gauge(t *testing.T, body string) (value, max int) {
	t.Helper()
	m := gaugePattern.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no energy meter on the page:\n%s", body)
	}
	value, _ = strconv.Atoi(m[1])
	max, _ = strconv.Atoi(m[2])
	return value, max
}

func TestEnergy_everyPageShowsTheGauge(t *testing.T) {
	t.Parallel()
	srv, c, _, code := energyServer(t, playtest.Cost)
	openPack(t, c, srv.URL, code)

	for _, path := range []string{"/", "/pack", "/collection"} {
		_, body := get(t, c, srv.URL+path)

		if value, max := gauge(t, body); value != 10 || max != 10 {
			t.Errorf("%s: energy meter = %d of %d, want 10 of 10", path, value, max)
		}
		if !strings.Contains(body, "Energy") || !strings.Contains(body, "10 / 10") {
			t.Errorf("%s: the gauge does not say 10 / 10 in words:\n%s", path, body)
		}
	}
}

func TestEnergy_theWelcomeBoostersCostNothingAndTheNextOnesDo(t *testing.T) {
	t.Parallel()
	srv, c, _, code := energyServer(t, playtest.Cost)

	spendWelcome(t, c, srv.URL, code)
	_, afterWelcome := get(t, c, srv.URL+"/")
	openPack(t, c, srv.URL, code)
	_, afterOne := get(t, c, srv.URL+"/")

	if value, _ := gauge(t, afterWelcome); value != 10 {
		t.Errorf("energy = %d after the welcome boosters, want 10: they are free", value)
	}
	if value, _ := gauge(t, afterOne); value != 10-playtest.Cost {
		t.Errorf("energy = %d after one paid booster, want %d", value, 10-playtest.Cost)
	}
}

func TestHome_aTileSaysItIsAFreeGiftThenWhatABoosterCosts(t *testing.T) {
	t.Parallel()
	srv, c, _, code := energyServer(t, playtest.Cost)

	_, welcome := get(t, c, srv.URL+"/")
	spendWelcome(t, c, srv.URL, code)
	_, paid := get(t, c, srv.URL+"/")

	if !strings.Contains(welcome, "10 boosters") || !strings.Contains(welcome, "Welcome gift") {
		t.Errorf("a tile with welcome boosters must count them and say they are a free gift:\n%s", welcome)
	}
	if !regexp.MustCompile(`Open · <svg class="energy-icon"[^>]*>.*</svg>5</button>`).MatchString(strings.ReplaceAll(paid, "\n", "")) || strings.Contains(paid, " disabled") {
		t.Errorf("once they are gone the button must say its price with the energy icon, and be enabled while the energy is there:\n%s", paid)
	}
	if !strings.Contains(paid, `aria-label="Open a `) || !strings.Contains(paid, ", 5 energy\"") {
		t.Errorf("the icon says nothing to a screen reader, so the button's name must spell the price out:\n%s", paid)
	}
	if !strings.Contains(welcome, "Open · free</button>") {
		t.Errorf("while welcome boosters are left the button must say it is free:\n%s", welcome)
	}
	if strings.Contains(paid, "Welcome gift") {
		t.Errorf("the free-gift line is still there with no welcome booster left:\n%s", paid)
	}
}

func TestHome_aTileIsDisabledAndSaysHowLongToWaitWhenEnergyIsShort(t *testing.T) {
	t.Parallel()
	srv, c, clock, code := energyServer(t, playtest.Cost)
	spendWelcome(t, c, srv.URL, code)
	openPack(t, c, srv.URL, code)
	openPack(t, c, srv.URL, code)

	_, empty := get(t, c, srv.URL+"/")
	clock.Advance(20 * time.Minute)
	_, later := get(t, c, srv.URL+"/")

	if !strings.Contains(empty, " disabled") || !strings.Contains(empty, "Ready in 50 min") {
		t.Errorf("with 0 energy the button must be disabled and the wait 50 min:\n%s", empty)
	}
	if !strings.Contains(later, "Ready in 30 min") {
		t.Errorf("20 minutes later the wait must be 30 min:\n%s", later)
	}
}

func TestOpen_withTooLittleEnergyIsAConflictThatExplainsTheWait(t *testing.T) {
	t.Parallel()
	srv, c, _, code := energyServer(t, playtest.Cost)
	spendWelcome(t, c, srv.URL, code)
	openPack(t, c, srv.URL, code)
	openPack(t, c, srv.URL, code)

	resp, err := c.PostForm(srv.URL+"/open", url.Values{"set": {code}})
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}
	for _, want := range []string{"Not enough energy", "costs 5", "50 min"} {
		if !strings.Contains(body, want) {
			t.Errorf("the message lacks %q:\n%s", want, body)
		}
	}
	_, home := get(t, c, srv.URL+"/")
	if value, _ := gauge(t, home); value != 0 {
		t.Errorf("energy = %d, want 0: a refused opening must not spend any", value)
	}
}

func TestPack_openAnotherIsDisabledWhenNoEnergyIsLeft(t *testing.T) {
	t.Parallel()
	srv, c, _, code := energyServer(t, playtest.Cost)
	spendWelcome(t, c, srv.URL, code)
	openPack(t, c, srv.URL, code)
	openPack(t, c, srv.URL, code)

	_, body := get(t, c, srv.URL+"/pack")

	_, actions, found := strings.Cut(body, `class="actions"`)
	if !found {
		t.Fatalf("no actions on the pack page:\n%s", body)
	}
	if !strings.Contains(actions, "disabled") || !strings.Contains(actions, "Ready in 50 min") {
		t.Errorf("Open another must be disabled with the wait said:\n%s", actions)
	}
}

func TestEnergy_theGaugeShowsTheSavingsAndWhenTheNextPointComes(t *testing.T) {
	t.Parallel()
	srv, c, clock, code := energyServer(t, playtest.Cost)
	spendWelcome(t, c, srv.URL, code)

	// With the reserve full, a savings point comes every three ticks: seven ticks give two, and the
	// next is two ticks, 20 minutes, away.
	clock.Advance(7 * playtest.Tick)
	_, body := get(t, c, srv.URL+"/")

	if !strings.Contains(body, "+2 saved") {
		t.Errorf("the savings are not shown:\n%s", body)
	}
	if !strings.Contains(body, "+1 in 20 min") {
		t.Errorf("the next point is not announced:\n%s", body)
	}
}

func TestEnergy_durationsAreSaidInHoursAndMinutesAndNeverAsZero(t *testing.T) {
	t.Parallel()
	// A booster of 10 empties the reserve, and the next is ten ticks, 100 minutes, away.
	srv, c, clock, code := energyServer(t, 10)
	spendWelcome(t, c, srv.URL, code)
	openPack(t, c, srv.URL, code)

	_, long := get(t, c, srv.URL+"/")
	// Nine and a half minutes into a tick, the next point is half a minute away.
	clock.Advance(9*time.Minute + 30*time.Second)
	_, short := get(t, c, srv.URL+"/")

	if !strings.Contains(long, "Ready in 1 h 40 min") {
		t.Errorf("100 minutes must read 1 h 40 min:\n%s", long)
	}
	if !strings.Contains(short, "+1 in less than a minute") {
		t.Errorf("30 seconds must read less than a minute, not 0 min:\n%s", short)
	}
}
