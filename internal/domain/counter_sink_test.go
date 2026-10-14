package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// shelfParams returns the test params with a Counter whose resold cards vanish after the given
// number of days. Zero days makes the Counter a sink.
func shelfParams(days int) domain.CounterParams {
	p := counterParams()
	p.ShelfLife = domain.VanishAfterDays(days)
	return p
}

// advanceTicks moves the Counter's clock forward by n ticks.
func advanceTicks(c *domain.Counter, n int) {
	for range n {
		c.Tick()
	}
}

func TestCounterThatIsASinkPaysThePlayerAndKeepsNoStock(t *testing.T) {
	t.Parallel()

	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), shelfParams(0))
	key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	before := c.Stock(key)

	if err := c.BuyFromPlayer(key); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}

	if got := j.Balance(domain.AccountPlayer); got != 120 {
		t.Errorf("player balance = %d, want 120: the sale still pays", got)
	}
	if got := c.Stock(key); got != before {
		t.Errorf("stock = %d, want %d: the card vanished", got, before)
	}
}

func TestCounterThatIsASinkDiscardsNPCSurplus(t *testing.T) {
	t.Parallel()

	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), shelfParams(0))
	key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	before := c.Stock(key)

	if err := c.AcceptSurplus(key, 3); err != nil {
		t.Fatalf("AcceptSurplus: %v", err)
	}

	if got := c.Stock(key); got != before {
		t.Errorf("stock = %d, want %d: the surplus vanished", got, before)
	}
	if n := len(j.Entries()); n != 0 {
		t.Errorf("journal has %d entries, want none", n)
	}
}

func TestCounterThatIsASinkStillRejectsAnUnknownSurplusCard(t *testing.T) {
	t.Parallel()

	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), shelfParams(0))

	err := c.AcceptSurplus(domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, 1)

	if err == nil {
		t.Fatal("want an error for an unknown card, got nil")
	}
}

func TestCounterThatIsASinkDoesNotRestockACardBoughtThenSoldBack(t *testing.T) {
	t.Parallel()

	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), shelfParams(0))
	key := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 500})
	before := c.Stock(key)

	if err := c.SellToPlayer(key); err != nil {
		t.Fatalf("SellToPlayer: %v", err)
	}
	if err := c.BuyFromPlayer(key); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}

	if got, want := c.Stock(key), before-1; got != want {
		t.Errorf("stock = %d, want %d: the card the player sold back vanished", got, want)
	}
}
