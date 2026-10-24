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

// Saves is what a [Game] needs to keep its state between runs.
type Saves struct {
	Store Store
	// Stream is the random source of the game's opener: the game saves where it stands and, when it
	// loads a save, moves it there.
	Stream *Stream
	// Failed is told of each save that fails, so that the composition root can log it. It is nil
	// when nobody wants to know, and it must not call the game.
	Failed func(error)
}

// Persist loads the game from saves.Store and has the game saved there after every action that
// changes it. A game that was never saved is a new one, and is saved at once. What a save holds
// replaces the state NewGame gave, and a part that the save lacks keeps NewGame's value, so that a
// save written before the game had that part still loads. A save that cannot be read, or that
// does not fit this game, is an error and the game is left as NewGame made it, and nothing is
// written over the save.
//
// A game loaded from a save catches its Counter up on the days gone by, see OpenCounter. An error
// there is a catalogue that does not hold a card its boosters draw.
//
// A save that fails later does not stop the game: SaveError reports it and Failed is told.
// saves.Stream must be the source that the game's opener draws from. Persist is called once, before
// the game is used.
func (g *Game) Persist(saves Saves) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	snap, found, err := saves.Store.Load()
	if err != nil {
		return err
	}
	if found {
		if err := g.restore(snap, saves.Stream); err != nil {
			return fmt.Errorf("play: the save does not fit this game: %w", err)
		}
	}
	g.saves = saves
	if !found {
		if g.economy != nil {
			if err := g.economy.warmUp(); err != nil {
				return err
			}
		}
		g.save()
		return nil
	}
	// The days the game was closed are caught up now rather than at the first page.
	if g.economy != nil {
		if err := g.economy.catchUp(); err != nil {
			return err
		}
	}
	g.unlockCaughtUp()
	return nil
}

// SaveError is why the latest save failed, and nil when it went through or the game keeps no save.
// A client shows it, since a restart would lose what was played since the last good save.
func (g *Game) SaveError() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.saveErr
}

// save writes the game to the store, if it has one, and remembers whether it went through. g.mu
// must be held. Every method that changes the game calls it last, once nothing can fail any more.
func (g *Game) save() {
	if g.saves.Store == nil {
		return
	}
	snap, err := g.snapshot()
	if err == nil {
		err = g.saves.Store.Save(snap)
	}
	g.saveErr = err
	if err != nil && g.saves.Failed != nil {
		g.saves.Failed(err)
	}
}

// snapshot is the state of the game, as it stood when the energy was last brought up to date.
// g.mu must be held.
func (g *Game) snapshot() (Snapshot, error) {
	state, err := g.saves.Stream.State()
	if err != nil {
		return Snapshot{}, err
	}
	energy := g.energy.State()
	snap := Snapshot{
		Version: SaveVersion,
		Opener:  &OpenerSnapshot{Seed: g.saves.Stream.Seed(), State: state},
		// The map is never nil, so that a game that has spent every welcome booster is saved as such
		// and not as a game that never had any.
		Welcome:    make(map[string]int, len(g.packs)),
		Collection: snapshotCollection(g.collection),
		Energy: &EnergySnapshot{
			Reserve: energy.Reserve, Savings: energy.Savings, SavingsTick: energy.SavingsTick,
			SyncedAt: g.lastTick,
		},
	}
	for code, n := range g.packs {
		snap.Welcome[code] = n
	}
	if g.last != nil {
		snap.LastOpening = snapshotOpening(*g.last)
	}
	if snap.Unlocks, err = g.snapshotUnlocks(); err != nil {
		return Snapshot{}, err
	}
	if snap.Counter, err = g.snapshotCounter(); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// snapshotUnlocks lists the unlocks in code order, so that a save does not depend on the maps'.
// g.mu must be held.
func (g *Game) snapshotUnlocks() (*UnlocksSnapshot, error) {
	s := g.staging
	if s == nil {
		return g.carried, nil
	}
	state, err := s.stream.State()
	if err != nil {
		return nil, err
	}
	return &UnlocksSnapshot{
		Unlocked:  slices.Sorted(maps.Keys(s.unlocked)),
		Triggered: slices.Sorted(maps.Keys(s.triggered)),
		Rand:      &OpenerSnapshot{Seed: s.stream.Seed(), State: state},
	}, nil
}

// snapshotCollection lists the copies held, in an order that does not depend on the map's.
func snapshotCollection(c *domain.Collection) []CardCount {
	held := c.Snapshot()
	out := make([]CardCount, 0, len(held))
	for key, n := range held {
		out = append(out, CardCount{ID: key.ID, Finish: key.Finish, Quantity: n})
	}
	slices.SortFunc(out, func(a, b CardCount) int {
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Finish, b.Finish))
	})
	return out
}

func snapshotOpening(o Opening) *OpeningSnapshot {
	out := &OpeningSnapshot{SetCode: o.SetCode, Cards: make([]OpenedCard, len(o.Cards))}
	for i, c := range o.Cards {
		out.Cards[i] = OpenedCard{ID: c.Card.ID, Finish: c.Finish, New: c.New}
	}
	for _, s := range o.Unlocked {
		out.Unlocked = append(out.Unlocked, s.Code)
	}
	return out
}

