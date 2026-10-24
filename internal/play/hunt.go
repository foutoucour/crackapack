package play

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// MaxHunted is the most cards the player hunts at once, the size of the wish list in
// docs/core-features.md#collection-and-binders.
const MaxHunted = 20

// huntReaskAfter is how long a hunt waits before it asks the NPCs about its card again, once the
// answers to the last time are over: the hunt makes the NPCs offer the card, not pester the player.
const huntReaskAfter = gameDay

var (
	// ErrHeld is returned by Hunt for a card the player holds already: there is nothing to hunt.
	ErrHeld = errors.New("play: the player already holds the card")
	// ErrTooManyHunted is returned by Hunt when the player hunts MaxHunted cards already.
	ErrTooManyHunted = errors.New("play: too many cards are hunted")
)

// Hunted is a card the player hunts, and where the hunt stands.
type Hunted struct {
	Card   domain.Card
	Finish domain.Finish
	// SetCode is the first unlocked set whose binder holds the card, where it can be bought, and
	// empty when none is unlocked.
	SetCode string
	// AtCounter is true while the Counter has a copy, and BuyPrice what one costs. It is never said
	// how many: hunting tells the player that a copy came, as the Counter's quotes do, not how many.
	AtCounter bool
	BuyPrice  domain.Coins
	// New is true when the card came to the Counter since the player last looked, see SeeHunted.
	New bool
	// Offers is the answers of the NPCs that the player has not taken, declined or countered.
	Offers int
}

// hunt is a card the player hunts, as the game keeps it.
type hunt struct {
	key domain.CardKey
	// seen is true when the Counter had a copy the last time the hunt looked, and told when the
	// player has been shown that copy: the card is new to the player while seen is true and told is
	// not. A card that runs out of stock is forgotten, so that its next copy is new again.
	seen, told bool
	// askedAt is when the hunt last asked the NPCs about the card, the zero time before it did.
	askedAt time.Time
}

// Hunt starts hunting the card id in finish: the NPCs that hold it offer it on the market, asked
// again each day, and Hunted tells when the Counter has a copy. The hunt ends once the player holds
// the card. Hunting a card already hunted does nothing. It returns ErrUnknownCard, ErrLocked when
// no unlocked set's binder holds the card, ErrHeld, ErrTooManyHunted or ErrNoCounter, wrapped, and
// then changes nothing.
func (g *Game) Hunt(id domain.CardID, finish domain.Finish) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, key, err := g.counterFor(id, finish)
	if err != nil {
		return fmt.Errorf("hunt %q: %w", id, err)
	}
	switch {
	case !g.buyable(id):
		return fmt.Errorf("hunt %q: %w", id, ErrLocked)
	case g.collection.Quantity(key) > 0:
		return fmt.Errorf("hunt %q: %w", id, ErrHeld)
	case g.huntIndex(key) >= 0:
		return nil
	case len(g.hunts) >= MaxHunted:
		return fmt.Errorf("hunt %q: %w", id, ErrTooManyHunted)
	}
	g.hunts = append(g.hunts, &hunt{key: key})
	if _, err := g.sweepHunts(); err != nil {
		return fmt.Errorf("hunt %q: %w", id, err)
	}
	g.save()
	return nil
}

// Unhunt stops hunting the card id in finish. It does nothing for a card that is not hunted. The
// requests the hunt made on the market stay there until the player withdraws them.
func (g *Game) Unhunt(id domain.CardID, finish domain.Finish) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	i := g.huntIndex(domain.CardKey{ID: id, Finish: finish})
	if i < 0 {
		return nil
	}
	g.hunts = slices.Delete(g.hunts, i, i+1)
	g.save()
	return nil
}

