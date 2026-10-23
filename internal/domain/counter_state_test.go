package domain_test

import (
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// testInstant is a clock a test moves by hand, for the Counter's memory of its shelf.
type testInstant struct{ now time.Time }

func (c *testInstant) Now() time.Time { return c.now }

func newRememberingCounter(t *testing.T, params domain.CounterParams, clock *testInstant) (*domain.Counter, *domain.Journal) {
	t.Helper()
	j := domain.NewJournal()
	c, err := domain.NewCounter([]domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), j, params, domain.WithLastInStock(clock.Now))
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 100_000})
	return c, j
}

func TestCounterLastInStock(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	rare := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	foil := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishFoil}

	t.Run("a card never on the shelf", func(t *testing.T) {
		t.Parallel()
		c, _ := newRememberingCounter(t, counterParams(), &testInstant{now: start})
		if at, ok := c.LastInStock(foil); ok {
			t.Errorf("LastInStock of a foil never stocked = %v, true", at)
		}
	})

	t.Run("a card in stock is on the shelf now", func(t *testing.T) {
		t.Parallel()
		clock := &testInstant{now: start}
		c, _ := newRememberingCounter(t, counterParams(), clock)
		clock.now = start.Add(time.Hour)
		if at, ok := c.LastInStock(rare); !ok || !at.Equal(clock.now) {
			t.Errorf("LastInStock of a card in stock = %v, %v, want now", at, ok)
		}
	})

	t.Run("the last copy bought", func(t *testing.T) {
		t.Parallel()
		clock := &testInstant{now: start}
		c, _ := newRememberingCounter(t, counterParams(), clock)
		soldOut := start.Add(5 * time.Minute)
		clock.now = soldOut
		if err := c.SellToPlayer(rare); err != nil {
			t.Fatalf("SellToPlayer: %v", err)
		}
		clock.now = start.Add(48 * time.Hour)
		if at, ok := c.LastInStock(rare); !ok || !at.Equal(soldOut) {
			t.Errorf("LastInStock = %v, %v, want %v, the instant the last copy left", at, ok, soldOut)
		}
	})

	t.Run("a card back in stock and gone again", func(t *testing.T) {
		t.Parallel()
		clock := &testInstant{now: start}
		c, _ := newRememberingCounter(t, counterParams(), clock)
		if err := c.SellToPlayer(rare); err != nil {
			t.Fatalf("SellToPlayer: %v", err)
		}
		clock.now = start.Add(time.Hour)
		if err := c.BuyFromPlayer(rare); err != nil {
			t.Fatalf("BuyFromPlayer: %v", err)
		}
		again := start.Add(2 * time.Hour)
		clock.now = again
		if err := c.SellToPlayer(rare); err != nil {
			t.Fatalf("SellToPlayer: %v", err)
		}
		if at, ok := c.LastInStock(rare); !ok || !at.Equal(again) {
			t.Errorf("LastInStock = %v, %v, want %v", at, ok, again)
		}
	})

	t.Run("a card whose shelf life ended", func(t *testing.T) {
		t.Parallel()
		clock := &testInstant{now: start}
		c, _ := newRememberingCounter(t, emptyShelfParams(1), clock)
		if err := c.AcceptSurplus(rare, 1); err != nil {
			t.Fatalf("AcceptSurplus: %v", err)
		}
		advanceTicks(c, domain.TicksPerDay-1)
		vanished := start.Add(24 * time.Hour)
		clock.now = vanished
		advanceTicks(c, 1)
		clock.now = vanished.Add(time.Hour)
		if at, ok := c.LastInStock(rare); !ok || !at.Equal(vanished) {
			t.Errorf("LastInStock = %v, %v, want %v, the tick it vanished", at, ok, vanished)
		}
	})

	t.Run("a Counter that does not remember", func(t *testing.T) {
		t.Parallel()
		c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), counterParams())
		mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})
		if err := c.SellToPlayer(rare); err != nil {
			t.Fatalf("SellToPlayer: %v", err)
		}
		if _, ok := c.LastInStock(rare); ok {
			t.Error("a Counter built without WithLastInStock remembered a card")
		}
	})
}

