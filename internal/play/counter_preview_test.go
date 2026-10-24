package play_test

import (
	"errors"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func TestHasCounter(t *testing.T) {
	t.Parallel()
	with := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	without, _, err := newSeededGame(playtest.NewClock(), counterSet())
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if !with.HasCounter() || without.HasCounter() {
		t.Errorf("HasCounter = %v with a Counter and %v without, want true and false", with.HasCounter(), without.HasCounter())
	}
}

// A card out of stock says how long ago the Counter last had a copy, by the game's clock.
func TestCounterQuote_saysHowLongAgoTheCounterLastHadACard(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, counterRules(), counterSet())
	if q := mustQuote(t, g, cheap()); q.LastInStockAgo != 0 {
		t.Errorf("a card in stock was last in stock %v ago, want 0", q.LastInStockAgo)
	}
	if _, err := g.Buy(cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("Buy: %v", err)
	}

	clock.Advance(5 * time.Minute)

	if q := mustQuote(t, g, cheap()); q.LastInStockAgo != 5*time.Minute {
		t.Errorf("LastInStockAgo = %v, want 5m", q.LastInStockAgo)
	}
}

func TestCounterQuote_aCardNeverInStockHasNoAge(t *testing.T) {
	t.Parallel()
	rules := counterRules()
	rules.Params.InitialStock = map[domain.Rarity]int{}
	g := counterGame(t, &memStore{}, playtest.NewClock(), rules, counterSet())
	if q := mustQuote(t, g, cheap()); q.InStock || q.LastInStockAgo != 0 || !q.LastInStock.IsZero() {
		t.Errorf("quote = %+v, want out of stock, never seen", q)
	}
}

func TestSaleEmptiesSlot(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	mustOpen(t, g, "S")
	mustOpen(t, g, "S")
	// The slot of S-00 holds two copies; a foil copy of it would keep the slot full.
	tests := []struct {
		name   string
		id     domain.CardID
		finish domain.Finish
		qty    int
		want   bool
	}{
		{name: "one of two copies", id: "S-00", finish: domain.FinishNonFoil, qty: 1, want: false},
		{name: "both copies", id: "S-00", finish: domain.FinishNonFoil, qty: 2, want: true},
		{name: "more copies than held", id: "S-00", finish: domain.FinishNonFoil, qty: 3, want: false},
		{name: "no copy at all", id: "S-00", finish: domain.FinishFoil, qty: 1, want: false},
		{name: "a card the game does not know", id: "nope", finish: domain.FinishNonFoil, qty: 1, want: false},
		{name: "a quantity below one", id: "S-00", finish: domain.FinishNonFoil, qty: 0, want: false},
	}
	for _, tt := range tests {
		if got := g.SaleEmptiesSlot(tt.id, tt.finish, tt.qty); got != tt.want {
			t.Errorf("%s: SaleEmptiesSlot = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDuplicates_isWhatSellDuplicatesWouldPayAndChangesNothing(t *testing.T) {
	t.Parallel()
	store := &playtest.FlakyStore{}
	g := counterGame(t, store, playtest.NewClock(), counterRules(), counterSet(), playtest.TinySet("T", 1, 1))
	empty, err := g.Duplicates("S", nil)
	if err != nil || empty.Copies != 0 || empty.Value != 0 {
		t.Fatalf("Duplicates of a binder with none = %+v, %v, want zero", empty, err)
	}
	for range 6 {
		mustOpen(t, g, "S")
	}
	mustOpen(t, g, "T")
	saved := store.Saved()

	got, err := g.Duplicates("S", nil)
	if err != nil {
		t.Fatalf("Duplicates: %v", err)
	}
	// Two copies of each above the four kept: 2 x 60 + 2 x 150.
	if want := (play.Duplicates{Copies: 4, Value: 420}); got != want {
		t.Errorf("Duplicates = %+v, want %+v", got, want)
	}
	if g.Balance() != 200 || store.Saved() != saved {
		t.Errorf("a preview moved the balance to %d or saved", g.Balance())
	}
	paid, err := g.SellDuplicates("S", nil)
	if err != nil || paid != got.Value {
		t.Errorf("SellDuplicates = %d, %v, want the %d the preview said", paid, err, got.Value)
	}
}

func TestDuplicates_onlyCountsAndSellsTheCardsIncludeAccepts(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	for range 6 {
		mustOpen(t, g, "S")
	}
	onlyDear := func(id domain.CardID) bool { return id == "S-01" }

	got, err := g.Duplicates("S", onlyDear)
	if err != nil {
		t.Fatalf("Duplicates: %v", err)
	}
	// Two copies of the one card accepted above the four kept: 2 x 150.
	if want := (play.Duplicates{Copies: 2, Value: 300}); got != want {
		t.Errorf("Duplicates = %+v, want %+v", got, want)
	}
	paid, err := g.SellDuplicates("S", onlyDear)
	if err != nil || paid != got.Value {
		t.Errorf("SellDuplicates = %d, %v, want the %d the preview said", paid, err, got.Value)
	}
	if rest, _ := g.Duplicates("S", nil); rest.Copies != 2 {
		t.Errorf("the duplicates left = %d copies, want the 2 of the card that was not accepted", rest.Copies)
	}
}

func TestDuplicates_refusals(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	if _, err := g.Duplicates("NOPE", nil); !errors.Is(err, play.ErrUnknownSet) {
		t.Errorf("Duplicates of an unknown set error = %v, want ErrUnknownSet", err)
	}
	bare, _, err := newSeededGame(playtest.NewClock(), counterSet())
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if _, err := bare.Duplicates("S", nil); !errors.Is(err, play.ErrNoCounter) {
		t.Errorf("Duplicates without a Counter error = %v, want ErrNoCounter", err)
	}
}

func TestCard(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	if card, ok := g.Card(cheap().ID); !ok || card.ID != cheap().ID || card.Name == "" {
		t.Errorf("Card(S-00) = %+v, %v, want the card", card, ok)
	}
	if _, ok := g.Card("nope"); ok {
		t.Error("Card(nope) = true, want false")
	}
}
