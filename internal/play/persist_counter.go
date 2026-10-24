package play

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// snapshotCounter is the economy as the save holds it, in an order that does not depend on the
// maps'. A game without a Counter writes back the Counter of its save. g.mu must be held.
func (g *Game) snapshotCounter() (*CounterSnapshot, error) {
	e := g.economy
	if e == nil {
		return g.carriedCounter, nil
	}
	state := e.counter.State()
	snap := &CounterSnapshot{
		SyncedAt: e.syncedAt,
		Balances: e.journal.Balances(),
		Tick:     state.Tick,
		Stock:    cardCounts(state.Stock),
	}
	shelf := make(map[domain.CardKey][]int, len(state.Shelf))
	for key, batches := range state.Shelf {
		for _, b := range batches {
			for range b.Qty {
				shelf[key] = append(shelf[key], b.Tick)
			}
		}
	}
	dict := newCardDict()
	snap.Shelved = encodeShelf(shelf, dict)
	for _, key := range slices.SortedFunc(maps.Keys(state.LastInStock), compareKeys) {
		snap.LastInStock = append(snap.LastInStock, LastInStock{ID: key.ID, Finish: key.Finish, At: state.LastInStock[key]})
	}
	if e.supply != nil {
		s := e.supply.State()
		snap.Supply = &SupplySnapshot{Credit: s.Credit, Reserve: s.Energy.Reserve, Savings: s.Energy.Savings, SavingsTick: s.Energy.SavingsTick, Next: s.Next}
		var err error
		if snap.Supply.Rand, err = streamSnapshot(e.supplyStream); err != nil {
			return nil, err
		}
	}
	if e.crowd != nil {
		var err error
		if snap.Crowd, err = snapshotCrowd(e, dict); err != nil {
			return nil, err
		}
	}
	if e.market != nil {
		snap.Market = snapshotMarket(e.market)
	}
	snap.CardIDs = dict.ids
	return snap, nil
}

// restoreCounter checks the Counter of snap and returns what puts it in the game, see
// restoreUnlocks: the economy is built anew on the streams of the save and set to what it holds.
// A part the save lacks, such as the crowd of a save from before the game had one, is the new
// game's; a part the game does not have is an error. g.mu must be held.
func (g *Game) restoreCounter(snap CounterSnapshot, collection *domain.Collection) (commit func(), err error) {
	old := g.economy
	if old == nil {
		return func() { g.carriedCounter = &snap }, nil
	}
	if snap.SyncedAt.IsZero() {
		return nil, errors.New("counter: no synced_at, so the days to catch up on are unknown")
	}
	supplySeed, crowdSeed := old.supplyStream.Seed(), old.crowdStream.Seed()
	if snap.Supply != nil {
		supplySeed = snap.Supply.Rand.Seed
	}
	if snap.Crowd != nil {
		crowdSeed = snap.Crowd.Rand.Seed
	}
	// The crowd draws who its NPCs are from the start of its stream, which then goes on from where
	// the save left it.
	e, err := g.newEconomy(old.rules, NewStream(supplySeed), NewStream(crowdSeed), collection)
	if err != nil {
		return nil, err
	}
	if snap.Supply != nil {
		if err := e.supplyStream.Restore(supplySeed, snap.Supply.Rand.State); err != nil {
			return nil, fmt.Errorf("counter: supply: %w", err)
		}
	}
	if err := e.journal.Restore(snap.Balances); err != nil {
		return nil, fmt.Errorf("counter: %w", err)
	}
	dict := readCardDict(snap.CardIDs)
	state, err := counterState(snap, dict)
	if err != nil {
		return nil, err
	}
	if err := e.counter.Restore(state); err != nil {
		return nil, err
	}
	if err := restoreSupply(e, snap.Supply); err != nil {
		return nil, err
	}
	reset, err := restoreCrowd(e, snap.Crowd, dict)
	if err != nil {
		return nil, err
	}
	// A crowd the save gave is warm; one that replaces the save's stays cold until the catch-up.
	e.cold = reset && e.market != nil
	if err := restoreMarket(e, snap.Market, reset); err != nil {
		return nil, err
	}
	// A save from the future, written while the clock was ahead, would stop the catch-up until the
	// clock caught up, so it counts from now, as the energy does.
	e.syncedAt = snap.SyncedAt
	if now := g.now(); e.syncedAt.After(now) {
		e.syncedAt = now
	}
	return func() { g.economy = e }, nil
}

