package web_test

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The NPCs of marketRules by ID. Their archetypes have no colour, so the market page names them
// after the first colourless characters, in ID order: "Karn Liberated" is fastNPC.
const (
	fastNPC = iota // a casual that answers after the delay given to marketRules and asks no margin
	slowNPC        // a casual that answers after slowDelay and asks 10%
	specNPC        // a speculator that opens nothing, answers after an hour and asks 20%
)

const slowDelay = 6 * time.Hour

// marketRules is counterRules with a market of three NPCs that trade, the fast one answering after
// fastDelay. The fast and the slow one open two boosters of counterSet a day, so that a new game's
// warm-up leaves a copy of each card on their shelves: the fast one asks 100 coins for cheapCard,
// the slow one 110, and the speculator pays 83 for a listed cheapCard.
func marketRules(fastDelay time.Duration) play.CounterRules {
	r := counterRules()
	r.Crowd = domain.PopulationParams{Size: 3, Valuer: domain.NewValuer(r.Prices, 1), MaxMarginPct: 1000, Archetypes: map[string]domain.ArchetypeSpec{
		"a_fast": {Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 2, KeepCopies: 1, Behaviour: "casual", ResponseDelay: fastDelay},
		"b_slow": {Share: 1, MinBoostersPerDay: 2, MaxBoostersPerDay: 2, KeepCopies: 1, Behaviour: "casual", MarginPct: 10, ResponseDelay: slowDelay},
		"c_spec": {Share: 1, KeepCopies: 1, Behaviour: "speculator", MarginPct: 20, ResponseDelay: time.Hour},
	}}
	r.Market = &play.MarketRules{SurplusHoldDays: 7, Params: domain.MarketParams{Expiry: 3 * 24 * time.Hour, MaxCardsPerSide: 10, CoinRate: 100}}
	return r
}

// newMarketShop serves a game with a market on counterSet, whose fast NPC answers at once.
func newMarketShop(t *testing.T) counterShop {
	t.Helper()
	return newShopWith(t, nil, marketRules(0), counterSet())
}

func (s counterShop) market(t *testing.T) play.MarketView {
	t.Helper()
	v, err := s.game.Market()
	if err != nil {
		t.Fatalf("Market: %v", err)
	}
	return v
}

// ask asks around for the card through the page.
func (s counterShop) ask(t *testing.T, card string) {
	t.Helper()
	if resp, body := s.post(t, "/ask-around", cardForm("S", card, "nonfoil")); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /ask-around %s = %d:\n%s", card, resp.StatusCode, body)
	}
}

// sellAll sells every copy of the card to the Counter, so that its slot is empty.
func (s counterShop) sellAll(t *testing.T, card string) {
	t.Helper()
	if _, err := s.game.Sell(domain.CardID(card), domain.FinishNonFoil, 6); err != nil {
		t.Fatalf("Sell: %v", err)
	}
}

// answerFrom returns the answer of npc to the request for the card, and fails without one.
func (s counterShop) answerFrom(t *testing.T, npc int) play.Answer {
	t.Helper()
	v := s.market(t)
	for _, r := range v.Requests {
		for _, a := range r.Answers {
			if a.NPC == npc {
				return a
			}
		}
	}
	for _, l := range v.Listings {
		for _, a := range l.Answers {
			if a.NPC == npc {
				return a
			}
		}
	}
	t.Fatalf("no answer of NPC %d in %+v", npc, v)
	return play.Answer{}
}

func answerForm(a play.Answer) url.Values {
	return url.Values{"answer": {strconv.Itoa(int(a.ID))}, "set": {"S"}, "card": {cheapCard}}
}

func listForm(card, price string) url.Values {
	f := cardForm("S", card, "nonfoil")
	f.Set("price", price)
	return f
}

func redirectsTo(t *testing.T, what string, resp *http.Response, location string) {
	t.Helper()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != location {
		t.Errorf("%s = %d to %q, want 303 to %q", what, resp.StatusCode, resp.Header.Get("Location"), location)
	}
}

// --- The hunt, from an empty slot ---

func TestMarketSlot_anEmptySlotOffersToAskAroundOnlyInAGameWithAMarket(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	counterOnly := newCounterShop(t)

	got := slot(t, shop.get(t, "/collection?set=S"), cheapCard)
	mustContain(t, "an empty slot", got, `action="/ask-around"`, `name="card" value="S-00"`, `name="finish" value="nonfoil"`, ">Ask around", "for S-00")
	mustContain(t, "an empty slot", got, `action="/buy"`)
	mustNotContain(t, "an empty slot", got, `action="/list"`, "asked")

	without := counterOnly.get(t, "/collection?set=S")
	mustNotContain(t, "a game without a market", without, `action="/ask-around"`, `action="/list"`, `href="/market"`)
}

