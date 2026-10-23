package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// newTestCounter creates a Counter for tests, returning both the Counter and
// its Journal so callers can fund accounts and inspect balances.
func newTestCounter(t *testing.T, cats []domain.SetCatalogue, prices domain.PriceSnapshot, params domain.CounterParams) (*domain.Counter, *domain.Journal) {
	t.Helper()
	j := domain.NewJournal()
	c, err := domain.NewCounter(cats, prices, j, params)
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	return c, j
}

// counterParams returns CounterParams with sensible defaults for tests.
func counterParams() domain.CounterParams {
	return domain.CounterParams{
		SellPct:       60,
		BuyPct:        120,
		CoinRate:      100,
		FallbackPrice: 10,
		InitialStock: map[domain.Rarity]int{
			domain.RarityCommon:   2,
			domain.RarityUncommon: 1,
			domain.RarityRare:     1,
		},
	}
}

// counterPrices returns a PriceSnapshot with known prices for cards used in
// counter tests. synth-c-0002 is intentionally absent to exercise the fallback
// price path.
func counterPrices() domain.PriceSnapshot {
	return domain.PriceSnapshot{
		"synth-r-0000": 200, // $2.00 — used in happy-path and round-trip tests
		"synth-r-0001": 200,
		"synth-c-0000": 100, // $1.00 — has FinishNonFoil and FinishFoil
		"synth-c-0001": 100, // $1.00 — has FinishNonFoil only (odd index)
	}
}