// A Counter restored from the state of another holds the same stock, ages it the same way and
// remembers the same instants.
func TestCounterRestore_continuesAsTheCounterItCameFrom(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	cat := domaintest.NewCatalogue(0)
	params := shelfParams(1)
	clock := &testInstant{now: start}
	a, _ := newRememberingCounter(t, params, clock)
	rare := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	common := domain.CardKey{ID: "synth-c-0000", Finish: domain.FinishFoil}
	if err := a.SellToPlayer(rare); err != nil {
		t.Fatalf("SellToPlayer: %v", err)
	}
	advanceTicks(a, 30)
	if err := a.AcceptSurplus(common, 2); err != nil {
		t.Fatalf("AcceptSurplus: %v", err)
	}
	advanceTicks(a, 10)
	if err := a.BuyFromPlayer(common); err != nil {
		t.Fatalf("BuyFromPlayer: %v", err)
	}

	state := a.State()
	b, _ := newRememberingCounter(t, params, clock)
	if err := b.Restore(state); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	for step := range 3 {
		for id, card := range cat.Cards {
			for _, f := range card.Finishes {
				key := domain.CardKey{ID: id, Finish: f}
				if a.Stock(key) != b.Stock(key) {
					t.Errorf("step %d: stock of %v = %d restored, %d original", step, key, b.Stock(key), a.Stock(key))
				}
				atA, okA := a.LastInStock(key)
				atB, okB := b.LastInStock(key)
				if okA != okB || !atA.Equal(atB) {
					t.Errorf("step %d: LastInStock of %v = %v %v restored, %v %v original", step, key, atB, okB, atA, okA)
				}
			}
		}
		advanceTicks(a, domain.TicksPerDay/2)
		advanceTicks(b, domain.TicksPerDay/2)
	}
}

// The state lists only the cards whose stock differs from the initial stock, so that a save of a
// Counter that stocks every printing of a hundred sets stays small.
func TestCounterState_leavesOutTheCardsAtTheirInitialStock(t *testing.T) {
	t.Parallel()
	c, j := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), counterParams())
	mustPost(t, j, domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 1000})
	if got := len(c.State().Stock); got != 0 {
		t.Errorf("a new Counter's state lists %d cards, want 0", got)
	}
	rare := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	if err := c.SellToPlayer(rare); err != nil {
		t.Fatalf("SellToPlayer: %v", err)
	}
	if got := c.State().Stock; len(got) != 1 || got[rare] != 0 {
		t.Errorf("state stock = %v, want only %v at 0", got, rare)
	}
}

func TestCounterRestore_refusesAStateThatCannotBe(t *testing.T) {
	t.Parallel()

	rare := domain.CardKey{ID: "synth-r-0000", Finish: domain.FinishNonFoil}
	tests := []struct {
		name   string
		params domain.CounterParams
		state  domain.CounterState
	}{
		{name: "a negative tick", params: shelfParams(1), state: domain.CounterState{Tick: -1}},
		{name: "a negative stock", params: counterParams(), state: domain.CounterState{Stock: map[domain.CardKey]int{rare: -1}}},
		{name: "a batch from after the tick", params: shelfParams(1), state: domain.CounterState{Tick: 5, Shelf: map[domain.CardKey][]domain.ShelfBatch{rare: {{Tick: 6, Qty: 1}}}}},
		{name: "an empty batch", params: shelfParams(1), state: domain.CounterState{Tick: 5, Shelf: map[domain.CardKey][]domain.ShelfBatch{rare: {{Tick: 1, Qty: 0}}}}},
		{name: "batches out of order", params: shelfParams(1), state: domain.CounterState{Tick: 5, Shelf: map[domain.CardKey][]domain.ShelfBatch{rare: {{Tick: 3, Qty: 1}, {Tick: 2, Qty: 1}}}}},
		{name: "a batch on a shelf that keeps cards forever", params: counterParams(), state: domain.CounterState{Tick: 5, Shelf: map[domain.CardKey][]domain.ShelfBatch{rare: {{Tick: 1, Qty: 1}}}}},
		{name: "a zero instant", params: counterParams(), state: domain.CounterState{LastInStock: map[domain.CardKey]time.Time{rare: {}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := newTestCounter(t, []domain.SetCatalogue{domaintest.NewCatalogue(0)}, counterPrices(), tt.params)
			before := c.Stock(rare)
			if err := c.Restore(tt.state); err == nil {
				t.Fatal("Restore went through")
			}
			if c.Stock(rare) != before {
				t.Errorf("a refused restore moved the stock from %d to %d", before, c.Stock(rare))
			}
		})
	}
}
