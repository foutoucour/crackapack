package domain_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// shelfRare is the rare card the shelf tests sell and buy.
func shelfRare() domain.CardKey {
	return domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
}

// emptyShelfParams is shelfParams with no initial stock, so that only resold cards are on the shelf.
func emptyShelfParams(days int) domain.CounterParams {
	p := shelfParams(days)
	p.InitialStock = map[domain.Rarity]int{}
	return p
}

func TestCounterResoldCardStaysOnTheShelfUntilItsLifeEnds(t *testing.T) {
	t.Parallel()

	const days = 2
	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), emptyShelfParams(days))
	if err := c.BuyFromPlayer(shelfRare()); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}

	advanceTicks(c, days*domain.TicksPerDay-1)
	if got := c.Stock(shelfRare()); got != 1 {
		t.Errorf("stock one tick before the end of its life = %d, want 1", got)
	}

	advanceTicks(c, 1)
	if got := c.Stock(shelfRare()); got != 0 {
		t.Errorf("stock when its life ends = %d, want 0: the card vanished", got)
	}
}

func TestCounterSurplusAgesOnTheShelfLikeAResoldCard(t *testing.T) {
	t.Parallel()

	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), emptyShelfParams(1))
	if err := c.AcceptSurplus(shelfRare(), 3); err != nil {
		t.Fatalf("AcceptSurplus: %v", err)
	}

	advanceTicks(c, domain.TicksPerDay-1)
	if got := c.Stock(shelfRare()); got != 3 {
		t.Errorf("stock before the end of the life = %d, want 3", got)
	}
	advanceTicks(c, 1)
	if got := c.Stock(shelfRare()); got != 0 {
		t.Errorf("stock at the end of the life = %d, want 0", got)
	}
}

func TestCounterSellsTheOldestResoldCopyFirst(t *testing.T) {
	t.Parallel()

	const days = 1
	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), emptyShelfParams(days))
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})
	if err := c.BuyFromPlayer(shelfRare()); err != nil {
		t.Fatalf("first BuyFromPlayer: %v", err)
	}
	advanceTicks(c, 50)
	if err := c.BuyFromPlayer(shelfRare()); err != nil {
		t.Fatalf("second BuyFromPlayer: %v", err)
	}

	if err := c.SellToPlayer(shelfRare()); err != nil {
		t.Fatalf("SellToPlayer: %v", err)
	}
	// The first copy would be gone by now, the second still has 50 ticks to go.
	advanceTicks(c, domain.TicksPerDay-50)

	if got := c.Stock(shelfRare()); got != 1 {
		t.Errorf("stock = %d, want 1: the younger copy should have been kept", got)
	}
}

func TestCounterInitialStockNeverExpires(t *testing.T) {
	t.Parallel()

	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), shelfParams(1))
	before := c.Stock(shelfRare())
	if before == 0 {
		t.Fatal("fixture has no initial stock of the rare")
	}

	advanceTicks(c, 5*domain.TicksPerDay)

	if got := c.Stock(shelfRare()); got != before {
		t.Errorf("stock = %d, want %d: initial stock is not a sold card", got, before)
	}
}

func TestCounterStockByRarityCountsResoldCardsUntilTheyExpire(t *testing.T) {
	t.Parallel()

	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), emptyShelfParams(1))
	if err := c.BuyFromPlayer(shelfRare()); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}
	if got := c.StockByRarity()[domain.RarityRare]; got != 1 {
		t.Errorf("rare stock = %d, want 1", got)
	}

	advanceTicks(c, domain.TicksPerDay)

	if got := c.StockByRarity()[domain.RarityRare]; got != 0 {
		t.Errorf("rare stock after expiry = %d, want 0", got)
	}
}

func TestCounterKeepsResoldCardsForeverByDefault(t *testing.T) {
	t.Parallel()

	p := emptyShelfParams(0)
	p.ShelfLife = domain.ShelfLife{}
	c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), p)
	if err := c.BuyFromPlayer(shelfRare()); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}

	advanceTicks(c, 30*domain.TicksPerDay)

	if got := c.Stock(shelfRare()); got != 1 {
		t.Errorf("stock = %d, want 1: the zero ShelfLife keeps cards forever", got)
	}
	if forever := domain.ForeverOnShelf(); forever != (domain.ShelfLife{}) {
		t.Errorf("ForeverOnShelf() = %+v, want the zero value", forever)
	}
}

func TestCounterNewRejectsANegativeShelfLife(t *testing.T) {
	t.Parallel()

	p := counterParams()
	p.ShelfLife = domain.ShelfLife{Limited: true, Days: -1}

	_, err := domain.NewCounter([]domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), domain.NewJournal(), p)

	if err == nil || !strings.Contains(err.Error(), "ShelfLife") {
		t.Errorf("error = %v, want one naming ShelfLife", err)
	}
}