// TestCounterNewValidation checks that NewCounter rejects invalid params.
func TestCounterNewValidation(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(0)
	j := domain.NewJournal()
	base := counterParams()

	tests := []struct {
		name    string
		journal *domain.Journal
		params  domain.CounterParams
		wantErr string
	}{
		{
			name:    "nil journal",
			journal: nil,
			params:  base,
			wantErr: "journal must not be nil",
		},
		{
			name:    "CoinRate zero",
			journal: j,
			params:  func() domain.CounterParams { p := base; p.CoinRate = 0; return p }(),
			wantErr: "CoinRate must be > 0",
		},
		{
			name:    "CoinRate negative",
			journal: j,
			params:  func() domain.CounterParams { p := base; p.CoinRate = -1; return p }(),
			wantErr: "CoinRate must be > 0",
		},
		{
			name:    "SellPct zero",
			journal: j,
			params:  func() domain.CounterParams { p := base; p.SellPct = 0; return p }(),
			wantErr: "SellPct must be > 0",
		},
		{
			name:    "SellPct above BuyPct",
			journal: j,
			params:  func() domain.CounterParams { p := base; p.SellPct = 130; p.BuyPct = 120; return p }(),
			wantErr: "SellPct (130) must be <= BuyPct (120)",
		},
		{
			name:    "FallbackPrice negative",
			journal: j,
			params:  func() domain.CounterParams { p := base; p.FallbackPrice = -1; return p }(),
			wantErr: "FallbackPrice must be >= 0",
		},
		{
			name:    "InitialStock negative",
			journal: j,
			params: func() domain.CounterParams {
				p := base
				p.InitialStock = map[domain.Rarity]int{domain.RarityCommon: -1}
				return p
			}(),
			wantErr: "InitialStock[common] must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewCounter([]domain.SetCatalogue{cat}, domain.PriceSnapshot{}, tt.journal, tt.params)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestCounterNewDuplicateCardID checks that NewCounter returns an error when
// the same CardID appears in more than one catalogue.
func TestCounterNewDuplicateCardID(t *testing.T) {
	t.Parallel()

	// NewCatalogue produces fixed IDs (e.g. synth-c-0000) regardless of seed.
	cat1 := domaintest.NewCatalogue(0)
	cat2 := domaintest.NewCatalogue(1) // same IDs, different set code
	cat2.Code = "SY2"

	_, err := domain.NewCounter(
		[]domain.SetCatalogue{cat1, cat2},
		domain.PriceSnapshot{},
		domain.NewJournal(),
		counterParams(),
	)
	if err == nil {
		t.Fatal("want error for duplicate card ID, got nil")
	}
	if !strings.Contains(err.Error(), "more than one catalogue") {
		t.Errorf("error %q does not mention duplicate catalogue", err.Error())
	}
}

// TestCounterNewInitialStock verifies that NewCounter stocks non-basic, non-foil
// cards at the configured quantity per rarity, and excludes basic lands and foil
// keys from the initial stock.
func TestCounterNewInitialStock(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(0)
	c, _ := newTestCounter(t, []domain.SetCatalogue{cat}, domain.PriceSnapshot{}, counterParams())

	// Commons (10 cards, indices 0–9): each stocked at 2 nonfoil copies.
	for i := range 10 {
		key := domain.CardKey{
			ID:     domain.CardID(fmt.Sprintf("synth-c-%04d", i)),
			Finish: domain.FinishNonFoil,
		}
		if got := c.Stock(key); got != 2 {
			t.Errorf("common %d nonfoil stock = %d, want 2", i, got)
		}
		// Foil keys are never in the initial stock.
		foil := domain.CardKey{ID: key.ID, Finish: domain.FinishFoil}
		if got := c.Stock(foil); got != 0 {
			t.Errorf("common %d foil initial stock = %d, want 0", i, got)
		}
	}

	// Basic lands (synth-b-0000, synth-b-0001): excluded regardless of finish.
	for i := range 2 {
		key := domain.CardKey{
			ID:     domain.CardID(fmt.Sprintf("synth-b-%04d", i)),
			Finish: domain.FinishNonFoil,
		}
		if got := c.Stock(key); got != 0 {
			t.Errorf("basic land %d stock = %d, want 0", i, got)
		}
	}

	// StockByRarity totals:
	//   common:   10 cards × 2 = 20
	//   uncommon: (4 synth-u-* + 4 synth-cs-u-*) × 1 = 8
	//   rare:     (2 synth-r-* + 2 synth-cs-r-*) × 1 = 4
	byRarity := c.StockByRarity()
	if got := byRarity[domain.RarityCommon]; got != 20 {
		t.Errorf("StockByRarity[common] = %d, want 20", got)
	}
	if got := byRarity[domain.RarityUncommon]; got != 8 {
		t.Errorf("StockByRarity[uncommon] = %d, want 8", got)
	}
	if got := byRarity[domain.RarityRare]; got != 4 {
		t.Errorf("StockByRarity[rare] = %d, want 4", got)
	}
}

// TestCounterBuyFromPlayer covers the player selling a card to the Counter.
func TestCounterBuyFromPlayer(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(0)
	prices := counterPrices()
	params := counterParams()

	t.Run("nonfoil: player is credited and stock increments", func(t *testing.T) {
		t.Parallel()
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
		before := c.Stock(key) // 1 from InitialStock{rare:1}

		if err := c.BuyFromPlayer(key); err != nil {
			t.Fatalf("BuyFromPlayer: %v", err)
		}
		// sell% of 200 cents at rate 100: floor(200×100×60/10000) = 120
		if got := j.Balance(domain.AccountPlayer); got != 120 {
			t.Errorf("AccountPlayer balance = %d, want 120", got)
		}
		if got := c.Stock(key); got != before+1 {
			t.Errorf("stock = %d, want %d", got, before+1)
		}
	})

	t.Run("foil: accepted and adds foil key to stock", func(t *testing.T) {
		t.Parallel()
		// synth-c-0000 has FinishNonFoil and FinishFoil (even index).
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishFoil}
		before := c.Stock(key) // 0: foil not in initial stock

		if err := c.BuyFromPlayer(key); err != nil {
			t.Fatalf("BuyFromPlayer foil: %v", err)
		}
		// sell% of 100 cents at rate 100: floor(100×100×60/10000) = 60
		if got := j.Balance(domain.AccountPlayer); got != 60 {
			t.Errorf("AccountPlayer balance = %d, want 60", got)
		}
		if got := c.Stock(key); got != before+1 {
			t.Errorf("foil stock = %d, want %d", got, before+1)
		}
	})

	t.Run("unknown card ID returns ErrUnknownCard", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "no-such-card", Finish: domain.FinishNonFoil}
		err := c.BuyFromPlayer(key)
		if !errors.Is(err, domain.ErrUnknownCard) {
			t.Errorf("BuyFromPlayer unknown ID: got %v, want ErrUnknownCard", err)
		}
	})

	t.Run("wrong finish returns ErrUnknownCard", func(t *testing.T) {
		t.Parallel()
		// synth-c-0001 has FinishNonFoil only (odd index).
		c, _ := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "synth-c-0001", Finish: domain.FinishFoil}
		err := c.BuyFromPlayer(key)
		if !errors.Is(err, domain.ErrUnknownCard) {
			t.Errorf("BuyFromPlayer wrong finish: got %v, want ErrUnknownCard", err)
		}
	})

	t.Run("a price under one coin pays one coin", func(t *testing.T) {
		t.Parallel()
		// 1 cent at rate=1, SellPct=60: floor(1×1×60/10000)=0, floored to one coin by ADR-0044.
		p := counterParams()
		p.CoinRate = 1
		subPrices := domain.PriceSnapshot{"synth-c-0000": 1}
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, subPrices, p)
		key := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishNonFoil}
		before := c.Stock(key)

		if err := c.BuyFromPlayer(key); err != nil {
			t.Fatalf("BuyFromPlayer one cent: %v", err)
		}
		if got := j.Balance(domain.AccountPlayer); got != 1 {
			t.Errorf("AccountPlayer balance = %d, want 1", got)
		}
		if got := c.Stock(key); got != before+1 {
			t.Errorf("stock = %d, want %d", got, before+1)
		}
	})

	t.Run("zero price (FallbackPrice=0) returns ErrSubCoin, ledger unchanged", func(t *testing.T) {
		t.Parallel()
		// synth-c-0002 is absent from the snapshot; FallbackPrice=0 → zero coins.
		p := counterParams()
		p.FallbackPrice = 0
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, domain.PriceSnapshot{}, p)
		key := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishNonFoil}

		err := c.BuyFromPlayer(key)
		if !errors.Is(err, domain.ErrSubCoin) {
			t.Errorf("BuyFromPlayer zero price: got %v, want ErrSubCoin", err)
		}
		if got := j.Balance(domain.AccountPlayer); got != 0 {
			t.Errorf("AccountPlayer balance after zero-price refusal = %d, want 0", got)
		}
	})
}