// Hunted lists the cards the player hunts, oldest hunt first, after the Counter is caught up and
// each hunt has looked at the Counter and asked the NPCs. A hunt ends once the player holds its
// card. It returns ErrNoCounter in a game without a Counter.
func (g *Game) Hunted() ([]Hunted, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.syncCounter(); err != nil {
		return nil, fmt.Errorf("hunted cards: %w", err)
	}
	changed, err := g.sweepHunts()
	if changed {
		g.save()
	}
	if err != nil {
		return nil, fmt.Errorf("hunted cards: %w", err)
	}
	out := make([]Hunted, 0, len(g.hunts))
	for _, h := range g.hunts {
		prices, err := g.economy.counter.Quote(h.key)
		if err != nil {
			return nil, fmt.Errorf("hunted cards: %w", err)
		}
		out = append(out, Hunted{
			Card: g.cards[h.key.ID], Finish: h.key.Finish, SetCode: g.unlockedBinderOf(h.key.ID),
			AtCounter: h.seen, BuyPrice: prices.Asks, New: h.seen && !h.told, Offers: g.huntOffers(h.key),
		})
	}
	return out, nil
}

// SeeHunted records that the player has been shown the copies at the Counter, so that they are no
// longer new. It returns ErrNoCounter in a game without a Counter.
func (g *Game) SeeHunted() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.economy == nil {
		return fmt.Errorf("see hunted cards: %w", ErrNoCounter)
	}
	changed := false
	for _, h := range g.hunts {
		if h.seen && !h.told {
			h.told, changed = true, true
		}
	}
	if changed {
		g.save()
	}
	return nil
}

// sweepHunts brings the hunts up to date: it drops those whose card the player holds, notes which
// cards the Counter has, and has the market ask the NPCs about each card not asked about for a day.
// It reports whether anything changed, which the caller saves. g.mu must be held and the Counter
// caught up.
func (g *Game) sweepHunts() (changed bool, err error) {
	e := g.economy
	now := g.now()
	kept := make([]*hunt, 0, len(g.hunts))
	asked := false
	for _, h := range g.hunts {
		if g.collection.Quantity(h.key) > 0 {
			changed = true
			continue
		}
		kept = append(kept, h)
		inStock := e.counter.Stock(h.key) > 0
		if inStock != h.seen || (!inStock && h.told) {
			h.seen, h.told = inStock, h.told && inStock
			changed = true
		}
		if e.market == nil || !g.buyable(h.key.ID) || (!h.askedAt.IsZero() && now.Sub(h.askedAt) < huntReaskAfter) {
			continue
		}
		switch _, askErr := e.market.AskAround(h.key, now); {
		case askErr == nil:
			h.askedAt, changed, asked = now, true, true
		case errors.Is(askErr, domain.ErrAlreadyAsked), errors.Is(askErr, domain.ErrNoHolder):
			// Someone is answering already, or nobody has the card: the next sweep looks again.
		default:
			g.hunts = kept
			return true, askErr
		}
	}
	g.hunts = kept
	if asked {
		err = e.advanceMarket(now)
	}
	return changed, err
}

// huntOffers is the answers of the market to the request for key that wait for the player. g.mu
// must be held.
func (g *Game) huntOffers(key domain.CardKey) int {
	m := g.economy.market
	if m == nil {
		return 0
	}
	n := 0
	for _, r := range m.Requests() {
		if r.Card != key {
			continue
		}
		for _, a := range m.Answers() {
			if a.To == r.ID && a.Counter == nil {
				n++
			}
		}
	}
	return n
}

// unlockedBinderOf is the code of the first unlocked set whose binder holds the card id, or "".
// g.mu must be held.
func (g *Game) unlockedBinderOf(id domain.CardID) string {
	for _, code := range g.binderSets[id] {
		if !g.locked(code) {
			return code
		}
	}
	return ""
}

// huntIndex is the position of key among the hunts, or -1. g.mu must be held.
func (g *Game) huntIndex(key domain.CardKey) int {
	return slices.IndexFunc(g.hunts, func(h *hunt) bool { return h.key == key })
}
