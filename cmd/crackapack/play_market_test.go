package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The demo lets a player use both sides of the market in a minute: asking around for a card of the
// open set gets an answer at once and another hours later, and the rare of a booster just opened,
// listed, draws an offer at once, which sells it.
func TestStartGame_theDemoMarketAnswersAtOnceAndLater(t *testing.T) {
	t.Parallel()
	a := demoGameArgs(t)
	clock := playtest.NewClock()
	a.now = clock.Now
	game, _, err := startGame(a, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}

	request := askUntilAFastAndASlowNPCHaveIt(t, game)
	clock.Advance(24 * time.Hour)
	v, err := game.Market()
	if err != nil {
		t.Fatalf("Market: %v", err)
	}
	if got := v.Requests[0]; got.ID != request.ID || got.Pending != 0 || len(got.Answers) <= len(request.Answers) {
		t.Fatalf("a day later, request = %+v, want the slow NPCs' answers in", got)
	}
	cheapest := cheapestAnswer(v.Requests[0].Answers)
	if _, err := game.AcceptAnswer(cheapest.ID); err != nil {
		t.Fatalf("AcceptAnswer(%+v), balance %d: %v", cheapest, game.Balance(), err)
	}

	opening, err := game.Open("DEM")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// The rare comes last, and is worth an offer.
	card := opening.Cards[len(opening.Cards)-1]
	if _, err := game.List(card.Card.ID, card.Finish, 1000); err != nil {
		t.Fatalf("List: %v", err)
	}
	if v, err = game.Market(); err != nil || len(v.Listings) != 1 || len(v.Listings[0].Answers) == 0 {
		t.Fatalf("listings = %+v, %v, want an offer at once", v.Listings, err)
	}
	before := game.Balance()
	offer := v.Listings[0].Answers[0]
	if _, err := game.AcceptAnswer(offer.ID); err != nil {
		t.Fatalf("AcceptAnswer(%+v): %v", offer, err)
	}
	if game.Balance() != before+offer.Coins {
		t.Errorf("balance = %d, want %d + the offer's %d", game.Balance(), before, offer.Coins)
	}
}

// askUntilAFastAndASlowNPCHaveIt asks around for the cards of DEM in turn until one gets an answer
// at once and leaves an NPC still to answer, and returns that request.
func askUntilAFastAndASlowNPCHaveIt(t *testing.T, game *play.Game) play.Request {
	t.Helper()
	for i := range 80 {
		id := domain.CardID(fmt.Sprintf("dem-c-%03d", i))
		rid, err := game.AskAround(id, domain.FinishNonFoil)
		if errors.Is(err, play.ErrNobodyHasIt) {
			continue
		}
		if err != nil {
			t.Fatalf("AskAround(%s): %v", id, err)
		}
		v, err := game.Market()
		if err != nil {
			t.Fatalf("Market: %v", err)
		}
		for _, r := range v.Requests {
			if r.ID == rid && len(r.Answers) > 0 && r.Pending > 0 {
				return r
			}
		}
		if err := game.Withdraw(rid); err != nil {
			t.Fatalf("Withdraw: %v", err)
		}
	}
	t.Fatal("no common of DEM is held by an NPC that answers at once and by one that answers later")
	return play.Request{}
}

func cheapestAnswer(answers []play.Answer) play.Answer {
	cheapest := answers[0]
	for _, a := range answers[1:] {
		if a.Coins < cheapest.Coins {
			cheapest = a
		}
	}
	return cheapest
}

