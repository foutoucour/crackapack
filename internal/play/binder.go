package play

import (
	"fmt"
	"sort"

	"github.com/foutoucour/crackapack/internal/domain"
)

// BinderSummary is the progress of one set's binder.
type BinderSummary struct {
	SetCode  string
	SetName  string
	Progress domain.Progress
}

// BinderCard is a card of a binder and how many copies of it the player holds, by finish. A card
// held in neither finish is an empty slot.
type BinderCard struct {
	Card    domain.Card
	Regular int
	Foil    int
}

// BinderPool is the part of a binder that comes from another catalogue: the timeshifted cards of
// Time Spiral are a set of their own, which the boosters of Time Spiral draw from.
type BinderPool struct {
	// Name is the name of the pool's catalogue.
	Name string
	// Slots has a slot for every card of the pool that a booster of the set can draw, in collector
	// number order. A card that no booster draws is left out, since nobody could ever hold it.
	Slots []BinderCard
}

// Binder is the view of one set's collection, see docs/core-features.md#collection-and-binders.
type Binder struct {
	SetCode string
	SetName string
	// Progress counts the set's own cards and the cards of its pools. Basic lands never count.
	Progress domain.Progress
	// Slots has one slot for every card of the set except basic lands, in collector number
	// order. A basic land comes after them, and only once the player holds one.
	Slots []BinderCard
	// Pools are the sections of the binder that come from another catalogue.
	Pools []BinderPool
}

// Binders lists the progress of every set's binder, in the order the sets were given to NewGame.
func (g *Game) Binders() []BinderSummary {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]BinderSummary, len(g.sets))
	for i, cat := range g.sets {
		out[i] = BinderSummary{SetCode: cat.Code, SetName: cat.Name, Progress: g.collection.BinderProgress(domain.WithPools(cat, g.pools))}
	}
	return out
}

// Binder returns the binder of setCode. It returns ErrUnknownSet, wrapped, for any other code.
func (g *Game) Binder(setCode string) (Binder, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	cat, ok := g.set(setCode)
	if !ok {
		return Binder{}, fmt.Errorf("binder %q: %w", setCode, ErrUnknownSet)
	}
	binder := Binder{
		SetCode:  cat.Code,
		SetName:  cat.Name,
		Progress: g.collection.BinderProgress(domain.WithPools(cat, g.pools)),
	}

	for _, card := range cat.Cards {
		held := g.held(card)
		if card.IsBasicLand && held.Regular+held.Foil == 0 {
			continue
		}
		binder.Slots = append(binder.Slots, held)
	}
	sort.Slice(binder.Slots, func(i, j int) bool {
		a, b := binder.Slots[i], binder.Slots[j]
		if a.Card.IsBasicLand != b.Card.IsBasicLand {
			return !a.Card.IsBasicLand
		}
		return cardLess(a.Card, b.Card)
	})

	for _, pool := range g.pools {
		drawable := domain.DrawableCards(cat, pool)
		if len(drawable) == 0 {
			continue
		}
		section := BinderPool{Name: pool.Name, Slots: make([]BinderCard, len(drawable))}
		for i, card := range drawable {
			section.Slots[i] = g.held(card)
		}
		sort.Slice(section.Slots, func(i, j int) bool { return cardLess(section.Slots[i].Card, section.Slots[j].Card) })
		binder.Pools = append(binder.Pools, section)
	}
	return binder, nil
}

func (g *Game) held(card domain.Card) BinderCard {
	return BinderCard{
		Card:    card,
		Regular: g.collection.Quantity(domain.CardKey{ID: card.ID, Finish: domain.FinishNonFoil}),
		Foil:    g.collection.Quantity(domain.CardKey{ID: card.ID, Finish: domain.FinishFoil}),
	}
}

// cardLess orders cards by collector number, as a number when it starts with digits so that 9
// comes before 10, then by the rest of the number and the ID so that the order is total.
func cardLess(a, b domain.Card) bool {
	na, aNumeric := leadingNumber(a.Number)
	nb, bNumeric := leadingNumber(b.Number)
	switch {
	case aNumeric != bNumeric:
		return aNumeric
	case aNumeric && na != nb:
		return na < nb
	case a.Number != b.Number:
		return a.Number < b.Number
	default:
		return a.ID < b.ID
	}
}

// leadingNumber returns the integer a collector number starts with, such as 123 for "123a".
func leadingNumber(s string) (int, bool) {
	n, digits := 0, 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		digits++
	}
	return n, digits > 0
}