func TestMarketSlot_aLockedBindersEmptySlotsCannotAskAround(t *testing.T) {
	t.Parallel()
	shop := newShopWith(t, &play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, marketRules(0), playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1))

	body := shop.get(t, "/collection?set=B")

	mustNotContain(t, "the locked binder", body, `action="/ask-around"`)
}

func TestAskAround_theSlotShowsThePendingStateAndTheAnswersAsTheyArrive(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	resp, _ := shop.post(t, "/ask-around", func() url.Values { f := cardForm("S", cheapCard, "nonfoil"); f.Set("rarity", "common"); return f }())

	redirectsTo(t, "POST /ask-around", resp, "/collection?rarity=common&set=S#c-S-00")
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after asking", body, "Asking around for S-00.")
	got := slot(t, body, cheapCard)
	mustContain(t, "the slot after asking", got,
		"2 NPCs asked, 1 still to answer",
		"Karn Liberated (a fast)", "asks ◎100",
		`action="/accept"`, `action="/decline"`, "/counter-offer?", `action="/withdraw"`)
	mustNotContain(t, "the slot after asking", got, `action="/ask-around"`, "Karn, the Great Creator")
	mustContain(t, "the answer's buttons for a screen reader", got,
		`Accept<span class="sr-only"> Karn Liberated (a fast) for S-00</span>`,
		`Counter<span class="sr-only"> Karn Liberated (a fast) for S-00</span>`,
		`Decline<span class="sr-only"> Karn Liberated (a fast) for S-00</span>`)

	shop.clock.Advance(slowDelay)
	got = slot(t, shop.get(t, "/collection?set=S"), cheapCard)
	mustContain(t, "the slot a few hours later", got, "2 NPCs asked, all have answered", "Karn Liberated (a fast)", "Karn, the Great Creator (b slow)", "asks ◎110", "Lapses in")
	mustNotContain(t, "the slot a few hours later", got, "still to answer")
}

func TestAskAround_withNoAnswerYetTheSlotSaysSo(t *testing.T) {
	t.Parallel()
	shop := newShopWith(t, nil, marketRules(time.Hour), counterSet())
	shop.ask(t, cheapCard)

	got := slot(t, shop.get(t, "/collection?set=S"), cheapCard)

	mustContain(t, "the pending slot", got, "2 NPCs asked, no answer yet", `action="/withdraw"`)
	mustNotContain(t, "the pending slot", got, `action="/accept"`, "asks ")
	if shop.market(t).Waiting != 0 {
		t.Errorf("waiting = %d, want 0 before any answer", shop.market(t).Waiting)
	}
}

func TestAskAround_whenEveryAnswerIsGoneTheSlotOffersToAskAgain(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	shop.clock.Advance(slowDelay)
	for _, npc := range []int{fastNPC, slowNPC} {
		shop.post(t, "/decline", answerForm(shop.answerFrom(t, npc)))
	}

	got := slot(t, shop.get(t, "/collection?set=S"), cheapCard)

	mustContain(t, "the slot with nothing left", got, "2 NPCs asked, none of the answers is left", `action="/ask-around"`, `action="/withdraw"`)
}

func TestAskAround_refusalsShowAMessageAndChangeNothing(t *testing.T) {
	t.Parallel()
	staged := &play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}
	nobody := marketRules(0)
	nobody.Crowd.Archetypes = map[string]domain.ArchetypeSpec{"c_spec": nobody.Crowd.Archetypes["c_spec"]}
	tests := []struct {
		name   string
		stage  *play.StageRules
		rules  play.CounterRules
		sets   []domain.SetCatalogue
		before func(*testing.T, counterShop)
		form   url.Values
		status int
		want   string
	}{
		{name: "no market", rules: counterRules(), sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", cheapCard, "nonfoil"), status: http.StatusNotFound, want: "no market"},
		{name: "nobody has the card", rules: nobody, sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", cheapCard, "nonfoil"), status: http.StatusConflict, want: "No NPC has this card"},
		{name: "already asking", rules: marketRules(time.Hour), sets: []domain.SetCatalogue{counterSet()}, before: func(t *testing.T, s counterShop) { s.ask(t, cheapCard) }, form: cardForm("S", cheapCard, "nonfoil"), status: http.StatusConflict, want: "already asking around"},
		{name: "a locked set", stage: staged, rules: marketRules(0), sets: []domain.SetCatalogue{playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1)}, form: cardForm("B", "B-00", "nonfoil"), status: http.StatusConflict, want: "locked set"},
		{name: "an unknown card", rules: marketRules(0), sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", "nope", "nonfoil"), status: http.StatusBadRequest, want: "Unknown card"},
		{name: "an unknown finish", rules: marketRules(0), sets: []domain.SetCatalogue{counterSet()}, form: cardForm("S", cheapCard, "gilded"), status: http.StatusBadRequest, want: "Unknown card"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shop := newShopWith(t, tt.stage, tt.rules, tt.sets...)
			if tt.before != nil {
				tt.before(t, shop)
			}
			requests := len(shop.marketOrNone(t).Requests)

			resp, body := shop.post(t, "/ask-around", tt.form)

			if resp.StatusCode != tt.status {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.status)
			}
			mustContain(t, "the refusal", body, tt.want, `role="alert"`, `href="/collection`)
			if got := len(shop.marketOrNone(t).Requests); got != requests {
				t.Errorf("a refused request changed the requests from %d to %d", requests, got)
			}
		})
	}
}