func restoreSupply(e *economy, snap *SupplySnapshot) error {
	switch {
	case snap == nil:
		return nil
	case e.supply == nil:
		return errors.New("counter: the save's Counter opens boosters, and this game's does not")
	}
	return e.supply.Restore(domain.SupplyState{
		Credit: snap.Credit,
		Energy: domain.EnergyState{Reserve: snap.Reserve, Savings: snap.Savings, SavingsTick: snap.SavingsTick},
		Next:   snap.Next,
	})
}

// counterState is the domain's form of the Counter part of snap.
func counterState(snap CounterSnapshot, dict *cardDict) (domain.CounterState, error) {
	stock, err := cardMap(snap.Stock)
	if err != nil {
		return domain.CounterState{}, fmt.Errorf("counter: stock: %w", err)
	}
	s := domain.CounterState{
		Tick:        snap.Tick,
		Stock:       stock,
		Shelf:       make(map[domain.CardKey][]domain.ShelfBatch),
		LastInStock: make(map[domain.CardKey]time.Time, len(snap.LastInStock)),
	}
	if snap.Shelf != nil && snap.Shelved != "" {
		return domain.CounterState{}, errors.New("counter: the shelf is listed both by batch and compactly")
	}
	for _, c := range snap.Shelf {
		key := domain.CardKey{ID: c.ID, Finish: c.Finish}
		s.Shelf[key] = append(s.Shelf[key], domain.ShelfBatch{Tick: c.Tick, Qty: c.Quantity})
	}
	shelved, err := decodeShelf(snap.Shelved, dict)
	if err != nil {
		return domain.CounterState{}, fmt.Errorf("counter: shelf: %w", err)
	}
	for key, ticks := range shelved {
		for _, tick := range ticks {
			if batches := s.Shelf[key]; len(batches) > 0 && batches[len(batches)-1].Tick == tick {
				batches[len(batches)-1].Qty++
				continue
			}
			s.Shelf[key] = append(s.Shelf[key], domain.ShelfBatch{Tick: tick, Qty: 1})
		}
	}
	for _, c := range snap.LastInStock {
		s.LastInStock[domain.CardKey{ID: c.ID, Finish: c.Finish}] = c.At
	}
	return s, nil
}

// cardCounts lists counts by card, in card order.
func cardCounts(counts map[domain.CardKey]int) []CardCount {
	out := make([]CardCount, 0, len(counts))
	for _, key := range slices.SortedFunc(maps.Keys(counts), compareKeys) {
		out = append(out, CardCount{ID: key.ID, Finish: key.Finish, Quantity: counts[key]})
	}
	return out
}

// cardMap is the map of counts, which may not name a card twice.
func cardMap(counts []CardCount) (map[domain.CardKey]int, error) {
	out := make(map[domain.CardKey]int, len(counts))
	for _, c := range counts {
		key := domain.CardKey{ID: c.ID, Finish: c.Finish}
		if _, twice := out[key]; twice {
			return nil, fmt.Errorf("card %q finish %q is listed twice", c.ID, c.Finish)
		}
		out[key] = c.Quantity
	}
	return out, nil
}

func streamSnapshot(s *Stream) (OpenerSnapshot, error) {
	state, err := s.State()
	if err != nil {
		return OpenerSnapshot{}, err
	}
	return OpenerSnapshot{Seed: s.Seed(), State: state}, nil
}