// TestCounterSellToPlayer covers the Counter selling a card to the player.
func TestCounterSellToPlayer(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(0)
	prices := counterPrices()
	params := counterParams()

	t.Run("happy path: player pays and stock decrements", func(t *testing.T) {
		t.Parallel()
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}

		// Fund the player: buy% of 200 cents at rate 100 = 240 coins.
		mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 500})
		before := c.Stock(key)

		if err := c.SellToPlayer(key); err != nil {
			t.Fatalf("SellToPlayer: %v", err)
		}
		// floor(200×100×120/10000) = 240 coins spent.
		if got := j.Balance(domain.AccountPlayer); got != 260 {
			t.Errorf("AccountPlayer balance = %d, want 260 (500-240)", got)
		}
		if got := c.Stock(key); got != before-1 {
			t.Errorf("stock = %d, want %d", got, before-1)
		}
	})

	t.Run("out of stock returns ErrOutOfStock, ledger unchanged", func(t *testing.T) {
		t.Parallel()
		// No initial stock for this card.
		p := counterParams()
		p.InitialStock = map[domain.Rarity]int{}
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, p)
		mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})

		key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
		err := c.SellToPlayer(key)
		if !errors.Is(err, domain.ErrOutOfStock) {
			t.Errorf("SellToPlayer out of stock: got %v, want ErrOutOfStock", err)
		}
		if got := j.Balance(domain.AccountPlayer); got != 1000 {
			t.Errorf("AccountPlayer balance changed on out-of-stock: got %d, want 1000", got)
		}
	})

	t.Run("unknown card ID returns ErrUnknownCard", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "no-such-card", Finish: domain.FinishNonFoil}
		err := c.SellToPlayer(key)
		if !errors.Is(err, domain.ErrUnknownCard) {
			t.Errorf("SellToPlayer unknown ID: got %v, want ErrUnknownCard", err)
		}
	})

	t.Run("wrong finish returns ErrUnknownCard", func(t *testing.T) {
		t.Parallel()
		// synth-c-0001 has FinishNonFoil only (odd index).
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})
		key := domain.CardKey{ID: "synth-c-0001", Finish: domain.FinishFoil}
		err := c.SellToPlayer(key)
		if !errors.Is(err, domain.ErrUnknownCard) {
			t.Errorf("SellToPlayer wrong finish: got %v, want ErrUnknownCard", err)
		}
	})

	t.Run("insufficient funds returns ErrInsufficientBalance, stock unchanged", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
		before := c.Stock(key)

		// Player has 0 coins — buy costs 240.
		err := c.SellToPlayer(key)
		if !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Errorf("SellToPlayer no funds: got %v, want ErrInsufficientBalance", err)
		}
		if got := c.Stock(key); got != before {
			t.Errorf("stock changed on ledger failure: got %d, want %d", got, before)
		}
	})

	t.Run("sub-coin price returns ErrSubCoin, stock and ledger unchanged", func(t *testing.T) {
		t.Parallel()
		// 1 cent at rate=1, BuyPct=120: floor(1×1×120/10000)=0 → ErrSubCoin.
		p := counterParams()
		p.CoinRate = 1
		subPrices := domain.PriceSnapshot{"synth-c-0000": 1}
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, subPrices, p)
		mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})
		key := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishNonFoil}
		before := c.Stock(key)

		err := c.SellToPlayer(key)
		if !errors.Is(err, domain.ErrSubCoin) {
			t.Errorf("SellToPlayer sub-coin: got %v, want ErrSubCoin", err)
		}
		if got := c.Stock(key); got != before {
			t.Errorf("stock changed on sub-coin refusal: got %d, want %d", got, before)
		}
		if got := j.Balance(domain.AccountPlayer); got != 1000 {
			t.Errorf("AccountPlayer balance changed on sub-coin refusal: got %d, want 1000", got)
		}
	})

	t.Run("zero price (FallbackPrice=0) returns ErrSubCoin, stock and ledger unchanged", func(t *testing.T) {
		t.Parallel()
		p := counterParams()
		p.FallbackPrice = 0
		// synth-c-0000 absent from snapshot → price=0 → ErrSubCoin.
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, domain.PriceSnapshot{}, p)
		mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})
		key := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishNonFoil}
		before := c.Stock(key)

		err := c.SellToPlayer(key)
		if !errors.Is(err, domain.ErrSubCoin) {
			t.Errorf("SellToPlayer zero price: got %v, want ErrSubCoin", err)
		}
		if got := c.Stock(key); got != before {
			t.Errorf("stock changed on zero-price refusal: got %d, want %d", got, before)
		}
		if got := j.Balance(domain.AccountPlayer); got != 1000 {
			t.Errorf("AccountPlayer balance changed on zero-price refusal: got %d, want 1000", got)
		}
	})

	t.Run("atomicity: ledger failure leaves stock untouched", func(t *testing.T) {
		t.Parallel()
		// Player has exactly 1 coin less than needed; the post will fail and
		// stock must remain at its pre-call value.
		c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)
		key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
		// buy costs floor(200×100×120/10000)=240; fund with 239.
		mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 239})
		before := c.Stock(key)

		err := c.SellToPlayer(key)
		if !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Fatalf("want ErrInsufficientBalance, got %v", err)
		}
		if got := c.Stock(key); got != before {
			t.Errorf("stock changed after failed post: got %d, want %d", got, before)
		}
		if got := j.Balance(domain.AccountPlayer); got != 239 {
			t.Errorf("AccountPlayer balance changed after failed post: got %d, want 239", got)
		}
	})
}

