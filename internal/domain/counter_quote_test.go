package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func TestCounterQuote(t *testing.T) {
	t.Parallel()

	prices := counterPrices()
	prices["synth-c-0003"] = 0
	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, prices, counterParams())

	tests := []struct {
		name    string
		key     domain.CardKey
		want    domain.Quote
		wantErr error
	}{
		// $2 at 100 coins a dollar: the Counter pays 60% and asks 120%.
		{name: "a priced card", key: domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}, want: domain.Quote{Pays: 120, Asks: 240}},
		// The fallback of 10 cents pays 6 coins and asks 12.
		{name: "a card the snapshot lacks", key: domain.CardKey{ID: "synth-c-0002", Finish: domain.FinishNonFoil}, want: domain.Quote{Pays: 6, Asks: 12}},
		{name: "a card priced at zero is not traded", key: domain.CardKey{ID: "synth-c-0003", Finish: domain.FinishNonFoil}, want: domain.Quote{}},
		{name: "an unknown card", key: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, wantErr: domain.ErrUnknownCard},
		{name: "a finish the card lacks", key: domain.CardKey{ID: "synth-c-0001", Finish: domain.FinishFoil}, wantErr: domain.ErrUnknownCard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.Quote(tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Quote error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Quote = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// The quote is what a trade moves: what the Counter pays for a copy and asks for one.
func TestCounterQuote_isWhatTheTradesMove(t *testing.T) {
	t.Parallel()
	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), counterParams())
	key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	q, err := c.Quote(key)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if err := c.BuyFromPlayer(key); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}
	if got := j.Balance(domain.AccountPlayer); got != q.Pays {
		t.Fatalf("paid %d, quote said %d", got, q.Pays)
	}
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: q.Asks})
	if err := c.SellToPlayer(key); err != nil {
		t.Fatalf("SellToPlayer: %v", err)
	}
	if got := j.Balance(domain.AccountPlayer); got != q.Pays {
		t.Errorf("after the purchase the player has %d, want %d: it cost the %d the quote asked", got, q.Pays, q.Asks)
	}
}

func TestCounterBuyLotFromPlayer(t *testing.T) {
	t.Parallel()

	rare := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	common := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishNonFoil}
	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), counterParams())
	rareBefore, commonBefore := c.Stock(rare), c.Stock(common)

	paid, err := c.BuyLotFromPlayer([]domain.TradeLine{{Key: rare, Qty: 2}, {Key: common, Qty: 3}})
	if err != nil {
		t.Fatalf("BuyLotFromPlayer: %v", err)
	}
	// 2 rares at 120 and 3 commons at 60.
	if paid != 420 || j.Balance(domain.AccountPlayer) != 420 {
		t.Errorf("paid %d, player has %d, want 420 both", paid, j.Balance(domain.AccountPlayer))
	}
	if got := len(j.Entries()); got != 1 {
		t.Errorf("entries = %d, want 1: a lot is one payment", got)
	}
	if c.Stock(rare) != rareBefore+2 || c.Stock(common) != commonBefore+3 {
		t.Errorf("stock rare %d common %d, want %d and %d", c.Stock(rare), c.Stock(common), rareBefore+2, commonBefore+3)
	}
}

// A lot goes through whole or not at all: a line the Counter refuses leaves the stock and the
// ledger as they were.
func TestCounterBuyLotFromPlayer_refusesTheWholeLot(t *testing.T) {
	t.Parallel()

	rare := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	prices := counterPrices()
	prices["synth-c-0003"] = 0
	treasury := counterParams()
	treasury.Treasury = true

	tests := []struct {
		name    string
		params  domain.CounterParams
		other   domain.TradeLine
		wantErr error
	}{
		{name: "an unknown card", params: counterParams(), other: domain.TradeLine{Key: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, Qty: 1}, wantErr: domain.ErrUnknownCard},
		{name: "a card worth nothing", params: counterParams(), other: domain.TradeLine{Key: domain.CardKey{ID: "synth-c-0003", Finish: domain.FinishNonFoil}, Qty: 1}, wantErr: domain.ErrSubCoin},
		{name: "no copy", params: counterParams(), other: domain.TradeLine{Key: rare, Qty: 0}, wantErr: nil},
		// The treasury holds 200: one rare at 120 fits, two do not.
		{name: "a treasury that cannot pay the whole lot", params: treasury, other: domain.TradeLine{Key: rare, Qty: 1}, wantErr: domain.ErrCounterBroke},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := domain.NewJournal(domain.AccountPlayer, domain.AccountCounterTreasury)
			c, err := domain.NewCounter([]domain.SetCatalogue{domaintest.NewCatalogue(0)}, prices, j, tt.params)
			if err != nil {
				t.Fatalf("NewCounter: %v", err)
			}
			if err := domain.FundTreasury(j, 200, "test"); err != nil {
				t.Fatalf("FundTreasury: %v", err)
			}
			before := c.Stock(rare)

			_, err = c.BuyLotFromPlayer([]domain.TradeLine{{Key: rare, Qty: 1}, tt.other})
			if err == nil {
				t.Fatal("BuyLotFromPlayer went through")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if c.Stock(rare) != before || j.Balance(domain.AccountPlayer) != 0 || len(j.Entries()) != 1 {
				t.Errorf("a refused lot changed something: stock %d (was %d), player %d, entries %d", c.Stock(rare), before, j.Balance(domain.AccountPlayer), len(j.Entries()))
			}
		})
	}
}
