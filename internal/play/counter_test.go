package play_test

import (
	"errors"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const day = 24 * time.Hour

// cheap is a card at $1: the Counter asks 120 coins and pays 60.
func cheap() domain.CardKey { return domain.CardKey{ID: "S-00", Finish: domain.FinishNonFoil} }

// dear is a card at $2.50: the Counter asks 300 coins and pays 150.
func dear() domain.CardKey { return domain.CardKey{ID: "S-01", Finish: domain.FinishNonFoil} }

// counterSet is a set of two commons whose booster holds both, so that each welcome booster gives
// one copy of each.
func counterSet() domain.SetCatalogue { return playtest.TinySet("S", 2, 2) }

// counterRules is a Counter at the default rates on the prices of counterSet, one copy of each
// common in stock, the welcome bonus of 200 coins, and no supply or crowd.
func counterRules() play.CounterRules {
	return play.CounterRules{
		Params: domain.CounterParams{
			SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1,
			InitialStock: map[domain.Rarity]int{domain.RarityCommon: 1},
		},
		Prices:       domain.PriceSnapshot{"S-00": 100, "S-01": 250},
		WelcomeBonus: 200,
		CatchUpDays:  30,
	}
}

// counterGame starts a game on sets with a Counter under rules, on clock, and loads what store
// holds. Its streams start from fixed seeds, so that two games on the same store are the same game.
func counterGame(t *testing.T, store play.Store, clock *playtest.Clock, rules play.CounterRules, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	g, err := tryCounterGame(store, clock, rules, sets...)
	if err != nil {
		t.Fatalf("counter game: %v", err)
	}
	return g
}

func tryCounterGame(store play.Store, clock *playtest.Clock, rules play.CounterRules, sets ...domain.SetCatalogue) (*play.Game, error) {
	g, stream, err := newSeededGame(clock, sets...)
	if err != nil {
		return nil, err
	}
	if err := g.OpenCounter(rules, play.NewStream(7), play.NewStream(8)); err != nil {
		return nil, err
	}
	return g, g.Persist(play.Saves{Store: store, Stream: stream})
}

func newSeededGame(clock *playtest.Clock, sets ...domain.SetCatalogue) (*play.Game, *play.Stream, error) {
	stream := play.NewStream(1)
	g, err := play.NewGame(sets, nil, newOpenerOn(stream), playtest.Rules(sets...), clock.Now)
	return g, stream, err
}

func mustQuote(t *testing.T, g *play.Game, key domain.CardKey) play.Quote {
	t.Helper()
	q, err := g.CounterQuote(key.ID, key.Finish)
	if err != nil {
		t.Fatalf("CounterQuote(%v): %v", key, err)
	}
	return q
}

func held(g *play.Game, set string, key domain.CardKey) int {
	b, err := g.Binder(set)
	if err != nil {
		return -1
	}
	for _, s := range b.Slots {
		if s.Card.ID == key.ID {
			if key.Finish == domain.FinishFoil {
				return s.Foil
			}
			return s.Regular
		}
	}
	return 0
}

func TestOpenCounter_aNewGameStartsWithTheWelcomeBonus(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	if got := g.Balance(); got != 200 {
		t.Errorf("Balance = %d, want 200", got)
	}
}

func TestCounterQuote(t *testing.T) {
	t.Parallel()
	rules := counterRules()
	rules.Params.InitialStock = map[domain.Rarity]int{}
	rules.Prices = domain.PriceSnapshot{"S-00": 100}
	stocked := counterRules()
	clock := playtest.NewClock()

	tests := []struct {
		name  string
		rules play.CounterRules
		key   domain.CardKey
		want  play.Quote
	}{
		{name: "a card in stock", rules: stocked, key: cheap(), want: play.Quote{BuyPrice: 120, SellPrice: 60, InStock: true}},
		{name: "a card never in stock", rules: rules, key: cheap(), want: play.Quote{BuyPrice: 120, SellPrice: 60}},
		// The fallback of one cent still pays a coin, see ADR-0044.
		{name: "a card with no price", rules: rules, key: dear(), want: play.Quote{BuyPrice: 1, SellPrice: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := counterGame(t, &memStore{}, clock, tt.rules, counterSet())
			if got := mustQuote(t, g, tt.key); got != tt.want {
				t.Errorf("CounterQuote = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCounterQuote_anUnknownCard(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	for _, key := range []domain.CardKey{{ID: "nope", Finish: domain.FinishNonFoil}, {ID: cheap().ID, Finish: domain.FinishFoil}} {
		if _, err := g.CounterQuote(key.ID, key.Finish); !errors.Is(err, play.ErrUnknownCard) {
			t.Errorf("CounterQuote(%v) error = %v, want ErrUnknownCard", key, err)
		}
	}
}

// Buying a card the player does not hold puts it in the binder and takes its price; once the
// Counter has no copy left, its quote tells when it last had one.
func TestBuy_aMissingCardThenTheCounterIsOutOfStock(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, counterRules(), counterSet())

	clock.Advance(time.Hour)
	boughtAt := clock.Now()
	p, err := g.Buy(cheap().ID, cheap().Finish)
	if err != nil {
		t.Fatalf("Buy: %v", err)
	}
	if p.Paid != 120 || g.Balance() != 80 || held(g, "S", cheap()) != 1 {
		t.Errorf("paid %d, balance %d, held %d, want 120, 80 and 1", p.Paid, g.Balance(), held(g, "S", cheap()))
	}

	clock.Advance(5 * time.Minute)
	if _, err := g.Buy(cheap().ID, cheap().Finish); !errors.Is(err, play.ErrOutOfStock) {
		t.Fatalf("second Buy error = %v, want ErrOutOfStock", err)
	}
	q := mustQuote(t, g, cheap())
	if q.InStock || !q.LastInStock.Equal(boughtAt) {
		t.Errorf("quote = %+v, want out of stock since %v", q, boughtAt)
	}
	if g.Balance() != 80 || held(g, "S", cheap()) != 1 {
		t.Errorf("a refused purchase moved the balance to %d and the copies to %d", g.Balance(), held(g, "S", cheap()))
	}
}

func TestBuy_refusalsChangeNothingAndAreNotSaved(t *testing.T) {
	t.Parallel()
	stocked := counterRules()
	empty := counterRules()
	empty.Params.InitialStock = map[domain.Rarity]int{}
	unpriced := counterRules()
	unpriced.Prices = domain.PriceSnapshot{"S-00": 0}
	tests := []struct {
		name    string
		rules   play.CounterRules
		key     domain.CardKey
		bonus   domain.Coins
		wantErr error
	}{
		{name: "not enough coins", rules: stocked, key: dear(), bonus: 299, wantErr: play.ErrNotEnoughCoins},
		{name: "out of stock", rules: empty, key: cheap(), bonus: 1000, wantErr: play.ErrOutOfStock},
		{name: "a card the Counter does not trade", rules: unpriced, key: cheap(), bonus: 1000, wantErr: play.ErrNotTraded},
		{name: "an unknown card", rules: stocked, key: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, bonus: 1000, wantErr: play.ErrUnknownCard},
		{name: "a finish the card lacks", rules: stocked, key: domain.CardKey{ID: "S-00", Finish: domain.FinishFoil}, bonus: 1000, wantErr: play.ErrUnknownCard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := tt.rules
			r.WelcomeBonus = tt.bonus
			store := &playtest.FlakyStore{}
			g := counterGame(t, store, playtest.NewClock(), r, counterSet())
			saved := store.Saved()

			if _, err := g.Buy(tt.key.ID, tt.key.Finish); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Buy error = %v, want %v", err, tt.wantErr)
			}
			if g.Balance() != tt.bonus || held(g, "S", tt.key) > 0 {
				t.Errorf("a refused purchase left a balance of %d and %d copies", g.Balance(), held(g, "S", tt.key))
			}
			if store.Saved() != saved {
				t.Error("a refused purchase was saved")
			}
		})
	}
}

// The player may sell any copy, the only one included, and the Counter then has it in stock.
func TestSell_theOnlyCopy(t *testing.T) {
	t.Parallel()
	rules := counterRules()
	rules.Params.InitialStock = map[domain.Rarity]int{}
	g := counterGame(t, &memStore{}, playtest.NewClock(), rules, counterSet())
	mustOpen(t, g, "S")

	paid, err := g.Sell(dear().ID, dear().Finish, 1)
	if err != nil {
		t.Fatalf("Sell: %v", err)
	}
	if paid != 150 || g.Balance() != 350 || held(g, "S", dear()) != 0 {
		t.Errorf("paid %d, balance %d, held %d, want 150, 350 and 0", paid, g.Balance(), held(g, "S", dear()))
	}
	if !mustQuote(t, g, dear()).InStock {
		t.Error("the copy sold is not in the Counter's stock")
	}
}

func TestSell_refusalsChangeNothingAndAreNotSaved(t *testing.T) {
	t.Parallel()
	unpriced := counterRules()
	unpriced.Prices = domain.PriceSnapshot{"S-00": 0}
	tests := []struct {
		name    string
		rules   play.CounterRules
		key     domain.CardKey
		qty     int
		wantErr error
	}{
		{name: "a finish the card lacks", rules: counterRules(), key: domain.CardKey{ID: "S-00", Finish: domain.FinishFoil}, qty: 1, wantErr: play.ErrUnknownCard},
		{name: "more copies than held", rules: counterRules(), key: cheap(), qty: 2, wantErr: play.ErrNothingToSell},
		{name: "no copy", rules: counterRules(), key: cheap(), qty: 0, wantErr: play.ErrNothingToSell},
		{name: "an unknown card", rules: counterRules(), key: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, qty: 1, wantErr: play.ErrUnknownCard},
		{name: "a card the Counter does not trade", rules: unpriced, key: cheap(), qty: 1, wantErr: play.ErrNotTraded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := &playtest.FlakyStore{}
			g := counterGame(t, store, playtest.NewClock(), tt.rules, counterSet())
			mustOpen(t, g, "S")
			saved := store.Saved()
			if _, err := g.Sell(tt.key.ID, tt.key.Finish, tt.qty); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Sell error = %v, want %v", err, tt.wantErr)
			}
			if g.Balance() != 200 || held(g, "S", cheap()) != 1 {
				t.Errorf("a refused sale left a balance of %d and %d copies", g.Balance(), held(g, "S", cheap()))
			}
			if store.Saved() != saved {
				t.Error("a refused sale was saved")
			}
		})
	}
}

