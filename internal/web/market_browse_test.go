package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// buyNowForm is the form of a Buy now button of the card page: the card, the NPC and the filter the
// player came with.
func buyNowForm(card string, npc int) url.Values {
	return url.Values{"card": {card}, "finish": {"nonfoil"}, "npc": {strconv.Itoa(npc)}}
}

func offerForm(card string, npc int, coins string) url.Values {
	f := buyNowForm(card, npc)
	f.Set("coins", coins)
	return f
}

const cardPage = "/market/card?card=S-00&finish=nonfoil"

// The market page is a shop window: every card an NPC holds, a tile each, with the lowest price and
// how many NPCs hold it, and tabs to the trades the player started.
func TestMarketBrowse_isTheLandingPageWithATilePerCardAndTabs(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	got := shop.get(t, "/market")

	mustContain(t, "Browse", got,
		"<h1>Market</h1>", `aria-current="page">Browse`, `href="/market/trades"`, "My trades",
		`href="/market/card?card=S-00&amp;finish=nonfoil"`, "S-00", "From ◎100", "2 NPCs",
		`href="/market/card?card=S-01&amp;finish=nonfoil"`, "From ◎250")
	mustNotContain(t, "Browse", got, "Asking around", "Your listings")
}

func TestMarketBrowse_filtersByWhatTheCardIs(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	if err := shop.game.Hunt(domain.CardID("S-01"), domain.FinishNonFoil); err != nil {
		t.Fatalf("Hunt: %v", err)
	}
	if _, err := shop.game.BuyNow(fastNPC, "S-00", domain.FinishNonFoil); err != nil {
		t.Fatalf("BuyNow: %v", err)
	}
	tests := []struct {
		name    string
		query   string
		want    []string
		notWant []string
	}{
		{name: "hunted", query: "hunted=1", want: []string{"S-01"}, notWant: []string{"S-00", "S-02"}},
		{name: "missing", query: "missing=1", want: []string{"S-01", "S-02"}, notWant: []string{"S-00"}},
		{name: "rarity", query: "rarity=uncommon", want: []string{"S-02"}, notWant: []string{"S-00", "S-01"}},
		{name: "colour", query: "colour=colourless", want: []string{"S-00", "S-01"}},
		{name: "colour with none", query: "colour=R", notWant: []string{"S-00", "S-01"}},
		{name: "set", query: "set=S", want: []string{"S-00"}},
		{name: "name", query: "q=s-01", want: []string{"S-01"}, notWant: []string{"S-00"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shop.get(t, "/market?"+tc.query)
			mustContain(t, tc.query, got, tc.want...)
			for _, bad := range tc.notWant {
				mustNotContain(t, tc.query, got, `card=`+bad+`&amp;finish`)
			}
		})
	}
}

func TestMarketBrowse_saysWhyNothingShows(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	got := shop.get(t, "/market?q=zzz")

	mustContain(t, "an empty filter", got, "No card matches", `href="/market"`)
}

func TestMarketBrowse_aPagePastTheLastShowsTheLastOne(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	got := shop.get(t, "/market?page=9")

	mustContain(t, "a page past the last", got, `card=S-00&amp;finish`)
	mustNotContain(t, "a page past the last", got, "Previous page", "Next page")
}

func TestMarketBrowse_aGameWithoutAMarketHasNoPage(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	resp, body := get(t, shop.c, shop.srv.URL+"/market")

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /market = %d, want 404:\n%s", resp.StatusCode, body)
	}
}

// A card's page lists each NPC that holds it with its price, what the player pays at once, and a
// form to offer less.
func TestMarketCard_listsEachNPCWithItsPriceAndTheTwoWaysToBuy(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	got := shop.get(t, cardPage)

	mustContain(t, "the card page", got,
		"<h1>S-00</h1>", "Karn Liberated (a fast)", "◎100", "Karn, the Great Creator (b slow)", "◎110",
		`action="/buy-now"`, `name="npc" value="0"`, `name="npc" value="1"`, "Buy now", `action="/offer"`, `name="coins"`,
		`href="/market"`)
	mustNotContain(t, "the card page", got, "Counter")
}

func TestMarketCard_saysWhenNobodySellsTheCard(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	got := shop.get(t, "/market/card?card=S-03&finish=foil")

	mustContain(t, "a card no NPC holds", got, "No NPC sells")
}

func TestMarketCard_refusesWhatIsNotACard(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	resp, body := get(t, shop.c, shop.srv.URL+"/market/card?card=nope&finish=nonfoil")

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("GET = %d, want 400:\n%s", resp.StatusCode, body)
	}
}