// marketOrNone is the market, empty in a game without one.
func (s counterShop) marketOrNone(t *testing.T) play.MarketView {
	t.Helper()
	v, err := s.game.Market()
	if err != nil && !errors.Is(err, play.ErrNoMarket) {
		t.Fatalf("Market: %v", err)
	}
	return v
}

func TestAskAround_withdrawingStopsTheRequestAndTellsSo(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	request := shop.market(t).Requests[0]

	resp, _ := shop.post(t, "/withdraw", url.Values{"request": {strconv.Itoa(int(request.ID))}, "set": {"S"}, "card": {cheapCard}})

	redirectsTo(t, "POST /withdraw", resp, "/collection?set=S#c-S-00")
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after withdrawing", body, "Stopped asking around for S-00.")
	mustContain(t, "the slot after withdrawing", slot(t, body, cheapCard), `action="/ask-around"`)
	if got := shop.market(t).Requests; len(got) != 0 {
		t.Errorf("requests = %+v, want none", got)
	}

	resp, body = shop.post(t, "/withdraw", url.Values{"request": {strconv.Itoa(int(request.ID))}})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("withdrawing twice = %d, want 404", resp.StatusCode)
	}
	mustContain(t, "the second withdrawal", body, "no longer on the market")
}

func TestAccept_anAskingPriceBuysTheCardAndComesBackToItsSlot(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)

	resp, _ := shop.post(t, "/accept", answerForm(shop.answerFrom(t, fastNPC)))

	redirectsTo(t, "POST /accept", resp, "/collection?set=S#c-S-00")
	if shop.game.Balance() != 100 {
		t.Errorf("balance %d, want 100: 200 less the asking price", shop.game.Balance())
	}
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after the deal", body, "Bought S-00 from Karn Liberated (a fast) for ◎100.", "<strong>100</strong>")
	got := slot(t, body, cheapCard)
	mustContain(t, "the bought card", got, `action="/sell"`, `action="/list"`)
	mustNotContain(t, "the bought card", got, "asked", `action="/accept"`)
	mustNotContain(t, "the notice is shown once", shop.get(t, "/collection?set=S"), "Bought S-00")
}

func TestAccept_aDealThatCompletesABinderSaysWhichSetsItUnlocked(t *testing.T) {
	t.Parallel()
	shop := newShopWith(t, &play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, marketRules(0), playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1))
	if resp, body := shop.post(t, "/ask-around", cardForm("A", "A-00", "nonfoil")); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /ask-around = %d:\n%s", resp.StatusCode, body)
	}
	answer := shop.market(t).Requests[0].Answers[0]

	shop.post(t, "/accept", url.Values{"answer": {strconv.Itoa(int(answer.ID))}, "set": {"A"}, "card": {"A-00"}})

	mustContain(t, "the binder after the deal", shop.get(t, "/collection?set=A"), "Unlocked:", "Set B")
}