// A game without a Counter has no coins, and every Counter action says why it cannot.
func TestCounter_aGameWithoutOne(t *testing.T) {
	t.Parallel()
	g := newGame(t, counterSet())
	if got := g.Balance(); got != 0 {
		t.Errorf("Balance = %d, want 0", got)
	}
	actions := map[string]func() error{
		"quote":           func() error { _, err := g.CounterQuote(cheap().ID, cheap().Finish); return err },
		"buy":             func() error { _, err := g.Buy(cheap().ID, cheap().Finish); return err },
		"sell":            func() error { _, err := g.Sell(cheap().ID, cheap().Finish, 1); return err },
		"sell duplicates": func() error { _, err := g.SellDuplicates("S"); return err },
	}
	for name, action := range actions {
		if err := action(); !errors.Is(err, play.ErrNoCounter) {
			t.Errorf("%s error = %v, want ErrNoCounter", name, err)
		}
	}
}

// A Counter with a treasury that cannot pay refuses the sale, and the cards stay with the player.
func TestSell_aTreasuryThatCannotPay(t *testing.T) {
	t.Parallel()
	rules := counterRules()
	rules.Params.Treasury = true
	rules.Treasury = 100
	store := &playtest.FlakyStore{}
	g := counterGame(t, store, playtest.NewClock(), rules, counterSet())
	mustOpen(t, g, "S")
	mustOpen(t, g, "S")
	saved := store.Saved()

	if _, err := g.Sell(cheap().ID, cheap().Finish, 2); !errors.Is(err, play.ErrCounterBroke) {
		t.Fatalf("Sell error = %v, want ErrCounterBroke", err)
	}
	if store.Saved() != saved {
		t.Error("a refused sale was saved")
	}
	if g.Balance() != 200 || held(g, "S", cheap()) != 2 {
		t.Errorf("a refused sale left a balance of %d and %d copies", g.Balance(), held(g, "S", cheap()))
	}
	if _, err := g.Sell(cheap().ID, cheap().Finish, 1); err != nil {
		t.Errorf("a sale of 60 from a treasury of 100: %v", err)
	}
}

