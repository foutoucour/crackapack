package web_test

import (
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const (
	cheapCard   = "S-00" // $1: the Counter asks 120 coins and pays 60, one copy in stock.
	dearCard    = "S-01" // $2.50: asks 300 and pays 150, one copy in stock.
	neverCard   = "S-02" // an uncommon: no copy is stocked at the start, and none ever was.
	unpricedKey = "S-03" // worth nothing: the Counter does not trade it.
)

// counterSet is the set S of four cards that a booster gives whole, so that one opening holds a copy
// of each. The uncommon is the one that the initial stock, which is of commons, leaves out.
func counterSet() domain.SetCatalogue {
	cat := playtest.TinySet("S", 4, 4)
	card := cat.Cards[neverCard]
	card.Rarity = domain.RarityUncommon
	cat.Cards[neverCard] = card
	return cat
}

func counterRules() play.CounterRules {
	return play.CounterRules{
		Params: domain.CounterParams{
			SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1,
			InitialStock: map[domain.Rarity]int{domain.RarityCommon: 1},
			ShelfLife:    domain.VanishAfterDays(1),
		},
		Prices:       domain.PriceSnapshot{cheapCard: 100, dearCard: 250, neverCard: 100, unpricedKey: 0},
		WelcomeBonus: 200,
		CatchUpDays:  30,
	}
}

func newOpener() *domain.Opener {
	return domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()},
		rand.New(rand.NewPCG(testSeed, testSeed)),
	)
}

type counterShop struct {
	srv   *httptest.Server
	c     *http.Client
	game  *play.Game
	clock *playtest.Clock
}

// newCounterShop serves a game with a Counter on counterSet.
func newCounterShop(t *testing.T) counterShop {
	t.Helper()
	return newShop(t, nil, counterSet())
}

// newShop serves a game with a Counter on sets, cut into stages by stage unless it is nil.
func newShop(t *testing.T, stage *play.StageRules, sets ...domain.SetCatalogue) counterShop {
	t.Helper()
	return newShopWith(t, stage, counterRules(), sets...)
}