func TestAccept_refusalsShowAMessageAndChangeNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		before func(*testing.T, counterShop) url.Values
		status int
		want   string
	}{
		{name: "not enough coins", before: func(t *testing.T, s counterShop) url.Values {
			s.ask(t, cheapCard)
			if _, err := s.game.Buy(dearCard, domain.FinishNonFoil); err == nil {
				t.Fatal("the dear card was bought with too few coins")
			}
			if _, err := s.game.Buy(cheapCard, domain.FinishNonFoil); err != nil {
				t.Fatalf("Buy: %v", err)
			}
			return answerForm(s.answerFrom(t, fastNPC))
		}, status: http.StatusConflict, want: "Not enough coins: you have 80"},
		{name: "an answer that is gone", before: func(*testing.T, counterShop) url.Values {
			return url.Values{"answer": {"999"}, "set": {"S"}, "card": {cheapCard}}
		}, status: http.StatusNotFound, want: "no longer on the market"},
		{name: "no answer", before: func(*testing.T, counterShop) url.Values { return url.Values{} }, status: http.StatusBadRequest, want: "Unknown answer"},
		{name: "an answer that is not a number", before: func(*testing.T, counterShop) url.Values { return url.Values{"answer": {"x"}} }, status: http.StatusBadRequest, want: "Unknown answer"},
		{name: "an answer being countered", before: func(t *testing.T, s counterShop) url.Values {
			s.ask(t, cheapCard)
			s.clock.Advance(slowDelay)
			slow := s.answerFrom(t, slowNPC)
			if err := s.game.CounterAnswer(slow.ID, 105, nil); err != nil {
				t.Fatalf("CounterAnswer: %v", err)
			}
			return answerForm(slow)
		}, status: http.StatusConflict, want: "wait for the NPC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shop := newMarketShop(t)
			form := tt.before(t, shop)
			coins, results := shop.game.Balance(), len(shop.market(t).Results)

			resp, body := shop.post(t, "/accept", form)

			if resp.StatusCode != tt.status {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.status)
			}
			mustContain(t, "the refusal", body, tt.want, `role="alert"`)
			if shop.game.Balance() != coins || len(shop.market(t).Results) != results {
				t.Errorf("a refused acceptance changed the balance from %d to %d or the results", coins, shop.game.Balance())
			}
		})
	}
}

func TestDecline_turnsTheAnswerDownAndKeepsTheRequest(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)

	resp, _ := shop.post(t, "/decline", answerForm(shop.answerFrom(t, fastNPC)))

	redirectsTo(t, "POST /decline", resp, "/collection?set=S#c-S-00")
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after declining", body, "Declined the answer of Karn Liberated (a fast) for S-00.")
	got := slot(t, body, cheapCard)
	mustContain(t, "the slot after declining", got, "2 NPCs asked, no answer yet")
	mustNotContain(t, "the slot after declining", got, `action="/accept"`)
	if shop.game.Balance() != 200 {
		t.Errorf("declining moved the balance to %d", shop.game.Balance())
	}
}

func TestDecline_anAnswerThatIsGoneIsRefused(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	resp, body := shop.post(t, "/decline", url.Values{"answer": {"999"}})

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, want 404", resp.StatusCode)
	}
	mustContain(t, "the refusal", body, "no longer on the market")
}

// --- Countering ---

func TestCounterOffer_theFormOfAnAskingPriceTakesCoinsAndTheSpareCards(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 6)
	shop.sellAll(t, neverCard)
	shop.ask(t, neverCard)
	answer := shop.answerFrom(t, fastNPC)

	_, body := get(t, shop.c, shop.srv.URL+"/counter-offer?answer="+strconv.Itoa(int(answer.ID))+"&set=S&card=S-02")

	mustContain(t, "the counter form", body,
		"Counter Karn Liberated (a fast)", "S-02", "asks ◎100",
		`action="/counter-offer"`, `name="answer" value="`+strconv.Itoa(int(answer.ID))+`"`, `name="coins"`,
		`name="give:S-00:nonfoil"`, `name="give:S-01:nonfoil"`, `max="2"`, "Send the counter",
		`href="/collection?set=S#c-S-02"`)
}

func TestCounterOffer_aPlayerWithNoSpareCardOffersCoinsOnly(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	answer := shop.answerFrom(t, fastNPC)

	_, body := get(t, shop.c, shop.srv.URL+"/counter-offer?answer="+strconv.Itoa(int(answer.ID)))

	mustContain(t, "the counter form", body, `name="coins"`, "no spare cards", `href="/market"`)
	mustNotContain(t, "the counter form", body, `name="give:`)
}

func TestCounterOffer_anAnswerThatIsGoneHasNoForm(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	for path, want := range map[string]int{"/counter-offer?answer=999": http.StatusNotFound, "/counter-offer": http.StatusBadRequest, "/counter-offer?answer=x": http.StatusBadRequest} {
		resp, body := get(t, shop.c, shop.srv.URL+path)
		if resp.StatusCode != want {
			t.Errorf("GET %s = %d, want %d", path, resp.StatusCode, want)
		}
		mustContain(t, path, body, `role="alert"`)
	}
}

func TestCounterOffer_anAnswerAlreadyCounteredHasNoForm(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	shop.clock.Advance(slowDelay)
	slow := shop.answerFrom(t, slowNPC)
	if err := shop.game.CounterAnswer(slow.ID, 105, nil); err != nil {
		t.Fatalf("CounterAnswer: %v", err)
	}

	resp, body := get(t, shop.c, shop.srv.URL+"/counter-offer?answer="+strconv.Itoa(int(slow.ID)))

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status %d, want 409", resp.StatusCode)
	}
	mustContain(t, "the refusal", body, "wait for the NPC")
}

