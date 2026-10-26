package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

func TestMarketRefusal_everyRefusalOfTheMarketHasAStatusAndAMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err    error
		status int
	}{
		{play.ErrNoMarket, http.StatusNotFound},
		{play.ErrNotOnMarket, http.StatusNotFound},
		{play.ErrUnknownCard, http.StatusBadRequest},
		{play.ErrLocked, http.StatusConflict},
		{play.ErrNobodyHasIt, http.StatusConflict},
		{play.ErrAlreadyAsked, http.StatusConflict},
		{play.ErrAlreadyListed, http.StatusConflict},
		{play.ErrReserved, http.StatusConflict},
		{play.ErrNotHeld, http.StatusConflict},
		{play.ErrInvalidPrice, http.StatusBadRequest},
		{play.ErrCoinsOnly, http.StatusBadRequest},
		{play.ErrNothingOffered, http.StatusBadRequest},
		{play.ErrNegative, http.StatusBadRequest},
		{play.ErrTooManyCards, http.StatusBadRequest},
		{play.ErrAwaitingReply, http.StatusConflict},
		{play.ErrGone, http.StatusConflict},
	}
	for _, tt := range tests {
		got, ok := marketRefusal(fmt.Errorf("action: %w", tt.err))
		if !ok || got.status != tt.status || got.message == "" {
			t.Errorf("marketRefusal(%v) = %+v, %v, want status %d and a message", tt.err, got, ok, tt.status)
		}
		if strings.Contains(got.message, tt.err.Error()) {
			t.Errorf("the message for %v repeats the error text: %q", tt.err, got.message)
		}
	}
	if _, ok := marketRefusal(errors.New("disk on fire")); ok {
		t.Error("an unexpected error is presented as a refusal")
	}
}

func TestRequestStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		asked, pending, answers int
		want                    string
	}{
		{3, 3, 0, "3 NPCs asked, no answer yet"},
		{1, 1, 0, "1 NPC asked, no answer yet"},
		{3, 2, 1, "3 NPCs asked, 2 still to answer"},
		{3, 0, 2, "3 NPCs asked, all have answered"},
		{3, 0, 0, "3 NPCs asked, none of the answers is left"},
	}
	for _, tt := range tests {
		if got := requestStatus(tt.asked, tt.pending, tt.answers); got != tt.want {
			t.Errorf("requestStatus(%d, %d, %d) = %q, want %q", tt.asked, tt.pending, tt.answers, got, tt.want)
		}
	}
}

func TestTermsText(t *testing.T) {
	t.Parallel()
	bolt := play.CardLine{Card: domain.Card{Name: "Bolt"}, Finish: domain.FinishNonFoil, Qty: 2}
	sol := play.CardLine{Card: domain.Card{Name: "Sol Ring"}, Finish: domain.FinishFoil, Qty: 1}
	tests := []struct {
		coins domain.Coins
		cards []play.CardLine
		want  string
	}{
		{83, nil, "◎83"},
		{1, nil, "◎1"},
		{0, []play.CardLine{bolt}, "Bolt ×2"},
		{10, []play.CardLine{bolt, sol}, "◎10 and Bolt ×2 and Sol Ring (foil) ×1"},
	}
	for _, tt := range tests {
		if got := termsText(tt.coins, tt.cards); got != tt.want {
			t.Errorf("termsText(%d, %v) = %q, want %q", tt.coins, tt.cards, got, tt.want)
		}
	}
}

func TestNewResultView_saysEveryWayAnAnswerEnds(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	card := domain.Card{Name: "Bolt"}
	npc := play.Answer{NPC: 4, Archetype: "playset_hunter", Coins: 30}
	countered := npc
	countered.Counter = &play.CounterTerms{Coins: 20}
	named := npc
	named.Name = "Ajani Goldmane"
	tests := []struct {
		name   string
		result play.Result
		want   string
	}{
		{"bought", play.Result{Kind: domain.AnswerAsk, Outcome: domain.MarketAccepted, Answer: npc}, "Bought Bolt from NPC 5 (playset hunter) for ◎30."},
		{"bought from a NPC with a name", play.Result{Kind: domain.AnswerAsk, Outcome: domain.MarketAccepted, Answer: named}, "Bought Bolt from Ajani Goldmane (playset hunter) for ◎30."},
		{"bought on a counter", play.Result{Kind: domain.AnswerAsk, Outcome: domain.MarketAccepted, Answer: countered}, "Bought Bolt from NPC 5 (playset hunter) for ◎20."},
		{"sold", play.Result{Kind: domain.AnswerOffer, Outcome: domain.MarketAccepted, Answer: npc}, "Sold Bolt to NPC 5 (playset hunter) for ◎30."},
		{"refused", play.Result{Kind: domain.AnswerAsk, Outcome: domain.MarketRefused, Answer: countered}, "NPC 5 (playset hunter) refused your counter of ◎20 for Bolt."},
		{"a price that lapsed", play.Result{Kind: domain.AnswerAsk, Outcome: domain.MarketExpired, Answer: npc}, "The price of NPC 5 (playset hunter) for Bolt lapsed."},
		{"an offer that lapsed", play.Result{Kind: domain.AnswerOffer, Outcome: domain.MarketExpired, Answer: npc}, "The offer of NPC 5 (playset hunter) for Bolt lapsed."},
		{"a deal that failed", play.Result{Kind: domain.AnswerOffer, Outcome: domain.MarketFailed, Answer: npc}, "The deal with NPC 5 (playset hunter) for Bolt fell through"},
	}
	for _, tt := range tests {
		tt.result.Card, tt.result.Finish, tt.result.At = card, domain.FinishNonFoil, now.Add(-3*time.Hour)
		got := newResultView(tt.result, now)
		if !strings.HasPrefix(got.Text, tt.want) || got.When != "3 hours ago" {
			t.Errorf("%s: result = %+v, want %q, 3 hours ago", tt.name, got, tt.want)
		}
	}
}