// TestCounterRoundTrip asserts that buying a card from the Counter and selling
// it back always results in a net coin loss, at rates 10, 25, 100 and 500.
// It also directly asserts the buy-at-least-twice-sell invariant.
func TestCounterRoundTrip(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(0)
	// 200 cents is large enough for both legs to succeed even at rate=10:
	//   sell: floor(200×10×60/10000)=12  buy: floor(200×10×120/10000)=24.
	const price = domain.USDCents(200)
	prices := domain.PriceSnapshot{"synth-r-0000": price}
	key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}

	tests := []struct {
		name string
		rate domain.CoinRate
	}{
		{"rate 10", 10},
		{"rate 25", 25},
		{"rate 100", 100},
		{"rate 500", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := counterParams()
			p.CoinRate = tt.rate
			p.InitialStock = map[domain.Rarity]int{domain.RarityRare: 2}

			c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, p)

			const funding = domain.Coins(10_000)
			mustPost(t, j, domain.Entry{
				From:   domain.AccountWelcomeBonus,
				To:     domain.AccountPlayer,
				Amount: funding,
			})

			// Player buys from Counter, then sells back.
			if err := c.SellToPlayer(key); err != nil {
				t.Fatalf("SellToPlayer at rate %d: %v", tt.rate, err)
			}
			if err := c.BuyFromPlayer(key); err != nil {
				t.Fatalf("BuyFromPlayer at rate %d: %v", tt.rate, err)
			}

			finalBalance := j.Balance(domain.AccountPlayer)
			if finalBalance >= funding {
				t.Errorf("rate %d: round trip did not lose value (initial=%d final=%d)",
					tt.rate, funding, finalBalance)
			}

			// Invariant: buy amount >= 2 × sell amount (ADR-0003).
			sellCoins, _ := domain.PriceToCoins(price, tt.rate, 60)
			buyCoins, _ := domain.PriceToCoins(price, tt.rate, 120)
			if buyCoins < 2*sellCoins {
				t.Errorf("rate %d: buy %d < 2 × sell %d (invariant violated)",
					tt.rate, buyCoins, sellCoins)
			}
			// Cross-check: net loss equals buy − sell.
			if got, want := funding-finalBalance, buyCoins-sellCoins; got != want {
				t.Errorf("rate %d: net loss = %d, want %d (buy=%d sell=%d)",
					tt.rate, got, want, buyCoins, sellCoins)
			}
		})
	}
}