func TestBuyNow_buysAtTheAskingPriceAndComesBackToTheCardPage(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	resp, body := shop.post(t, "/buy-now", buyNowForm("S-00", slowNPC))

	redirectsTo(t, "POST /buy-now", resp, cardPage)
	if got := shop.game.Balance(); got != 90 {
		t.Errorf("balance = %d, want 90 after paying the slow NPC's 110: %s", got, body)
	}
	got := shop.get(t, cardPage)
	mustContain(t, "the card page after the purchase", got, "Bought S-00 from Karn, the Great Creator (b slow) for ◎110")
}

func TestBuyNow_refusalsKeepThePlayerOnTheCardWithAMessage(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	tests := []struct {
		name   string
		form   url.Values
		status int
		want   string
	}{
		{name: "not enough coins", form: buyNowForm("S-01", fastNPC), status: http.StatusConflict, want: "Not enough coins"},
		{name: "the NPC has no copy", form: buyNowForm("S-00", specNPC), status: http.StatusConflict, want: "no longer has"},
		{name: "an unknown card", form: buyNowForm("nope", fastNPC), status: http.StatusBadRequest, want: "Unknown card"},
		{name: "a card with no finish", form: url.Values{"card": {"S-00"}, "npc": {"0"}}, status: http.StatusBadRequest, want: "Unknown card"},
		{name: "an NPC that is not a number", form: url.Values{"card": {"S-00"}, "finish": {"nonfoil"}, "npc": {"x"}}, status: http.StatusBadRequest, want: "Unknown NPC"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := shop.post(t, "/buy-now", tc.form)
			if resp.StatusCode != tc.status {
				t.Errorf("POST /buy-now = %d, want %d:\n%s", resp.StatusCode, tc.status, body)
			}
			mustContain(t, "the refusal", body, tc.want)
		})
	}
	if got := shop.game.Balance(); got != 200 {
		t.Errorf("balance = %d, want 200: a refusal changes nothing", got)
	}
}

func TestOffer_startsATradeThatWaitsOnTheNPC(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	resp, body := shop.post(t, "/offer", offerForm("S-00", slowNPC, "105"))

	redirectsTo(t, "POST /offer", resp, cardPage)
	v := shop.market(t)
	if len(v.Requests) != 1 || !v.Requests[0].Direct || v.Requests[0].Stage != play.StageWaiting {
		t.Fatalf("requests = %+v, want one offer that waits on the NPC: %s", v.Requests, body)
	}
	got := shop.get(t, cardPage)
	mustContain(t, "the card page after the offer", got, "Offered ◎105 for S-00 to Karn, the Great Creator (b slow)")
	trades := shop.get(t, "/market/trades")
	mustContain(t, "My trades", trades, "Waiting on an NPC", "replies in about 6 h")
}

func TestOffer_refusalsKeepThePlayerOnTheCardWithAMessage(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	tests := []struct {
		name   string
		form   url.Values
		status int
		want   string
	}{
		{name: "not below the price", form: offerForm("S-00", slowNPC, "110"), status: http.StatusBadRequest, want: "below the asking price"},
		{name: "no number", form: offerForm("S-00", slowNPC, "cheap"), status: http.StatusBadRequest, want: "whole coins"},
		{name: "no coin", form: offerForm("S-00", slowNPC, "0"), status: http.StatusBadRequest, want: "at least 1 coin"},
		{name: "the NPC has no copy", form: offerForm("S-00", specNPC, "50"), status: http.StatusConflict, want: "no longer has"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := shop.post(t, "/offer", tc.form)
			if resp.StatusCode != tc.status {
				t.Errorf("POST /offer = %d, want %d:\n%s", resp.StatusCode, tc.status, body)
			}
			mustContain(t, "the refusal", body, tc.want)
		})
	}
	if v := shop.market(t); len(v.Requests) != 0 {
		t.Errorf("requests = %+v, want none: a refusal changes nothing", v.Requests)
	}
}

// The filter the player browsed with comes back with them from a card's page and from a purchase.
func TestMarketCard_carriesTheFilterBackToBrowse(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	got := shop.get(t, cardPage+"&rarity=common&missing=1")

	mustContain(t, "the card page", got, `href="/market?missing=1&amp;rarity=common"`, `name="rarity" value="common"`, `name="missing" value="1"`)
	form := buyNowForm("S-00", fastNPC)
	form.Set("rarity", "common")
	resp, _ := shop.post(t, "/buy-now", form)
	redirectsTo(t, "POST /buy-now with a filter", resp, cardPage+"&rarity=common")
}
