// Package play is the player's interactive session: the boosters the player owns, opening them and
// the collection they fill. It is what a client such as the web UI calls, so that the rules stay in
// Go and the client only renders what it gets back.
package play

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// WelcomeBoosters is the number of boosters a new player receives, split across the sets as
// docs/core-features.md#installation-and-saves describes.
const WelcomeBoosters = 10

// ErrUnknownSet is returned by Open when the set code is not one of the game's sets.
var ErrUnknownSet = errors.New("play: unknown set")

// Pack is what the player can open of one set: the welcome boosters left, which are free, and what
// one more costs in energy once they are gone.
type Pack struct {
	SetCode string
	SetName string
	// Count is the welcome boosters left. They are opened before any energy is spent.
	Count int
	// Cost is the energy a booster of the set costs once no welcome booster is left.
	Cost int
	// Wait is the time until the player can open a booster of the set: 0 while a welcome booster is
	// left or the energy can pay for one.
	Wait time.Duration
}

// RevealedCard is one card of an opened booster.
type RevealedCard struct {
	Card   domain.Card
	Finish domain.Finish
	// New is true when the player never held a copy of this printing before, in either finish.
	New bool
}

// Opening is the result of opening one booster, in reveal order with the rare last.
type Opening struct {
	SetCode string
	SetName string
	Cards   []RevealedCard
}

// Game is one player's session. It is safe for concurrent use, since an HTTP server calls it from
// several goroutines and the opener's random generator is not.
type Game struct {
	mu         sync.Mutex
	sets       []domain.SetCatalogue
	pools      []domain.SetCatalogue
	cards      map[domain.CardID]domain.Card
	opener     *domain.Opener
	collection *domain.Collection
	packs      map[string]int
	last       *Opening

	// saves is where the game is kept between runs, see Persist, and saveErr why the latest save failed.
	saves   Saves
	saveErr error

	rules  Rules
	energy *domain.Energy
	now    Clock
	// lastTick is the instant up to which the energy has been brought up to date: a whole number of
	// ticks after the start of the session.
	lastTick time.Time
}

// NewGame starts a session on sets, the openable catalogues in display order, and grants the
// welcome boosters. pools are catalogues whose cards the boosters of sets can draw, such as the
// timeshifted cards of Time Spiral, so that every drawn card has a name. The energy starts full and
// recharges with the time that now tells, under rules.
func NewGame(sets, pools []domain.SetCatalogue, opener *domain.Opener, rules Rules, now Clock) (*Game, error) {
	if len(sets) == 0 {
		return nil, errors.New("play: at least one set is required")
	}
	energy, err := domain.NewEnergy(rules.Energy)
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	costs := make(map[string]int, len(sets))
	for _, cat := range sets {
		cost := rules.Costs[cat.Code]
		if cost <= 0 {
			return nil, fmt.Errorf("play: set %q needs an energy cost above zero, got %d", cat.Code, cost)
		}
		if _, err := rules.Energy.TicksToAfford(cost); err != nil {
			return nil, fmt.Errorf("play: set %q: %w", cat.Code, err)
		}
		costs[cat.Code] = cost
	}
	rules.Costs = costs
	cards := make(map[domain.CardID]domain.Card)
	for _, cat := range append(append([]domain.SetCatalogue(nil), sets...), pools...) {
		for id, card := range cat.Cards {
			cards[id] = card
		}
	}
	packs := make(map[string]int, len(sets))
	for i := range WelcomeBoosters {
		packs[sets[i%len(sets)].Code]++
	}
	return &Game{
		sets:       sets,
		pools:      pools,
		cards:      cards,
		opener:     opener,
		collection: domain.NewCollection(),
		packs:      packs,
		rules:      rules,
		energy:     energy,
		now:        now,
		lastTick:   now(),
	}, nil
}

// Packs lists what the player can open of every set, in the order the sets were given to NewGame.
func (g *Game) Packs() []Pack {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.sync()
	out := make([]Pack, len(g.sets))
	for i, cat := range g.sets {
		pack := Pack{SetCode: cat.Code, SetName: cat.Name, Count: g.packs[cat.Code], Cost: g.rules.Costs[cat.Code]}
		if pack.Count == 0 {
			pack.Wait = g.waitFor(pack.Cost, now)
		}
		out[i] = pack
	}
	return out
}

// Open opens one booster of setCode and adds its cards to the collection. A welcome booster of the
// set is used first and costs nothing; once none is left the booster costs the set's energy. It
// returns ErrUnknownSet or ErrNotEnoughEnergy, wrapped, when it cannot, and then changes nothing.
func (g *Game) Open(setCode string) (Opening, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.sync()
	cat, ok := g.set(setCode)
	if !ok {
		return Opening{}, fmt.Errorf("open %q: %w", setCode, ErrUnknownSet)
	}
	welcome := g.packs[setCode] > 0
	cost := g.rules.Costs[setCode]
	if have := g.energy.Reserve() + g.energy.Savings(); !welcome && have < cost {
		return Opening{}, fmt.Errorf("open %q: %w", setCode, &NotEnoughEnergyError{Need: cost, Have: have, Wait: g.waitFor(cost, now)})
	}
	booster, err := g.opener.Open(domain.BoosterLayoutClassic, cat)
	if err != nil {
		return Opening{}, fmt.Errorf("open %q: %w", setCode, err)
	}

	opening := Opening{SetCode: cat.Code, SetName: cat.Name, Cards: make([]RevealedCard, len(booster.Cards))}
	// Resolve every card before touching the collection, so a card in no catalogue leaves the
	// collection and the stock as they were.
	inPack := make(map[domain.CardID]bool, len(booster.Cards))
	for i, key := range booster.Cards {
		card, ok := g.cards[key.ID]
		if !ok {
			return Opening{}, fmt.Errorf("open %q: card %q is in no catalogue", setCode, key.ID)
		}
		isNew := g.collection.Copies(key.ID) == 0 && !inPack[key.ID]
		inPack[key.ID] = true
		opening.Cards[i] = RevealedCard{Card: card, Finish: key.Finish, New: isNew}
	}
	// Pay only now that nothing can fail any more.
	if welcome {
		g.packs[setCode]--
	} else if err := g.energy.Spend(cost); err != nil {
		return Opening{}, fmt.Errorf("open %q: %w", setCode, err)
	}
	g.collection.AddBooster(booster)
	g.last = &opening
	g.save()
	return opening, nil
}

// LastOpening returns the most recent opening, and false when no booster was opened yet.
func (g *Game) LastOpening() (Opening, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.last == nil {
		return Opening{}, false
	}
	return *g.last, true
}

func (g *Game) set(code string) (domain.SetCatalogue, bool) {
	for _, cat := range g.sets {
		if cat.Code == code {
			return cat, true
		}
	}
	return domain.SetCatalogue{}, false
}