func TestCounterTerms_readsCoinsAndTheCardsGiven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		form      url.Values
		wantCoins domain.Coins
		wantLines []domain.TradeLine
		wantOK    bool
	}{
		{name: "empty", form: url.Values{}, wantOK: true},
		{name: "coins and a card", form: url.Values{"coins": {" 15 "}, "give:S-01:nonfoil": {"2"}}, wantCoins: 15, wantOK: true,
			wantLines: []domain.TradeLine{{Key: domain.CardKey{ID: "S-01", Finish: domain.FinishNonFoil}, Qty: 2}}},
		{name: "a card id with a colon", form: url.Values{"give:a:b:foil": {"1"}}, wantOK: true,
			wantLines: []domain.TradeLine{{Key: domain.CardKey{ID: "a:b", Finish: domain.FinishFoil}, Qty: 1}}},
		{name: "empty and zero quantities are no card", form: url.Values{"give:S-01:nonfoil": {""}, "give:S-02:nonfoil": {"0"}}, wantOK: true},
		{name: "other fields are not cards", form: url.Values{"giver": {"x"}, "give:nocolon": {"3"}}, wantOK: true},
		{name: "coins that are not a number", form: url.Values{"coins": {"1.5"}}},
		{name: "a quantity that is not a number", form: url.Values{"give:S-01:nonfoil": {"two"}}},
		{name: "a quantity too large to mean anything", form: url.Values{"give:S-01:nonfoil": {"4294967296"}}},
		{name: "negative numbers go to the game", form: url.Values{"coins": {"-3"}, "give:S-01:foil": {"-1"}}, wantCoins: -3, wantOK: true,
			wantLines: []domain.TradeLine{{Key: domain.CardKey{ID: "S-01", Finish: domain.FinishFoil}, Qty: -1}}},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodPost, "/counter-offer", strings.NewReader(tt.form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		coins, lines, ok := counterTerms(r)
		if ok != tt.wantOK || coins != tt.wantCoins || fmt.Sprint(lines) != fmt.Sprint(tt.wantLines) {
			t.Errorf("%s: counterTerms = %d, %v, %v, want %d, %v, %v", tt.name, coins, lines, ok, tt.wantCoins, tt.wantLines, tt.wantOK)
		}
	}
}

func TestBackOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query string
		want  string
		label string
	}{
		{"", "/market", "Back to the market"},
		{"from=market", "/market", "Back to the market"},
		{"from=market&set=S&card=S-00", "/market", "Back to the market"},
		{"set=S&card=S-00", "/collection?set=S#c-S-00", "Back to the binder"},
		{"set=S&card=S-00&rarity=rare", "/collection?rarity=rare&set=S#c-S-00", "Back to the binder"},
		{"set=S&card=S-00&rarity=rare&colour=U", "/collection?colour=U&rarity=rare&set=S#c-S-00", "Back to the binder"},
	}
	for _, tt := range tests {
		href := backOf(httptest.NewRequest(http.MethodGet, "/x?"+tt.query, nil)).href()
		if href != tt.want || backLabelOf(href) != tt.label {
			t.Errorf("backOf(%q) = %q, %q, want %q, %q", tt.query, href, backLabelOf(href), tt.want, tt.label)
		}
	}
}

func TestMarketIndex_aListedCardShowsListedAndAnotherShowsWhatACounterHolds(t *testing.T) {
	t.Parallel()
	listed := play.Listing{ID: 7, Card: domain.Card{ID: "A", Name: "A"}, Finish: domain.FinishNonFoil, Price: 40}
	view := play.MarketView{Listings: []play.Listing{listed}}
	idx := newMarketIndex(view, func(id domain.CardID, _ domain.Finish) int {
		if id == "B" {
			return 2
		}
		return 1
	}, nil)

	a := idx.sale(cardRef{Set: "S", ID: "A", Finish: "nonfoil"}, false, 48)
	b := idx.sale(cardRef{Set: "S", ID: "B", Finish: "nonfoil"}, false, 48)

	if a.Listing == nil || a.Listing.Price != 40 || a.Held != 0 {
		t.Errorf("the listed card = %+v, want its listing and nothing held for a counter", a)
	}
	if b.Listing != nil || b.Held != 2 || b.Price != 48 {
		t.Errorf("the card in a counter = %+v, want 2 held and the price to start from", b)
	}
}