func TestSellDuplicates_sellsWhatIsAboveAPlaysetOfEveryCardOfTheSet(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet(), playtest.TinySet("T", 1, 1))
	for range 6 {
		mustOpen(t, g, "S")
	}
	mustOpen(t, g, "T")

	paid, err := g.SellDuplicates("S")
	if err != nil {
		t.Fatalf("SellDuplicates: %v", err)
	}
	// Two copies of each above the four kept: 2 x 60 + 2 x 150.
	if paid != 420 || g.Balance() != 620 {
		t.Errorf("paid %d, balance %d, want 420 and 620", paid, g.Balance())
	}
	if held(g, "S", cheap()) != domain.PlaysetSize || held(g, "S", dear()) != domain.PlaysetSize {
		t.Errorf("held %d and %d, want a playset of each", held(g, "S", cheap()), held(g, "S", dear()))
	}
	if _, err := g.SellDuplicates("S"); !errors.Is(err, play.ErrNothingToSell) {
		t.Errorf("second SellDuplicates error = %v, want ErrNothingToSell", err)
	}
	if _, err := g.SellDuplicates("NOPE"); !errors.Is(err, play.ErrUnknownSet) {
		t.Errorf("SellDuplicates of an unknown set error = %v, want ErrUnknownSet", err)
	}
}