func TestCounterOffer_aCounterWaitsForTheNPCAndIsSettledAfterItsDelay(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	shop.clock.Advance(slowDelay)
	slow := shop.answerFrom(t, slowNPC)
	form := answerForm(slow)
	form.Set("coins", "110")

	resp, _ := shop.post(t, "/counter-offer", form)

	redirectsTo(t, "POST /counter-offer", resp, "/collection?set=S#c-S-00")
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after countering", body, "Countered the answer of Karn, the Great Creator (b slow) for S-00")
	got := slot(t, body, cheapCard)
	mustContain(t, "the countered answer", got, "You countered with ◎110", "Karn, the Great Creator replies in 6 h")
	if slow := strings.Count(got, "/counter-offer?"); slow != 1 {
		t.Errorf("the slot offers to counter %d answers, want only the fast NPC's", slow)
	}
	if shop.game.Balance() != 200 {
		t.Errorf("a counter moved the balance to %d before the reply", shop.game.Balance())
	}

	shop.clock.Advance(slowDelay)
	// 110 coins is what the slow NPC asks, so it takes the card.
	after := shop.get(t, "/collection?set=S")
	if shop.game.Balance() != 90 {
		t.Errorf("balance %d after the reply, want 90", shop.game.Balance())
	}
	mustContain(t, "the slot once the NPC took it", slot(t, after, cheapCard), `action="/sell"`)
}

func TestCounterOffer_cardsInACounterAreHeldAndShownAsHeld(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 6)
	shop.sellAll(t, neverCard)
	shop.ask(t, neverCard)
	shop.clock.Advance(slowDelay)
	slow := shop.answerFrom(t, slowNPC)
	form := answerForm(slow)
	form.Set("coins", "0")
	form.Set("give:S-01:nonfoil", "2")

	resp, _ := shop.post(t, "/counter-offer", form)

	redirectsTo(t, "POST /counter-offer", resp, "/collection?set=S#c-S-00")
	if got := shop.game.Reserved(dearCard, domain.FinishNonFoil); got != 2 {
		t.Errorf("reserved = %d, want the 2 copies of the counter", got)
	}
	body := shop.get(t, "/collection?set=S")
	got := slot(t, body, dearCard)
	mustContain(t, "the card in a counter", got, "2 held for a counter")
	mustContain(t, "the counter", slot(t, body, neverCard), "You countered with S-01 ×2")
}

func TestCounterOffer_refusalsShowAMessageAndChangeNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		coins  string
		give   map[string]string
		status int
		want   string
	}{
		{name: "nothing offered", coins: "", status: http.StatusBadRequest, want: "Offer coins or cards"},
		{name: "negative coins", coins: "-5", status: http.StatusBadRequest, want: "no negative"},
		{name: "coins that are not a number", coins: "lots", status: http.StatusBadRequest, want: "whole numbers"},
		{name: "a quantity that is not a number", coins: "5", give: map[string]string{"give:S-01:nonfoil": "x"}, status: http.StatusBadRequest, want: "whole numbers"},
		{name: "more coins than held", coins: "500", status: http.StatusConflict, want: "Not enough coins: you have 200"},
		{name: "cards not held", coins: "5", give: map[string]string{"give:S-03:nonfoil": "1"}, status: http.StatusConflict, want: "do not hold"},
		{name: "a card that does not exist", coins: "5", give: map[string]string{"give:nope:nonfoil": "1"}, status: http.StatusConflict, want: "do not hold"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shop := newMarketShop(t)
			shop.ask(t, cheapCard)
			shop.clock.Advance(slowDelay)
			form := answerForm(shop.answerFrom(t, slowNPC))
			form.Set("coins", tt.coins)
			for k, v := range tt.give {
				form.Set(k, v)
			}

			resp, body := shop.post(t, "/counter-offer", form)

			if resp.StatusCode != tt.status {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.status)
			}
			mustContain(t, "the refusal", body, tt.want, `role="alert"`)
			if got := shop.answerFrom(t, slowNPC); got.Counter != nil {
				t.Errorf("a refused counter was kept: %+v", got.Counter)
			}
		})
	}
}

// --- The sale, from a held card ---

