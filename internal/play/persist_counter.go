package play

import (
	"cmp"
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
	for _, key := range slices.SortedFunc(maps.Keys(state.Shelf), compareKeys) {
		for _, b := range state.Shelf[key] {
			snap.Shelf = append(snap.Shelf, ShelfCount{ID: key.ID, Finish: key.Finish, Tick: b.Tick, Quantity: b.Qty})
		}
	}
	for _, key := range slices.SortedFunc(maps.Keys(state.LastInStock), compareKeys) {
		snap.LastInStock = append(snap.LastInStock, LastInStock{ID: key.ID, Finish: key.Finish, At: state.LastInStock[key]})
	}
	var err error
	if e.supply != nil {
		s := e.supply.State()
		snap.Supply = &SupplySnapshot{Credit: s.Credit, Reserve: s.Energy.Reserve, Savings: s.Energy.Savings, SavingsTick: s.Energy.SavingsTick, Next: s.Next}
		if snap.Supply.Rand, err = streamSnapshot(e.supplyStream); err != nil {
			return nil, err
		}
	}
	if e.crowd != nil {
		s, err := e.crowd.State()
		if err != nil {
			return nil, err
		}
		snap.Crowd = &CrowdSnapshot{Tick: s.Tick, NPCs: make([]NPCSnapshot, len(s.NPCs))}
		for i, n := range s.NPCs {
			snap.Crowd.NPCs[i] = NPCSnapshot{Credit: n.Credit, Cards: cardCounts(n.Cards)}
		}
		if snap.Crowd.Rand, err = streamSnapshot(e.crowdStream); err != nil {
			return nil, err
		}
	}
	return snap, nil
}

// restoreCounter checks the Counter of snap and returns what puts it in the game, see
// restoreUnlocks: the economy is built anew on the streams of the save and set to what it holds.
// A part the save lacks, such as the crowd of a save from before the game had one, is the new
// game's; a part the game does not have is an error. g.mu must be held.
func (g *Game) restoreCounter(snap CounterSnapshot) (commit func(), err error) {
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
	e, err := g.newEconomy(old.rules, NewStream(supplySeed), NewStream(crowdSeed))
	if err != nil {
		return nil, err
	}
	if snap.Supply != nil {
		if err := e.supplyStream.Restore(supplySeed, snap.Supply.Rand.State); err != nil {
			return nil, fmt.Errorf("counter: supply: %w", err)
		}
	}
	if snap.Crowd != nil {
		if err := e.crowdStream.Restore(crowdSeed, snap.Crowd.Rand.State); err != nil {
			return nil, fmt.Errorf("counter: crowd: %w", err)
		}
	}
	if err := e.journal.Restore(snap.Balances); err != nil {
		return nil, fmt.Errorf("counter: %w", err)
	}
	state, err := counterState(snap)
	if err != nil {
		return nil, err
	}
	if err := e.counter.Restore(state); err != nil {
		return nil, err
	}
	if err := restoreSupply(e, snap.Supply); err != nil {
		return nil, err
	}
	if err := restoreCrowd(e, snap.Crowd); err != nil {
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

func restoreCrowd(e *economy, snap *CrowdSnapshot) error {
	switch {
	case snap == nil:
		return nil
	case e.crowd == nil:
		return errors.New("counter: the save has NPCs, and this game has none")
	}
	state := domain.CrowdState{Tick: snap.Tick, NPCs: make([]domain.NPCState, len(snap.NPCs))}
	for i, n := range snap.NPCs {
		cards, err := cardMap(n.Cards)
		if err != nil {
			return fmt.Errorf("counter: NPC %d: %w", i, err)
		}
		state.NPCs[i] = domain.NPCState{Credit: n.Credit, Cards: cards}
	}
	return e.crowd.Restore(state)
}

// counterState is the domain's form of the Counter part of snap.
func counterState(snap CounterSnapshot) (domain.CounterState, error) {
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
	for _, c := range snap.Shelf {
		key := domain.CardKey{ID: c.ID, Finish: c.Finish}
		s.Shelf[key] = append(s.Shelf[key], domain.ShelfBatch{Tick: c.Tick, Qty: c.Quantity})
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

func compareKeys(a, b domain.CardKey) int {
	return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Finish, b.Finish))
}

func streamSnapshot(s *Stream) (OpenerSnapshot, error) {
	state, err := s.State()
	if err != nil {
		return OpenerSnapshot{}, err
	}
	return OpenerSnapshot{Seed: s.Seed(), State: state}, nil
}
