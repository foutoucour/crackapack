package domain

import (
	"fmt"
	"maps"
	"time"
)

// CounterState is what a Counter holds that its catalogues and parameters do not give, so that a
// save can keep it and Restore can bring it back.
type CounterState struct {
	// Tick is the ticks the Counter has lived, which the shelf batches are stamped with.
	Tick int
	// Stock is the copies held outside a limited shelf, for the cards whose count differs from
	// the initial stock only: a Counter that stocks every printing of a hundred sets would
	// otherwise list them all.
	Stock map[CardKey]int
	// Shelf is the copies on a limited shelf, by card, in arrival order, see ShelfLife.
	Shelf map[CardKey][]ShelfBatch
	// LastInStock is, for the cards out of stock that the Counter held, the instant the last copy
	// left, see WithLastInStock.
	LastInStock map[CardKey]time.Time
}

// ShelfBatch is the copies of a card that arrived on a limited shelf at the same tick.
type ShelfBatch struct {
	Tick int
	Qty  int
}

// State returns what the Counter holds.
func (c *Counter) State() CounterState {
	s := CounterState{Tick: c.tick, Stock: make(map[CardKey]int), Shelf: make(map[CardKey][]ShelfBatch, len(c.resold))}
	for key, qty := range c.stock {
		if qty != c.initialQuantity(key) {
			s.Stock[key] = qty
		}
	}
	for key, batches := range c.resold {
		out := make([]ShelfBatch, len(batches))
		for i, b := range batches {
			out[i] = ShelfBatch{Tick: b.tick, Qty: b.qty}
		}
		s.Shelf[key] = out
	}
	if c.lastSeen != nil {
		s.LastInStock = maps.Clone(c.lastSeen)
	}
	return s
}

// Restore sets the Counter to s, which State returned, on top of the initial stock. A card the
// Counter does not know is kept as it is, inert, so that a save played on fewer sets keeps the
// stock of the others. It returns an error, and changes nothing, when s cannot be the state of
// this Counter: a negative tick or count, a shelf batch that is empty, out of arrival order or
// stamped after the tick, a batch on a shelf that is not limited, or a zero instant.
func (c *Counter) Restore(s CounterState) error {
	if s.Tick < 0 {
		return fmt.Errorf("counter: tick %d is negative", s.Tick)
	}
	stock := c.initialStock()
	for key, qty := range s.Stock {
		if qty < 0 {
			return fmt.Errorf("counter: card %q finish %q has %d copies", key.ID, key.Finish, qty)
		}
		stock[key] = qty
	}
	life := c.params.ShelfLife
	resold := make(map[CardKey][]shelfBatch, len(s.Shelf))
	for key, batches := range s.Shelf {
		if len(batches) == 0 {
			continue
		}
		if !life.Limited || life.Days == 0 {
			return fmt.Errorf("counter: card %q finish %q is on a limited shelf, and this Counter's is not", key.ID, key.Finish)
		}
		out := make([]shelfBatch, len(batches))
		for i, b := range batches {
			switch {
			case b.Qty <= 0:
				return fmt.Errorf("counter: card %q finish %q has a shelf batch of %d copies", key.ID, key.Finish, b.Qty)
			case b.Tick < 0 || b.Tick > s.Tick:
				return fmt.Errorf("counter: card %q finish %q has a shelf batch at tick %d, outside 0..%d", key.ID, key.Finish, b.Tick, s.Tick)
			case i > 0 && b.Tick < batches[i-1].Tick:
				return fmt.Errorf("counter: card %q finish %q has its shelf batches out of arrival order", key.ID, key.Finish)
			}
			out[i] = shelfBatch{tick: b.Tick, qty: b.Qty}
		}
		resold[key] = out
	}
	for key, at := range s.LastInStock {
		if at.IsZero() {
			return fmt.Errorf("counter: card %q finish %q was last in stock at the zero instant", key.ID, key.Finish)
		}
	}
	c.tick, c.stock, c.resold = s.Tick, stock, resold
	if c.lastSeen != nil {
		c.lastSeen = maps.Clone(s.LastInStock)
		if c.lastSeen == nil {
			c.lastSeen = make(map[CardKey]time.Time)
		}
	}
	return nil
}

// LastInStock returns the last instant the Counter held a copy of key, and false when it never
// did or does not remember, see WithLastInStock. For a card in stock it is now.
func (c *Counter) LastInStock(key CardKey) (time.Time, bool) {
	if c.lastSeen == nil {
		return time.Time{}, false
	}
	if c.Stock(key) > 0 {
		return c.now(), true
	}
	at, ok := c.lastSeen[key]
	return at, ok
}