func TestMarketSlot_aHeldCardOffersToListWithTheCountersSellingPriceAsAsk(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 1)

	body := shop.get(t, "/collection?set=S")

	got := slot(t, body, cheapCard)
	mustContain(t, "a held card", got, `action="/list"`, `name="price"`, `value="120"`, "List", "on the market", `aria-describedby="price-hint"`)
	mustContain(t, "a held card, which can be asked around for again, for a playset", got, `action="/ask-around"`)
	mustNotContain(t, "a held card", got, "Listed at")
	mustContain(t, "the binder", body, `id="price-hint"`, "what the Counter sells")
	// A card the Counter does not trade has no reference to start from.
	mustNotContain(t, "an untraded card", slot(t, body, unpricedKey), `value="1"`)
}

func TestList_listsTheCardAndTheSlotShowsItListedWithItsOffers(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 2)

	resp, _ := shop.post(t, "/list", listForm(cheapCard, "100"))

	redirectsTo(t, "POST /list", resp, "/collection?set=S#c-S-00")
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after listing", body, "Listed S-00 for ◎100.")
	got := slot(t, body, cheapCard)
	mustContain(t, "the listed card", got, "Listed at ◎100", "No offers yet", `action="/unlist"`)
	mustNotContain(t, "the listed card", got, `action="/list"`)
	if shop.game.Reserved(cheapCard, domain.FinishNonFoil) != 1 {
		t.Error("the listing holds no copy")
	}

	shop.clock.Advance(time.Hour)
	got = slot(t, shop.get(t, "/collection?set=S"), cheapCard)
	mustContain(t, "the listed card with an offer", got, "Karn, Scion of Urza (c spec)", "offers ◎83", `action="/accept"`, `action="/decline"`, "/counter-offer?")
	mustNotContain(t, "the listed card with an offer", got, "No offers yet")
}

func TestAccept_anOfferSellsTheListedCard(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 2)
	shop.post(t, "/list", listForm(cheapCard, "100"))
	shop.clock.Advance(time.Hour)
	offer := shop.answerFrom(t, specNPC)

	resp, _ := shop.post(t, "/accept", answerForm(offer))

	redirectsTo(t, "POST /accept", resp, "/collection?set=S#c-S-00")
	if shop.game.Balance() != 283 {
		t.Errorf("balance %d, want 283", shop.game.Balance())
	}
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after the sale", body, "Sold S-00 to Karn, Scion of Urza (c spec) for ◎83.")
	mustContain(t, "the slot after the sale", slot(t, body, cheapCard), `action="/list"`)
}

func TestList_unlistingFreesTheCard(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 1)
	shop.post(t, "/list", listForm(cheapCard, "100"))
	listing := shop.market(t).Listings[0]

	resp, _ := shop.post(t, "/unlist", url.Values{"listing": {strconv.Itoa(int(listing.ID))}, "set": {"S"}, "card": {cheapCard}})

	redirectsTo(t, "POST /unlist", resp, "/collection?set=S#c-S-00")
	body := shop.get(t, "/collection?set=S")
	mustContain(t, "the binder after unlisting", body, "Took S-00 off the market.")
	mustContain(t, "the slot after unlisting", slot(t, body, cheapCard), `action="/list"`)
	if shop.game.Reserved(cheapCard, domain.FinishNonFoil) != 0 {
		t.Error("the copy is still held after unlisting")
	}
	if resp, _ := shop.post(t, "/unlist", url.Values{"listing": {strconv.Itoa(int(listing.ID))}}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unlisting twice = %d, want 404", resp.StatusCode)
	}
}

func TestCounterOffer_anOfferIsCounteredWithCoinsOnly(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 2)
	shop.post(t, "/list", listForm(cheapCard, "100"))
	shop.clock.Advance(time.Hour)
	offer := shop.answerFrom(t, specNPC)

	_, body := get(t, shop.c, shop.srv.URL+"/counter-offer?answer="+strconv.Itoa(int(offer.ID))+"&set=S&card=S-00")
	mustContain(t, "the counter form of an offer", body, "Counter Karn, Scion of Urza (c spec)", "offers ◎83", `name="coins"`, "Coins you ask for")
	mustNotContain(t, "the counter form of an offer", body, `name="give:`)

	form := answerForm(offer)
	form.Set("coins", "90")
	resp, _ := shop.post(t, "/counter-offer", form)
	redirectsTo(t, "POST /counter-offer", resp, "/collection?set=S#c-S-00")
	mustContain(t, "the slot after the counter", slot(t, shop.get(t, "/collection?set=S"), cheapCard), "You countered with ◎90", "Karn, Scion of Urza replies in 1 h")
}