// Buying the card that fills a binder unlocks sets as an opening does, and selling the binder
// empty afterwards locks nothing again: an unlock is permanent.
func TestBuy_unlocksSetsAndASaleNeverLocksThemAgain(t *testing.T) {
	t.Parallel()
	g, stream, err := newSeededGame(playtest.NewClock(), ladder()[:2]...)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := g.Stage(play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, play.NewStream(1)); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := g.OpenCounter(counterRules(), play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	store := &playtest.FlakyStore{}
	if err := g.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	saved := store.Saved()
	if _, err := g.Buy("B-00", domain.FinishNonFoil); !errors.Is(err, play.ErrLocked) {
		t.Fatalf("Buy of a locked set's card error = %v, want ErrLocked", err)
	}
	if store.Saved() != saved {
		t.Error("a refused purchase was saved")
	}

	p, err := g.Buy("A-00", domain.FinishNonFoil)
	if err != nil {
		t.Fatalf("Buy: %v", err)
	}
	if len(p.Unlocked) != 1 || p.Unlocked[0].Code != "B" {
		t.Fatalf("Unlocked = %+v, want B", p.Unlocked)
	}
	if _, err := g.Sell("A-00", domain.FinishNonFoil, 1); err != nil {
		t.Fatalf("Sell: %v", err)
	}
	if got := locked(g); len(got) != 0 {
		t.Errorf("locked after the sale = %v, want none", got)
	}
	if b := g.Binders()[0]; b.SetCode != "A" || !b.PastThreshold {
		t.Errorf("binder %+v lost its unlock mark", b)
	}
	if _, err := g.Buy("B-00", domain.FinishNonFoil); err != nil {
		t.Errorf("Buy of the unlocked set's card: %v", err)
	}
}

// Buying a card and selling it back always loses coins, whatever the card's price: the Counter
// asks 120% and pays 60%, and the one-coin floor of ADR-0044 at most evens a one-cent card out.
func TestCounter_aRoundTripNeverGainsCoins(t *testing.T) {
	t.Parallel()
	for _, price := range []domain.USDCents{1, 2, 3, 5, 7, 9, 10, 11, 99, 100, 101, 333, 999, 1234, 50_000, 999_999} {
		rules := counterRules()
		rules.Prices = domain.PriceSnapshot{"S-00": price}
		rules.WelcomeBonus = 10_000_000
		g := counterGame(t, &memStore{}, playtest.NewClock(), rules, counterSet())

		start := g.Balance()
		if _, err := g.Buy(cheap().ID, cheap().Finish); err != nil {
			t.Fatalf("price %d: Buy: %v", price, err)
		}
		if _, err := g.Sell(cheap().ID, cheap().Finish, 1); err != nil {
			t.Fatalf("price %d: Sell: %v", price, err)
		}
		if g.Balance() > start {
			t.Errorf("price %d: buying and selling back took the balance from %d to %d", price, start, g.Balance())
		}
		if price > 1 && g.Balance() == start {
			t.Errorf("price %d: a round trip cost nothing", price)
		}
	}
}