// restore replaces the state of the game by the parts of snap that are there. It changes nothing
// when it returns an error. g.mu must be held.
func (g *Game) restore(snap Snapshot, stream *Stream) error {
	collection := g.collection
	if snap.Collection != nil {
		var err error
		if collection, err = restoreCollection(snap.Collection); err != nil {
			return err
		}
	}
	packs := g.packs
	if snap.Welcome != nil {
		var err error
		if packs, err = restorePacks(snap.Welcome); err != nil {
			return err
		}
	}
	energy, lastTick := g.energy, g.lastTick
	if snap.Energy != nil {
		var err error
		if energy, lastTick, err = g.restoreEnergy(*snap.Energy); err != nil {
			return err
		}
	}
	commitUnlocks := func() {}
	if snap.Unlocks != nil {
		var err error
		if commitUnlocks, err = g.restoreUnlocks(*snap.Unlocks); err != nil {
			return err
		}
	}
	commitCounter := func() {}
	if snap.Counter != nil {
		var err error
		if commitCounter, err = g.restoreCounter(*snap.Counter, collection); err != nil {
			return err
		}
	} else if e := g.economy; e != nil && e.market != nil {
		// A save from before the Counter keeps the economy of the new game, whose market must
		// trade with the collection of the save.
		m, err := e.newMarket(collection)
		if err != nil {
			return err
		}
		commitCounter = func() { e.market = m }
	}
	if snap.Opener != nil {
		if err := stream.Restore(snap.Opener.Seed, snap.Opener.State); err != nil {
			return err
		}
	}
	g.collection, g.packs, g.energy, g.lastTick = collection, packs, energy, lastTick
	commitUnlocks()
	commitCounter()
	g.openCrowdSets(g.economy)
	if snap.LastOpening != nil {
		g.last = g.restoreOpening(*snap.LastOpening)
	}
	return nil
}

// restoreUnlocks checks the unlocks of snap and returns what puts them in the game: they add to
// the first stage, which stays open whatever snap holds. A set that the game does not play is kept,
// as restorePacks does. A game without stages keeps snap as it is. Nothing changes until commit is
// called, so that a save refused later leaves the game as it was. g.mu must be held.
func (g *Game) restoreUnlocks(snap UnlocksSnapshot) (commit func(), err error) {
	s := g.staging
	if s == nil {
		return func() { g.carried = &snap }, nil
	}
	stream := s.stream
	if snap.Rand != nil {
		stream = NewStream(snap.Rand.Seed)
		if err := stream.Restore(snap.Rand.Seed, snap.Rand.State); err != nil {
			return nil, fmt.Errorf("unlocks: %w", err)
		}
	}
	return func() {
		*s.stream = *stream
		for _, code := range snap.Unlocked {
			s.unlocked[code] = true
		}
		for _, code := range snap.Triggered {
			s.triggered[code] = true
		}
	}, nil
}

func restoreCollection(counts []CardCount) (*domain.Collection, error) {
	c := domain.NewCollection()
	for _, n := range counts {
		if n.Finish != domain.FinishNonFoil && n.Finish != domain.FinishFoil {
			return nil, fmt.Errorf("collection: card %q has the unknown finish %q", n.ID, n.Finish)
		}
		if err := c.Add(domain.CardKey{ID: n.ID, Finish: n.Finish}, n.Quantity); err != nil {
			return nil, fmt.Errorf("collection: card %q: %w", n.ID, err)
		}
	}
	return c, nil
}

// restorePacks keeps the boosters of a set that the game does not play, so that it has them again
// when the set is back.
func restorePacks(welcome map[string]int) (map[string]int, error) {
	packs := make(map[string]int, len(welcome))
	for code, n := range welcome {
		if n < 0 {
			return nil, fmt.Errorf("welcome boosters: set %q has %d", code, n)
		}
		packs[code] = n
	}
	return packs, nil
}

func (g *Game) restoreEnergy(s EnergySnapshot) (*domain.Energy, time.Time, error) {
	if s.SyncedAt.IsZero() {
		return nil, time.Time{}, errors.New("energy: no synced_at, so the time the game was closed is unknown")
	}
	energy, err := domain.NewEnergy(g.rules.Energy)
	if err != nil {
		return nil, time.Time{}, err
	}
	if err := energy.Restore(domain.EnergyState{Reserve: s.Reserve, Savings: s.Savings, SavingsTick: s.SavingsTick}); err != nil {
		return nil, time.Time{}, err
	}
	// A save from the future, written while the clock was ahead, would stop the recharge until the
	// clock caught up, so it counts from now.
	syncedAt := s.SyncedAt
	if now := g.now(); syncedAt.After(now) {
		syncedAt = now
	}
	return energy, syncedAt, nil
}

// restoreOpening resolves the cards of a saved opening again. An opening of a set the game does not
// play, or of a card in no catalogue, is dropped: the pack page then has nothing to show, which is
// all it costs.
func (g *Game) restoreOpening(s OpeningSnapshot) *Opening {
	cat, ok := g.set(s.SetCode)
	if !ok {
		return nil
	}
	opening := Opening{SetCode: cat.Code, SetName: cat.Name, IconCode: cat.IconCode, Cards: make([]RevealedCard, len(s.Cards))}
	for i, c := range s.Cards {
		card, ok := g.cards[c.ID]
		if !ok {
			return nil
		}
		opening.Cards[i] = RevealedCard{Card: card, Finish: c.Finish, New: c.New}
	}
	// A set the game no longer plays is left out of the message.
	for _, code := range s.Unlocked {
		if unlocked, ok := g.set(code); ok {
			opening.Unlocked = append(opening.Unlocked, UnlockedSet{Code: code, Name: unlocked.Name})
		}
	}
	return &opening
}