func TestCounterOffer_aCardsOnlyCounterOnAnOfferIsRefusedAsCoinsOnly(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 6)
	shop.post(t, "/list", listForm(cheapCard, "100"))
	shop.clock.Advance(time.Hour)
	form := answerForm(shop.answerFrom(t, specNPC))
	form.Set("coins", "90")
	form.Set("give:S-01:nonfoil", "1")

	resp, body := shop.post(t, "/counter-offer", form)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status %d, want 400", resp.StatusCode)
	}
	mustContain(t, "the refusal", body, "asks for coins")
}

func TestList_refusalsShowAMessageAndChangeNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		shop   func(*testing.T) counterShop
		before func(*testing.T, counterShop)
		form   url.Values
		status int
		want   string
	}{
		{name: "no market", shop: newCounterShop, form: listForm(cheapCard, "100"), status: http.StatusNotFound, want: "no market"},
		{name: "a card not held", form: listForm(cheapCard, "100"), status: http.StatusConflict, want: "do not hold"},
		{name: "a price of zero", before: func(t *testing.T, s counterShop) { s.open(t, 1) }, form: listForm(cheapCard, "0"), status: http.StatusBadRequest, want: "at least 1 coin"},
		{name: "no price", before: func(t *testing.T, s counterShop) { s.open(t, 1) }, form: listForm(cheapCard, ""), status: http.StatusBadRequest, want: "price"},
		{name: "a price that is not a number", before: func(t *testing.T, s counterShop) { s.open(t, 1) }, form: listForm(cheapCard, "dear"), status: http.StatusBadRequest, want: "price"},
		{name: "listed already", before: func(t *testing.T, s counterShop) {
			s.open(t, 2)
			s.post(t, "/list", listForm(cheapCard, "100"))
		}, form: listForm(cheapCard, "100"), status: http.StatusConflict, want: "on the market already"},
		{name: "an unknown card", before: func(t *testing.T, s counterShop) { s.open(t, 1) }, form: listForm("nope", "100"), status: http.StatusBadRequest, want: "Unknown card"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			newShop := tt.shop
			if newShop == nil {
				newShop = newMarketShop
			}
			shop := newShop(t)
			if tt.before != nil {
				tt.before(t, shop)
			}
			listings := len(shop.marketOrNone(t).Listings)

			resp, body := shop.post(t, "/list", tt.form)

			if resp.StatusCode != tt.status {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.status)
			}
			mustContain(t, "the refusal", body, tt.want, `role="alert"`)
			if got := len(shop.marketOrNone(t).Listings); got != listings {
				t.Errorf("a refused listing changed the listings from %d to %d", listings, got)
			}
		})
	}
}

func TestSell_aCardHeldForTheMarketIsRefusedWithALinkToTheMarket(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 1)
	shop.post(t, "/list", listForm(cheapCard, "100"))
	coins := shop.game.Balance()

	sale := sellForm(cheapCard, "1")
	sale.Set("confirm", "yes")
	resp, body := shop.post(t, "/sell", sale)

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("POST /sell of a listed card = %d, want 409", resp.StatusCode)
	}
	mustContain(t, "the refusal", body, "held for the market", `href="/market"`, `href="/collection?set=S#c-S-00"`, `role="alert"`)
	if shop.game.Balance() != coins {
		t.Errorf("a refused sale moved the balance from %d to %d", coins, shop.game.Balance())
	}
}

// --- The header and the market page ---

func TestHeader_theMarketLinkShowsTheAnswersWaitingAndOnlyInAGameWithAMarket(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	counterOnly := newCounterShop(t)

	for _, path := range []string{"/", "/collection", "/market"} {
		body := shop.get(t, path)
		mustContain(t, path, body, `href="/market"`)
		mustNotContain(t, path+" with no answer", body, `class="badge"`)
	}
	shop.ask(t, cheapCard)
	for _, path := range []string{"/", "/collection", "/market"} {
		mustContain(t, path+" with an answer", shop.get(t, path), `class="badge"`, `<span class="sr-only"> answer waiting</span>`)
	}
	shop.clock.Advance(slowDelay)
	mustContain(t, "two answers", shop.get(t, "/"), `<span class="sr-only"> answers waiting</span>`)

	shop.post(t, "/accept", answerForm(shop.answerFrom(t, fastNPC)))
	mustNotContain(t, "after the deal", shop.get(t, "/"), `class="badge"`)
	mustNotContain(t, "a game without a market", counterOnly.get(t, "/"), `href="/market"`)
}

func TestMarketPage_isEmptyAtFirst(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	body := shop.get(t, "/market")

	mustContain(t, "the empty market", body, "<h1>Market</h1>", "not asking around", "no card listed", "No deal yet", `href="/collection"`)
}

