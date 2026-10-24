package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

const showPanelLabel = "Show cards"

// newNobodyTradesShop serves the market of marketRules with only the speculator, who opens nothing:
// no NPC has a card to trade.
func newNobodyTradesShop(t *testing.T) counterShop {
	t.Helper()
	rules := marketRules(0)
	rules.Crowd.Archetypes = map[string]domain.ArchetypeSpec{"c_spec": rules.Crowd.Archetypes["c_spec"]}
	return newShopWith(t, nil, rules, counterSet())
}

func TestCollection_theShowListOffersOnlyWhatTheGameCanDo(t *testing.T) {
	t.Parallel()
	counterOnly := newCounterShop(t)
	market := newMarketShop(t)
	srv, c := newServer(t, counterSet())

	_, bare := get(t, c, srv.URL+"/collection")

	tests := []struct {
		name string
		body string
		want []string
		not  []string
	}{
		{name: "a game with a Counter", body: counterOnly.get(t, "/collection?set=S"), want: []string{"All cards", "Duplicates", "At the Counter"}, not: []string{"Available to trade"}},
		{name: "a game with a market", body: market.get(t, "/collection?set=S"), want: []string{"All cards", "Duplicates", "At the Counter", "Available to trade"}},
		{name: "a game with neither", body: bare, want: []string{"All cards", "Duplicates"}, not: []string{"At the Counter", "Available to trade"}},
	}
	for _, tt := range tests {
		got := panel(tt.body, showPanelLabel)
		mustContain(t, tt.name, got, tt.want...)
		mustNotContain(t, tt.name, got, tt.not...)
	}
}

func TestCollection_showCounterKeepsTheCardsTheCounterHasInStock(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	body := shop.get(t, "/collection?set=S&show=counter")

	if got := slotsShown(body); got != 2 {
		t.Errorf("%d slots shown, want the 2 commons the Counter stocks", got)
	}
	mustContain(t, "the binder", body, `id="c-`+cheapCard+`"`, `id="c-`+dearCard+`"`)
	mustNotContain(t, "the binder", body, `id="c-`+neverCard+`"`, `id="c-`+unpricedKey+`"`)
	mustContain(t, "the show list", panel(body, showPanelLabel), `aria-current="true">At the Counter`)
}

func TestCollection_showCounterKeepsAHeldCardTheCounterStillHas(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 1)

	body := shop.get(t, "/collection?set=S&show=counter")

	mustContain(t, "the binder", slot(t, body, cheapCard), `action="/buy"`, `action="/sell"`)
}

func TestCollection_showDuplicatesKeepsTheCardsAboveAPlayset(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 5)
	if _, err := shop.game.Sell(domain.CardID(cheapCard), domain.FinishNonFoil, 1); err != nil {
		t.Fatalf("Sell: %v", err)
	}
	if _, err := shop.game.Sell(domain.CardID(dearCard), domain.FinishNonFoil, 1); err != nil {
		t.Fatalf("Sell: %v", err)
	}

	body := shop.get(t, "/collection?set=S&show=duplicates")

	// Five copies of every card, less one of two of them: a playset of four is not a duplicate.
	mustContain(t, "the binder", body, `id="c-`+neverCard+`"`, `id="c-`+unpricedKey+`"`)
	mustNotContain(t, "the binder", body, `id="c-`+cheapCard+`"`, `id="c-`+dearCard+`"`)
	if got := slotsShown(body); got != 2 {
		t.Errorf("%d slots shown, want the 2 cards held above four", got)
	}
}

func TestCollection_showTradeKeepsTheCardsAnNPCHolds(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	body := shop.get(t, "/collection?set=S&show=trade")

	mustContain(t, "the binder", body, `id="c-`+cheapCard+`"`)
	mustContain(t, "the show list", panel(body, showPanelLabel), `aria-current="true">Available to trade`)
	mustNotContain(t, "the binder", body, "No card matches")
}

func TestCollection_showTradeIsEmptyWhenNoNPCHasACard(t *testing.T) {
	t.Parallel()
	shop := newNobodyTradesShop(t)

	body := shop.get(t, "/collection?set=S&show=trade")

	if got := slotsShown(body); got != 0 {
		t.Errorf("%d slots shown, want none", got)
	}
	mustContain(t, "the empty binder", body, "No card matches this filter.")
}

func TestCollection_showListCountsTheCardsOfEachChoiceWithinTheOtherFilters(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	counts := func(path string) string { return panel(shop.get(t, path), showPanelLabel) }

	all := counts("/collection?set=S")
	mustContain(t, "the list of the whole binder", all, `All cards <strong>4</strong>`, `At the Counter <strong>2</strong>`, `Duplicates <strong>0</strong>`)
	uncommons := counts("/collection?set=S&rarity=uncommon")
	mustContain(t, "the list of the uncommons", uncommons, `All cards <strong>1</strong>`, `At the Counter <strong>0</strong>`)
}

func TestCollection_theShowListIsAFilterThatKeepsTheOthersInForce(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	got := panel(shop.get(t, "/collection?set=S&rarity=common&show=counter"), showPanelLabel)

	mustContain(t, "the show list",
		got,
		`href="/collection?rarity=common&amp;set=S"`,                     // At the Counter is in force, so its link clears it
		`href="/collection?rarity=common&amp;set=S&amp;show=duplicates"`) // Duplicates picks another
	if n := strings.Count(got, `href="/collection?rarity=common&amp;set=S"`); n != 2 {
		t.Errorf("%d links clear the show filter, want 2 (All cards and At the Counter):\n%s", n, got)
	}
	rarities := panel(shop.get(t, "/collection?set=S&rarity=common&show=counter"), "Filter by rarity")
	mustContain(t, "the rarity list", rarities, `href="/collection?rarity=uncommon&amp;set=S&amp;show=counter"`)
}

func TestCollection_everyFormOfAShownBinderCarriesTheShowFilter(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	body := shop.get(t, "/collection?set=S&show=counter")

	mustContain(t, "a form of the binder", slot(t, body, cheapCard), `name="show" value="counter"`)
}

func TestBuy_comesBackToTheBinderWithTheShowFilterInForce(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	form := cardForm("S", cheapCard, "nonfoil")
	form.Set("show", "counter")

	resp, _ := shop.post(t, "/buy", form)

	redirectsTo(t, "POST /buy", resp, "/collection?set=S&show=counter#c-S-00")
}

func TestCollection_aShowFilterTheGameDoesNotOfferIsABadRequest(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	srv, c := newServer(t, counterSet())

	for _, path := range []string{"/collection?set=S&show=nonsense", "/collection?set=S&show=trade"} {
		resp, body := get(t, shop.c, shop.srv.URL+path)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", path, resp.StatusCode)
		}
		mustContain(t, path, body, "Unknown filter")
	}
	resp, _ := get(t, c, srv.URL+"/collection?show=counter")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("a game without a Counter: GET show=counter = %d, want 400", resp.StatusCode)
	}
}
