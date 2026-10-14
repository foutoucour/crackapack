package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// newTreasuryCounter builds a Counter that pays from a treasury funded with initial coins. The
// journal guards the treasury, so that it can never go negative.
func newTreasuryCounter(t *testing.T, initial domain.Coins) (*domain.Counter, *domain.Journal) {
	t.Helper()
	j := domain.NewJournal(domain.AccountPlayer, domain.AccountCounterTreasury)
	params := counterParams()
	params.Treasury = true
	c, err := domain.NewCounter([]domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), j, params)
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	if initial > 0 {
		mustPost(t, j, domain.Entry{From: domain.AccountCounterFloat, To: domain.AccountCounterTreasury, Amount: initial})
	}
	return c, j
}

func TestCounterWithATreasuryPaysThePlayerFromIt(t *testing.T) {
	t.Parallel()
	c, j := newTreasuryCounter(t, 1000)

	if err := c.BuyFromPlayer(shelfRare()); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}

	if got := j.Balance(domain.AccountPlayer); got != 120 {
		t.Errorf("player balance = %d, want 120", got)
	}
	if got := j.Balance(domain.AccountCounterTreasury); got != 880 {
		t.Errorf("treasury = %d, want 880: the Counter paid from it", got)
	}
	if got := j.Balance(domain.AccountCounterSale); got != 0 {
		t.Errorf("counter_sale = %d, want 0: nothing is minted with a treasury", got)
	}
}

func TestCounterWithATreasuryRefusesASaleItCannotPay(t *testing.T) {
	t.Parallel()
	c, j := newTreasuryCounter(t, 100)
	before := c.Stock(shelfRare())

	err := c.BuyFromPlayer(shelfRare())

	if !errors.Is(err, domain.ErrCounterBroke) {
		t.Fatalf("error = %v, want ErrCounterBroke", err)
	}
	if got := j.Balance(domain.AccountPlayer); got != 0 {
		t.Errorf("player balance = %d, want 0: nothing was paid", got)
	}
	if got := c.Stock(shelfRare()); got != before {
		t.Errorf("stock = %d, want %d: the refused card was not taken", got, before)
	}
	if got := j.Balance(domain.AccountCounterTreasury); got != 100 {
		t.Errorf("treasury = %d, want 100", got)
	}
}

func TestCounterWithATreasuryIsPaidByThePlayer(t *testing.T) {
	t.Parallel()
	c, j := newTreasuryCounter(t, 0)
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 500})

	if err := c.SellToPlayer(shelfRare()); err != nil {
		t.Fatalf("SellToPlayer: %v", err)
	}

	if got := j.Balance(domain.AccountCounterTreasury); got != 240 {
		t.Errorf("treasury = %d, want 240: the player paid into it", got)
	}
	if got := j.Balance(domain.AccountCounterPurchase); got != 0 {
		t.Errorf("counter_purchase = %d, want 0: the coins stay in the treasury", got)
	}
}

func TestCounterTreasuryConservesMoney(t *testing.T) {
	t.Parallel()
	c, j := newTreasuryCounter(t, 1000)
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 500})

	for _, step := range []func(domain.CardKey) error{c.SellToPlayer, c.BuyFromPlayer, c.BuyFromPlayer} {
		if err := step(shelfRare()); err != nil {
			t.Fatalf("trade: %v", err)
		}
	}

	if got := j.SumAllBalances(); got != 0 {
		t.Errorf("balances sum to %d, want 0", got)
	}
	if got := j.Balance(domain.AccountCounterTreasury); got < 0 {
		t.Errorf("treasury = %d, want it never negative", got)
	}
}