// A player can use the whole market through the demo's pages: ask around and get an answer at once
// and another later, accept one, counter one, list a card and get an offer, accept it, unlist, and see
// the answers waiting in the header.
func TestRunPlay_theDemoMarketWorksThroughThePages(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	a := demoPlayArgs(t)
	a.now = clock.Now
	base := servePlay(t, a)

	binder := getBody(t, base+"/collection?set=DEM")
	pageHas(t, "a new demo binder", binder, `href="/market"`, `action="/ask-around"`)
	pageHasNot(t, "a new demo binder", binder, `class="badge"`)

	// Hunt: some common is held by an NPC that answers at once and by one that answers later.
	hunted, next, asked := askAroundUntil(t, base, 0, "still to answer")
	pageHas(t, "the slot just asked around", asked, "NPCs asked", "asks ", `action="/accept"`)
	pageHas(t, "the header with an answer waiting", getBody(t, base+"/market"), `class="badge"`)
	clock.Advance(24 * time.Hour)
	later := entryOf(t, getBody(t, base+"/collection?set=DEM"), hunted)
	pageHas(t, "the slot a day later", later, "all have answered")
	pageHasNot(t, "the slot a day later", later, "still to answer")

	// Accept the cheapest answer.
	bought := postBody(t, base, "/accept", url.Values{"answer": {cheapestAnswerID(t, later)}, "set": {"DEM"}, "card": {hunted}})
	pageHas(t, "the binder after accepting", bought, "Bought ", `<strong>`)
	pageHasNot(t, "the bought slot", entryOf(t, bought, hunted), "NPCs asked")

	// Counter an answer to another card.
	other, _, otherAsked := askAroundUntil(t, base, next, "asks ")
	countered := postBody(t, base, "/counter-offer", url.Values{"answer": {cheapestAnswerID(t, otherAsked)}, "set": {"DEM"}, "card": {other}, "coins": {"1"}})
	pageHas(t, "the binder after countering", countered, "Countered the answer of")
	clock.Advance(24 * time.Hour)
	if market := getBody(t, base+"/market"); !regexp.MustCompile(`refused your counter|Bought `).MatchString(market) {
		t.Errorf("a day after the counter the market tells neither a deal nor a refusal:\n%s", market)
	}

	// Sell: the rare of a booster, listed, draws an offer at once, which sells it.
	postBody(t, base, "/open", url.Values{"set": {"DEM"}})
	rares := getBody(t, base+"/collection?set=DEM&rarity=rare")
	rare := firstMatch(t, regexp.MustCompile(`action="/list" class="list">\s*<input type="hidden" name="set" value="DEM"><input type="hidden" name="card" value="([^"]+)"`), rares)
	listed := postBody(t, base, "/list", url.Values{"set": {"DEM"}, "card": {rare}, "finish": {"nonfoil"}, "price": {"1000"}, "rarity": {"rare"}})
	pageHas(t, "the binder after listing", listed, "Listed ", "Listed at 1000 coins", "offers ", `class="badge"`)
	sold := postBody(t, base, "/accept", url.Values{"answer": {cheapestAnswerID(t, entryOf(t, listed, rare))}, "set": {"DEM"}, "card": {rare}, "rarity": {"rare"}})
	pageHas(t, "the binder after the sale", sold, "Sold ")
	pageHas(t, "the slot of the rare after the sale", entryOf(t, sold, rare), "slot empty")

	// Unlist a card that stays held.
	held := getBody(t, base+"/collection?set=DEM&rarity=common")
	common := firstMatch(t, regexp.MustCompile(`action="/list" class="list">\s*<input type="hidden" name="set" value="DEM"><input type="hidden" name="card" value="([^"]+)"`), held)
	listedCommon := postBody(t, base, "/list", url.Values{"set": {"DEM"}, "card": {common}, "finish": {"nonfoil"}, "price": {"5"}, "rarity": {"common"}})
	id := firstMatch(t, regexp.MustCompile(`action="/unlist">\s*<input type="hidden" name="listing" value="(\d+)">`), entryOf(t, listedCommon, common))
	unlisted := postBody(t, base, "/unlist", url.Values{"listing": {id}, "set": {"DEM"}, "card": {common}, "rarity": {"common"}})
	pageHas(t, "the binder after unlisting", unlisted, "off the market")
	pageHas(t, "the unlisted slot", entryOf(t, unlisted, common), `action="/list"`)
}

// askAroundUntil asks around for the commons of DEM from index from on, one by one, until the page
// of the slot says want, and returns the card, the index after it and the page. The requests that
// did not do are withdrawn.
func askAroundUntil(t *testing.T, base string, from int, want string) (card string, next int, page string) {
	t.Helper()
	for i := from; i < 80; i++ {
		id := fmt.Sprintf("dem-c-%03d", i)
		resp, err := http.PostForm(base+"/ask-around", url.Values{"set": {"DEM"}, "card": {id}, "finish": {"nonfoil"}})
		if err != nil {
			t.Fatalf("POST /ask-around: %v", err)
		}
		page = readAll(t, resp)
		if resp.StatusCode != http.StatusOK {
			continue
		}
		entry := entryOf(t, page, id)
		if strings.Contains(entry, want) && strings.Contains(entry, "asks ") {
			return id, i + 1, page
		}
		if request := regexp.MustCompile(`name="request" value="(\d+)"`).FindStringSubmatch(entry); request != nil {
			postBody(t, base, "/withdraw", url.Values{"request": {request[1]}, "set": {"DEM"}, "card": {id}})
		}
	}
	t.Fatalf("no common of DEM from %d on has the answers %q", from, want)
	return "", 0, ""
}

// entryOf is the slot of the card id in a binder page.
func entryOf(t *testing.T, page, id string) string {
	t.Helper()
	return firstMatch(t, regexp.MustCompile(`(?s)(<li class="entry" id="c-`+regexp.QuoteMeta(id)+`">.*?</li>\s*(?:<li class="entry"|</ol>))`), page)
}

func firstMatch(t *testing.T, re *regexp.Regexp, s string) string {
	t.Helper()
	m := re.FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("no match for %v in:\n%s", re, s)
	}
	return m[1]
}

// cheapestAnswerID is the ID of the answer a slot offers to accept that asks the lowest price, or
// of the first one when the answers are offers of cards, which have none.
func cheapestAnswerID(t *testing.T, entry string) string {
	t.Helper()
	ids := regexp.MustCompile(`action="/accept">\s*<input type="hidden" name="answer" value="(\d+)">`).FindAllStringSubmatch(entry, -1)
	prices := regexp.MustCompile(`asks (\d+) coins?\.`).FindAllStringSubmatch(entry, -1)
	if len(ids) == 0 {
		t.Fatalf("no answer to accept in:\n%s", entry)
	}
	if len(prices) != len(ids) {
		return ids[0][1]
	}
	best, bestPrice := 0, -1
	for i, p := range prices {
		price, err := strconv.Atoi(p[1])
		if err != nil {
			t.Fatalf("price %q: %v", p[1], err)
		}
		if bestPrice < 0 || price < bestPrice {
			best, bestPrice = i, price
		}
	}
	return ids[best][1]
}

func pageHas(t *testing.T, what, page string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(page, want) {
			t.Errorf("%s lacks %q:\n%s", what, want, page)
		}
	}
}

func pageHasNot(t *testing.T, what, page string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(page, bad) {
			t.Errorf("%s has %q:\n%s", what, bad, page)
		}
	}
}
