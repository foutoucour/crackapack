package domain

import (
	"cmp"
	"fmt"
	"slices"
)

// InsufficientQuantityError is returned by Remove when the collection holds fewer
// copies of a card than requested.
type InsufficientQuantityError struct {
	Key  CardKey
	Have int
	Want int
}

// Error implements the error interface.
func (e *InsufficientQuantityError) Error() string {
	return fmt.Sprintf("collection: remove card %s (finish %s): have %d, want %d",
		e.Key.ID, e.Key.Finish, e.Have, e.Want)
}

// InvalidQuantityError is returned by Add and Remove when n is not positive.
type InvalidQuantityError struct {
	N int
}

// Error implements the error interface.
func (e *InvalidQuantityError) Error() string {
	return fmt.Sprintf("collection: quantity must be positive, got %d", e.N)
}

// InvalidKeepError is returned by DuplicatesAbove when keep is outside [1, 4].
type InvalidKeepError struct {
	Keep int
}

// Error implements the error interface.
func (e *InvalidKeepError) Error() string {
	return fmt.Sprintf("collection: keep must be between 1 and %d inclusive, got %d", PlaysetSize, e.Keep)
}

// RarityProgress holds the owned and total card count for one rarity within a set binder.
type RarityProgress struct {
	Owned int
	Total int
}

// Progress is the binder progress for one SetCatalogue.
// Basic lands are excluded from all counts.
// A card is counted as owned when any finish holds at least one copy.
// A playset is complete when the total copies across all finishes is at least 4.
type Progress struct {
	Owned    int
	Total    int
	ByRarity map[Rarity]RarityProgress
	Playsets int
}

// Surplus describes the copies of one CardKey that exceed the keep threshold in
// DuplicatesAbove.
type Surplus struct {
	Key      CardKey
	Quantity int
}

// Collection holds the player's (or an NPC's) cards as quantities per CardKey
// (printing + finish). Construct with NewCollection; the zero value is not ready to use.
//
// Collection is not safe for concurrent use.
type Collection struct {
	qty map[CardKey]int
}

// NewCollection returns an empty Collection ready to use.
func NewCollection() *Collection {
	return &Collection{qty: make(map[CardKey]int)}
}

// Add adds n copies of key to the collection.
// It returns InvalidQuantityError when n is not positive.
func (c *Collection) Add(key CardKey, n int) error {
	if n <= 0 {
		return &InvalidQuantityError{N: n}
	}
	c.qty[key] += n
	return nil
}

// AddBooster adds one copy of each card in b to the collection.
func (c *Collection) AddBooster(b Booster) {
	for _, key := range b.Cards {
		c.qty[key]++
	}
}

// Remove removes n copies of key from the collection.
// It returns InvalidQuantityError when n <= 0, and InsufficientQuantityError when
// the collection holds fewer than n copies. Both checks precede any mutation, so
// the collection is unchanged on error.
// When Remove brings a quantity to zero, the key is deleted from the map.
func (c *Collection) Remove(key CardKey, n int) error {
	if n <= 0 {
		return &InvalidQuantityError{N: n}
	}
	have := c.qty[key]
	if have < n {
		return &InsufficientQuantityError{Key: key, Have: have, Want: n}
	}
	if have == n {
		delete(c.qty, key)
	} else {
		c.qty[key] -= n
	}
	return nil
}

// BinderProgress reports binder progress for the cards in cat.
// Basic lands (IsBasicLand == true) are excluded from all counts.
// Cards in the collection but absent from cat are ignored.
//
// Copies are counted by summing FinishNonFoil and FinishFoil, not card.Finishes,
// so a copy stored under a finish the catalogue does not list still counts.
func (c *Collection) BinderProgress(cat SetCatalogue) Progress {
	p := Progress{ByRarity: make(map[Rarity]RarityProgress)}
	for _, card := range cat.Cards {
		if card.IsBasicLand {
			continue
		}
		// Sum across the two finish constants, independent of card.Finishes.
		copies := c.qty[CardKey{ID: card.ID, Finish: FinishNonFoil}] +
			c.qty[CardKey{ID: card.ID, Finish: FinishFoil}]

		p.Total++
		rp := p.ByRarity[card.Rarity]
		rp.Total++

		if copies > 0 {
			p.Owned++
			rp.Owned++
		}
		if copies >= PlaysetSize {
			p.Playsets++
		}
		p.ByRarity[card.Rarity] = rp
	}
	return p
}

// DuplicatesAbove returns the surplus copies per CardKey for cards whose total count
// across finishes exceeds keep. keep must be in [1, 4]; InvalidKeepError is returned otherwise.
//
// Surplus is allocated by removing non-foil copies first, so that foils — rarer and harder
// to replace — are kept. Results are sorted by (CardKey.ID, CardKey.Finish) ascending for
// determinism; callers must not rely on map iteration order.
func (c *Collection) DuplicatesAbove(keep int) ([]Surplus, error) {
	if keep < 1 || keep > PlaysetSize {
		return nil, &InvalidKeepError{Keep: keep}
	}

	type finishQtys struct{ nonfoil, foil int }
	byID := make(map[CardID]finishQtys)
	for key, qty := range c.qty {
		fq := byID[key.ID]
		switch key.Finish {
		case FinishNonFoil:
			fq.nonfoil += qty
		case FinishFoil:
			fq.foil += qty
		}
		byID[key.ID] = fq
	}

	type idEntry struct {
		id CardID
		fq finishQtys
	}
	entries := make([]idEntry, 0, len(byID))
	for id, fq := range byID {
		if fq.nonfoil+fq.foil > keep {
			entries = append(entries, idEntry{id: id, fq: fq})
		}
	}
	slices.SortFunc(entries, func(a, b idEntry) int { return cmp.Compare(a.id, b.id) })

	var result []Surplus
	for _, e := range entries {
		surplus := e.fq.nonfoil + e.fq.foil - keep
		// Non-foil surplus is taken first; foils are preserved as long as possible.
		nfSurplus := min(surplus, e.fq.nonfoil)
		fSurplus := surplus - nfSurplus
		if nfSurplus > 0 {
			result = append(result, Surplus{
				Key:      CardKey{ID: e.id, Finish: FinishNonFoil},
				Quantity: nfSurplus,
			})
		}
		if fSurplus > 0 {
			result = append(result, Surplus{
				Key:      CardKey{ID: e.id, Finish: FinishFoil},
				Quantity: fSurplus,
			})
		}
	}
	return result, nil
}

// Value returns the total collection value in USD cents.
// Each physical copy of a card is priced from snap; cards absent from snap use fallback.
// Foil and non-foil copies of the same printing share one price (a simplification).
func (c *Collection) Value(snap PriceSnapshot, fallback USDCents) USDCents {
	var total USDCents
	for key, qty := range c.qty {
		price, ok := snap[key.ID]
		if !ok {
			price = fallback
		}
		total += price * USDCents(qty)
	}
	return total
}

// CoinValue converts the total collection value to coins at rate.
// PriceToCoins is applied once to the total, producing a single truncation per ADR-0003.
// ErrSubCoin is returned when the total is positive but rounds to fewer than one coin.
func (c *Collection) CoinValue(snap PriceSnapshot, fallback USDCents, rate CoinRate) (Coins, error) {
	return PriceToCoins(c.Value(snap, fallback), rate, 100)
}