// TestCounterStockByRarity verifies that StockByRarity sums copies by rarity
// and that a BuyFromPlayer of a foil is reflected in the rarity total.
func TestCounterStockByRarity(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(0)
	prices := counterPrices()
	params := counterParams()

	c, j := newTestCounter(t, []domain.SetCatalogue{cat}, prices, params)

	// Initial totals (same as TestCounterNewInitialStock).
	byRarity := c.StockByRarity()
	if got := byRarity[domain.RarityCommon]; got != 20 {
		t.Errorf("initial common = %d, want 20", got)
	}
	if got := byRarity[domain.RarityUncommon]; got != 8 {
		t.Errorf("initial uncommon = %d, want 8", got)
	}
	if got := byRarity[domain.RarityRare]; got != 4 {
		t.Errorf("initial rare = %d, want 4", got)
	}

	// BuyFromPlayer with a foil common (synth-c-0000, even index → has FinishFoil).
	foilKey := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishFoil}
	if err := c.BuyFromPlayer(foilKey); err != nil {
		t.Fatalf("BuyFromPlayer foil: %v", err)
	}
	// Player received sell% of 100 cents at rate 100 = 60 coins; discard the balance.
	_ = j.Balance(domain.AccountPlayer)

	// Common total should now be 21 (20 nonfoil + 1 foil).
	byRarityAfter := c.StockByRarity()
	if got := byRarityAfter[domain.RarityCommon]; got != 21 {
		t.Errorf("common after foil buy = %d, want 21", got)
	}
	// Other rarities unchanged.
	if got := byRarityAfter[domain.RarityUncommon]; got != 8 {
		t.Errorf("uncommon after foil buy = %d, want 8", got)
	}
	if got := byRarityAfter[domain.RarityRare]; got != 4 {
		t.Errorf("rare after foil buy = %d, want 4", got)
	}
}