func TestMarketPage_listsEveryRequestListingAndAnswerNewestFirst(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 2)
	shop.ask(t, neverCard)
	shop.clock.Advance(time.Minute)
	shop.ask(t, dearCard)
	shop.post(t, "/list", listForm(cheapCard, "100"))
	shop.clock.Advance(time.Minute)
	shop.post(t, "/list", listForm(unpricedKey, "5"))

	body := shop.get(t, "/market")

	mustContain(t, "the market", body, "S-02", "S-01", "S-00", "2 NPCs asked, 1 still to answer", "Listed at ◎100", "Listed at ◎5", `action="/accept"`, `action="/unlist"`, `action="/withdraw"`, `name="from" value="market"`)
	if dear, never := strings.Index(body, "Asking around for S-01"), strings.Index(body, "Asking around for S-02"); dear < 0 || never < 0 || dear > never {
		t.Errorf("the request made last is at %d, the first at %d: want the newest first", dear, never)
	}
	if later, earlier := strings.Index(body, "Listed at ◎5"), strings.Index(body, "Listed at ◎100"); later > earlier {
		t.Error("the listing made last does not come first")
	}
}

func TestMarketPage_actionsComeBackToTheMarketAndShowTheResult(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	answer := shop.answerFrom(t, fastNPC)
	form := url.Values{"answer": {strconv.Itoa(int(answer.ID))}, "from": {"market"}}

	resp, _ := shop.post(t, "/accept", form)

	redirectsTo(t, "POST /accept from the market", resp, "/market")
	body := shop.get(t, "/market")
	mustContain(t, "the market after the deal", body, "Bought S-00 from Karn Liberated (a fast) for ◎100.", "Recent results", "less than a minute ago")
	mustNotContain(t, "the market after the deal", body, "No deal yet")

	if resp, body := shop.post(t, "/accept", form); resp.StatusCode != http.StatusNotFound || !strings.Contains(body, `href="/market"`) || !strings.Contains(body, "Back to the market") {
		t.Errorf("a refusal from the market = %d, want 404 with a way back to the market:\n%s", resp.StatusCode, body)
	}
}

func TestMarketPage_everyWayAnAnswerEndsIsTold(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 2)
	// Sold: the speculator's offer for a listed card.
	shop.post(t, "/list", listForm(cheapCard, "100"))
	shop.clock.Advance(time.Hour)
	shop.post(t, "/accept", answerForm(shop.answerFrom(t, specNPC)))
	// Refused: a counter far below the asking price.
	shop.ask(t, dearCard)
	shop.clock.Advance(slowDelay)
	slow := shop.answerFrom(t, slowNPC)
	counter := answerForm(slow)
	counter.Set("coins", "1")
	shop.post(t, "/counter-offer", counter)
	shop.clock.Advance(slowDelay)
	// Expired: the fast NPC's answer lapses after its three days.
	shop.clock.Advance(4 * 24 * time.Hour)

	body := shop.get(t, "/market")

	mustContain(t, "the results", body, "Sold S-00 to Karn, Scion of Urza (c spec) for ◎83.", "Karn, the Great Creator (b slow) refused your counter of ◎1 for S-01.", "The price of Karn Liberated (a fast) for S-01 lapsed")
}

func TestMarketPage_namesAreEscaped(t *testing.T) {
	t.Parallel()
	cat := counterSet()
	card := cat.Cards[cheapCard]
	card.Name = `<b onmouseover="x">`
	cat.Cards[cheapCard] = card
	shop := newShopWith(t, nil, marketRules(0), cat)
	shop.ask(t, cheapCard)
	shop.open(t, 1)
	shop.post(t, "/list", listForm(neverCard, "5"))

	for path, body := range map[string]string{"the market": shop.get(t, "/market"), "the binder": shop.get(t, "/collection?set=S")} {
		mustNotContain(t, path, body, `<b onmouseover`)
		mustContain(t, path, body, "&lt;b onmouseover")
	}
}

func TestMarketPage_aRequestShowsItsCardInGreyAndAListingShowsItInColour(t *testing.T) {
	t.Parallel()
	set := counterSet()
	ids := withScryfallIDs(set)
	shop := newShopWith(t, nil, marketRules(0), set)
	shop.ask(t, cheapCard)
	shop.open(t, 1)
	if resp, body := shop.post(t, "/list", listForm(dearCard, "100")); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /list = %d:\n%s", resp.StatusCode, body)
	}

	got := shop.get(t, "/market")

	mustContain(t, "the request", got, `<a class="card-thumb unowned" href="/collection?set=S#c-S-00">`, `<img src="/card-image/`+ids[domain.CardID(cheapCard)]+`"`)
	mustContain(t, "the listing", got, `<a class="card-thumb" href="/collection?set=S#c-S-01">`, `class="thumb-set"`, "Set S")
}