// newShopWith is newShop under rules.
func newShopWith(t *testing.T, stage *play.StageRules, rules play.CounterRules, sets ...domain.SetCatalogue) counterShop {
	t.Helper()
	clock := playtest.NewClock()
	game, err := play.NewGame(sets, nil, newOpener(), playtest.Rules(sets...), clock.Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if stage != nil {
		if err := game.Stage(*stage, play.NewStream(1)); err != nil {
			t.Fatalf("Stage: %v", err)
		}
	}
	if err := game.OpenCounter(rules, play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	srv, c, _ := serve(t, game, fakeImages{})
	return counterShop{srv: srv, c: c, game: game, clock: clock}
}

func (s counterShop) post(t *testing.T, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	resp, err := s.c.PostForm(s.srv.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp, readBody(t, resp)
}

func (s counterShop) get(t *testing.T, path string) string {
	t.Helper()
	resp, body := get(t, s.c, s.srv.URL+path)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200:\n%s", path, resp.StatusCode, body)
	}
	return body
}

// open opens n boosters of S, which gives a copy of each card per booster.
func (s counterShop) open(t *testing.T, n int) {
	t.Helper()
	for range n {
		if _, err := s.game.Open("S"); err != nil {
			t.Fatalf("Open: %v", err)
		}
		s.clock.Advance(time.Hour)
	}
}

func cardForm(set, card, finish string) url.Values {
	return url.Values{"set": {set}, "card": {card}, "finish": {finish}}
}

func sellForm(card string, qty string) url.Values {
	f := cardForm("S", card, "nonfoil")
	f.Set("qty", qty)
	return f
}

// slot is the part of body about the card id: its entry of the binder, which ends where the next
// entry or the binder does, since the market lists the answers inside it.
func slot(t *testing.T, body, id string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<li class="entry" id="c-` + id + `">.*?</li>\s*(?:<li class="entry"|</ol>)`).FindString(body)
	if m == "" {
		t.Fatalf("the page has no entry for %s:\n%s", id, body)
	}
	return m
}

func mustContain(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s lacks %q:\n%s", what, want, body)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(body, bad) {
			t.Errorf("%s has %q:\n%s", what, bad, body)
		}
	}
}

func TestHeader_showsTheCoinsNextToTheEnergyAndOnlyInAGameWithACounter(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	srv, c := newServer(t, counterSet())

	for _, path := range []string{"/", "/collection"} {
		body := shop.get(t, path)
		mustContain(t, path, body, `class="coins"`, "<strong>200</strong>")
		if coins, energy := strings.Index(body, `class="coins"`), strings.Index(body, `class="energy"`); coins < energy {
			t.Errorf("%s: the coins come before the energy gauge", path)
		}
	}
	_, bare := get(t, c, srv.URL+"/")
	mustNotContain(t, "a game without a Counter", bare, `class="coins"`)
}

func TestCollection_everyStateOfAnEmptySlotAndOfAHeldCard(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	// Open a booster, sell the only copy of the uncommon and wait for the Counter's shelf to let it
	// go: the slot is empty and the Counter had a copy a day ago.
	shop.open(t, 1)
	if _, err := shop.game.Sell(neverCard, domain.FinishNonFoil, 1); err != nil {
		t.Fatalf("Sell: %v", err)
	}
	if _, err := shop.game.Sell(unpricedKey, domain.FinishNonFoil, 1); err == nil {
		t.Fatal("the Counter bought a card worth nothing")
	}
	shop.clock.Advance(48 * time.Hour)

	body := shop.get(t, "/collection?set=S")

	tests := []struct {
		name string
		card string
		want []string
		not  []string
	}{
		{name: "held", card: cheapCard, want: []string{`action="/sell"`, "60 coins"}, not: []string{`action="/buy"`, "stock"}},
		{name: "not traded", card: unpricedKey, want: []string{"Not traded"}, not: []string{`action="/buy"`, `action="/sell"`}},
		{name: "out of stock", card: neverCard, want: []string{"last in stock 1 day ago", "120 coins"}, not: []string{`action="/buy"`, "never seen"}},
	}
	for _, tt := range tests {
		got := slot(t, body, tt.card)
		mustContain(t, tt.name, got, tt.want...)
		mustNotContain(t, tt.name, got, tt.not...)
	}
}

func TestCollection_aCardInStockHasAPriceAndABuyButtonAndNeverACount(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	body := shop.get(t, "/collection?set=S")

	got := slot(t, body, cheapCard)
	mustContain(t, "an empty slot in stock", got, `action="/buy"`, `name="card" value="S-00"`, `name="finish" value="nonfoil"`, "120 coins", ">Buy")
	mustNotContain(t, "an empty slot in stock", got, "last in stock", "never seen")
	if numbers := regexp.MustCompile(`(?i)\d+ (cop(y|ies)|left|in stock|available)`); numbers.MatchString(body) {
		t.Errorf("the page tells a stock: %q", numbers.FindString(body))
	}
}

func TestCollection_aCardNeverInStockSaysSo(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	got := slot(t, shop.get(t, "/collection?set=S"), neverCard)

	mustContain(t, "a never-seen slot", got, "never seen at the Counter", "120 coins")
	mustNotContain(t, "a never-seen slot", got, `action="/buy"`, "last in stock")
}

func TestBuy_comesBackToTheBinderWithBothFiltersInForce(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	resp, _ := shop.post(t, "/buy", func() url.Values {
		f := cardForm("S", cheapCard, "nonfoil")
		f.Set("rarity", "common")
		f.Set("colour", "colourless")
		return f
	}())

	redirectsTo(t, "POST /buy", resp, "/collection?colour=colourless&rarity=common&set=S#c-S-00")
}

func TestCollection_everyFormOfAFilteredBinderCarriesTheFilters(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	body := shop.get(t, "/collection?set=S&rarity=common&colour=colourless")

	mustContain(t, "a form of the filtered binder", slot(t, body, cheapCard), `name="rarity" value="common"`, `name="colour" value="colourless"`)
}

func TestBuy_takesTheCoinsAddsTheCardAndComesBackToItsPlaceInTheBinder(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	resp, _ := shop.post(t, "/buy", func() url.Values { f := cardForm("S", cheapCard, "nonfoil"); f.Set("rarity", "common"); return f }())

	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/collection?rarity=common&set=S#c-S-00" {
		t.Fatalf("POST /buy = %d to %q, want 303 to the card's place in the filtered binder", resp.StatusCode, resp.Header.Get("Location"))
	}
	if shop.game.Balance() != 80 {
		t.Errorf("balance %d, want 80", shop.game.Balance())
	}
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after a purchase", body, "Bought S-00", "for 120 coins", "<strong>80</strong>")
	mustContain(t, "the bought card", slot(t, body, cheapCard), `action="/sell"`)
	if again := shop.get(t, "/collection?set=S"); strings.Contains(again, "Bought") {
		t.Error("the notice of the purchase is shown twice")
	}
}

func TestBuy_aPurchaseThatUnlocksSetsSaysSoAfterTheRedirect(t *testing.T) {
	t.Parallel()
	shop := newShop(t, &play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1))

	shop.post(t, "/buy", cardForm("A", "A-00", "nonfoil"))

	body := shop.get(t, "/collection?set=A")
	mustContain(t, "the binder after the unlocking purchase", body, "Unlocked:", "Set B", `href="/"`)
}

func TestBuy_refusalsShowAMessageAndChangeNothing(t *testing.T) {
	t.Parallel()
	staged := &play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}
	tests := []struct {
		name   string
		stage  *play.StageRules
		sets   []domain.SetCatalogue
		before func(*testing.T, counterShop)
		form   url.Values
		status int
		want   string
	}{
		{name: "not enough coins", sets: []domain.SetCatalogue{counterSet()}, before: func(t *testing.T, s counterShop) {
			if _, err := s.game.Buy(cheapCard, domain.FinishNonFoil); err != nil {
				t.Fatalf("Buy: %v", err)
			}
		}, form: cardForm("S", dearCard, "nonfoil"), status: http.StatusConflict, want: "Not enough coins: this card costs 300 and you have 80"},
		{name: "out of stock", sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", neverCard, "nonfoil"), status: http.StatusConflict, want: "no copy of this card"},
		{name: "not traded", sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", unpricedKey, "nonfoil"), status: http.StatusConflict, want: "does not trade this card"},
		{name: "a locked set", stage: staged, sets: []domain.SetCatalogue{playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1)}, form: cardForm("B", "B-00", "nonfoil"), status: http.StatusConflict, want: "locked set"},
		{name: "an unknown card", sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", "nope", "nonfoil"), status: http.StatusBadRequest, want: "Unknown card"},
		{name: "an unknown finish", sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", cheapCard, "gilded"), status: http.StatusBadRequest, want: "Unknown card"},
		{name: "no card", sets: []domain.SetCatalogue{counterSet()}, form: url.Values{"set": {"S"}}, status: http.StatusBadRequest, want: "Unknown card"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shop := newShop(t, tt.stage, tt.sets...)
			if tt.before != nil {
				tt.before(t, shop)
			}
			coins := shop.game.Balance()

			resp, body := shop.post(t, "/buy", tt.form)

			if resp.StatusCode != tt.status {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.status)
			}
			mustContain(t, "the refusal", body, tt.want, `role="alert"`)
			if shop.game.Balance() != coins {
				t.Errorf("a refused purchase moved the balance from %d to %d", coins, shop.game.Balance())
			}
		})
	}
}

func TestBuy_theRefusalLinksBackToTheBinder(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	_, body := shop.post(t, "/buy", cardForm("S", neverCard, "nonfoil"))

	mustContain(t, "the refusal", body, `href="/collection?set=S#c-S-02"`, "Back to the binder")
}

func TestSell_oneOfSeveralCopiesPaysAndComesBackToItsPlace(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 2)
	mustContain(t, "the quantity of a held card for a screen reader", slot(t, shop.get(t, "/collection?set=S"), cheapCard),
		`Quantity<span class="sr-only"> of S-00</span>`)

	resp, _ := shop.post(t, "/sell", sellForm(cheapCard, "1"))

	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/collection?set=S#c-S-00" {
		t.Fatalf("POST /sell = %d to %q, want 303 to the card's place", resp.StatusCode, resp.Header.Get("Location"))
	}
	if shop.game.Balance() != 260 {
		t.Errorf("balance %d, want 260", shop.game.Balance())
	}
	mustContain(t, "the binder after a sale", shop.get(t, "/collection?set=S"), "Sold 1 ", "S-00", "for 60 coins")
}

func TestSell_theLastCopyAsksBeforeSellingAndCancellingChangesNothing(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 1)

	resp, body := shop.post(t, "/sell", sellForm(cheapCard, "1"))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /sell of the last copy = %d, want the confirmation page (200)", resp.StatusCode)
	}
	mustContain(t, "the confirmation", body, "last copy", "S-00", "60 coins", `name="confirm" value="yes"`, `action="/sell"`, `href="/collection?set=S#c-S-00"`, "Keep")
	if shop.game.Balance() != 200 {
		t.Errorf("asking sold the card: balance %d, want 200", shop.game.Balance())
	}

	confirmed := sellForm(cheapCard, "1")
	confirmed.Set("confirm", "yes")
	resp, _ = shop.post(t, "/sell", confirmed)

	if resp.StatusCode != http.StatusSeeOther || shop.game.Balance() != 260 {
		t.Errorf("confirmed sale = %d with balance %d, want 303 and 260", resp.StatusCode, shop.game.Balance())
	}
	mustContain(t, "the emptied slot", slot(t, shop.get(t, "/collection?set=S"), cheapCard), `action="/buy"`)
}

func TestSell_allTheCopiesAsksToo(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 3)

	_, body := shop.post(t, "/sell", sellForm(dearCard, "3"))

	mustContain(t, "the confirmation", body, "all 3 copies", "450 coins")
}

func TestSell_refusalsShowAMessageAndChangeNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		form   url.Values
		status int
		want   string
	}{
		{name: "more than held", form: sellForm(cheapCard, "3"), status: http.StatusConflict, want: "do not hold that many"},
		{name: "a finish the card lacks", form: func() url.Values { f := sellForm(cheapCard, "1"); f.Set("finish", "foil"); return f }(), status: http.StatusBadRequest, want: "Unknown card"},
		{name: "a quantity of zero", form: sellForm(cheapCard, "0"), status: http.StatusBadRequest, want: "quantity"},
		{name: "a quantity that is not a number", form: sellForm(cheapCard, "many"), status: http.StatusBadRequest, want: "quantity"},
		{name: "no quantity", form: cardForm("S", cheapCard, "nonfoil"), status: http.StatusBadRequest, want: "quantity"},
		{name: "an unknown card", form: sellForm("nope", "1"), status: http.StatusBadRequest, want: "Unknown card"},
		{name: "a card the Counter does not trade", form: sellForm(unpricedKey, "1"), status: http.StatusConflict, want: "does not trade this card"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shop := newCounterShop(t)
			shop.open(t, 2)
			coins := shop.game.Balance()

			resp, body := shop.post(t, "/sell", tt.form)

			if resp.StatusCode != tt.status {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.status)
			}
			mustContain(t, "the refusal", body, tt.want, `role="alert"`)
			if shop.game.Balance() != coins {
				t.Errorf("a refused sale moved the balance from %d to %d", coins, shop.game.Balance())
			}
		})
	}
}

func TestSellDuplicates_aDuplicateTheCounterDoesNotTradeHidesTheOffer(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 6)

	body := shop.get(t, "/collection?set=S")

	mustNotContain(t, "a binder whose duplicates include an untraded card", body, `action="/sell-duplicates"`)
}

func TestSellDuplicates_pricesAndSellsTheSurplus(t *testing.T) {
	t.Parallel()
	cat := playtest.TinySet("T", 2, 2)
	shop := newShop(t, nil, cat)
	for range 6 {
		if _, err := shop.game.Open("T"); err != nil {
			t.Fatalf("Open: %v", err)
		}
		shop.clock.Advance(time.Hour)
	}

	body := shop.get(t, "/collection?set=T")

	// T-00 and T-01 are worth the fallback of one coin: 2 copies of each above four.
	mustContain(t, "the duplicates section", body, "4 copies above a playset", "4 coins", `action="/sell-duplicates"`, `name="set" value="T"`)
	resp, _ := shop.post(t, "/sell-duplicates", url.Values{"set": {"T"}})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/collection?set=T" {
		t.Fatalf("POST /sell-duplicates = %d to %q, want 303 to the binder", resp.StatusCode, resp.Header.Get("Location"))
	}
	if shop.game.Balance() != 204 {
		t.Errorf("balance %d, want 204", shop.game.Balance())
	}
	after := shop.get(t, "/collection?set=T")
	mustContain(t, "the binder after the sale", after, "Sold your duplicates for 4 coins")
	mustNotContain(t, "the binder after the sale", after, `action="/sell-duplicates"`)
	mustContain(t, "the binder after the sale", after, "no duplicates")
}

func TestSellDuplicates_refusals(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 1)

	tests := []struct {
		name   string
		set    string
		status int
		want   string
	}{
		{name: "nothing above a playset", set: "S", status: http.StatusConflict, want: "no duplicates"},
		{name: "an unknown set", set: "NOPE", status: http.StatusBadRequest, want: "Unknown set"},
	}
	for _, tt := range tests {
		resp, body := shop.post(t, "/sell-duplicates", url.Values{"set": {tt.set}})
		if resp.StatusCode != tt.status {
			t.Errorf("%s: status %d, want %d", tt.name, resp.StatusCode, tt.status)
		}
		mustContain(t, tt.name, body, tt.want)
	}
	if shop.game.Balance() != 200 {
		t.Errorf("a refused sale moved the balance to %d", shop.game.Balance())
	}
}

func TestCounter_aLockedBinderCannotBuyButStillSells(t *testing.T) {
	t.Parallel()
	shop := newShop(t, &play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1))

	body := shop.get(t, "/collection?set=B")

	mustContain(t, "the locked binder", body, "This set is locked", "cannot be bought")
	mustNotContain(t, "the locked binder", body, `action="/buy"`)
}

func TestCounter_aGameWithoutACounterHasNoControlsAndRefusesTheActions(t *testing.T) {
	t.Parallel()
	srv, c := newServer(t, counterSet())
	openPack(t, c, srv.URL, "S")

	_, body := get(t, c, srv.URL+"/collection")
	mustNotContain(t, "a binder without a Counter", body, `action="/buy"`, `action="/sell"`, `action="/sell-duplicates"`, "never seen")

	for _, path := range []string{"/buy", "/sell", "/sell-duplicates"} {
		resp, err := c.PostForm(srv.URL+path, sellForm(cheapCard, "1"))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		if got := readBody(t, resp); resp.StatusCode != http.StatusNotFound || !strings.Contains(got, "no Counter") {
			t.Errorf("POST %s without a Counter = %d %q, want 404 and a message", path, resp.StatusCode, got)
		}
	}
}

func TestCounter_cardNamesAreEscaped(t *testing.T) {
	t.Parallel()
	cat := counterSet()
	card := cat.Cards[cheapCard]
	card.Name = `<script>alert("x")</script>`
	cat.Cards[cheapCard] = card
	shop := newShop(t, nil, cat)

	empty := shop.get(t, "/collection?set=S")
	shop.open(t, 1)
	held := shop.get(t, "/collection?set=S")
	_, confirm := shop.post(t, "/sell", sellForm(cheapCard, "1"))

	for name, body := range map[string]string{"an empty slot": empty, "a held card": held, "the confirmation": confirm} {
		mustNotContain(t, name, body, `<script>alert`)
	}
	mustContain(t, "the confirmation", confirm, "&lt;script&gt;")
}
