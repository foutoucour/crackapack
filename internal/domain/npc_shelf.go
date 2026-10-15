package domain

import (
	"fmt"
	"maps"
	"slices"
)

// Shelf returns what the NPC can trade away: the surplus it holds before selling it to the
// Counter, sorted by card.
func (n *NPC) Shelf() []TradeLine {
	lines := make([]TradeLine, 0, len(n.shelf))
	for key, ticks := range n.shelf {
		lines = append(lines, TradeLine{Key: key, Qty: len(ticks)})
	}
	slices.SortFunc(lines, func(a, b TradeLine) int { return cmpKey(a.Key, b.Key) })
	return lines
}

// ShelfQuantity returns how many copies of key the NPC holds on its shelf.
func (n *NPC) ShelfQuantity(key CardKey) int { return len(n.shelf[key]) }

// Wishes returns a copy of the NPC's wish list.
func (n *NPC) Wishes() []Wish { return slices.Clone(n.wishes) }

// HoldersOf returns the IDs of the NPCs holding key on their shelf, ascending.
func (c *Crowd) HoldersOf(key CardKey) []int {
	return slices.Sorted(maps.Keys(c.holders[key]))
}

// indexAdd records that NPC npc relates to key in idx.
func indexAdd[K comparable](idx map[K]map[int]struct{}, key K, npc int) {
	if idx[key] == nil {
		idx[key] = make(map[int]struct{})
	}
	idx[key][npc] = struct{}{}
}

// indexRemove drops the link between NPC npc and key in idx, and the key when no NPC is left.
func indexRemove[K comparable](idx map[K]map[int]struct{}, key K, npc int) {
	delete(idx[key], npc)
	if len(idx[key]) == 0 {
		delete(idx, key)
	}
}

// shelfPut puts one copy of key on npc's shelf, stamped with the current tick.
func (c *Crowd) shelfPut(npc *NPC, key CardKey) {
	if len(npc.shelf[key]) == 0 {
		indexAdd(c.holders, key, npc.ID)
	}
	npc.shelf[key] = append(npc.shelf[key], c.tick)
}

// shelfTake removes the n oldest copies of key from npc's shelf.
func (c *Crowd) shelfTake(npc *NPC, key CardKey, n int) error {
	if n <= 0 {
		return &InvalidQuantityError{N: n}
	}
	ticks := npc.shelf[key]
	if len(ticks) < n {
		return &InsufficientQuantityError{Key: key, Have: len(ticks), Want: n}
	}
	if len(ticks) == n {
		delete(npc.shelf, key)
		indexRemove(c.holders, key, npc.ID)
		return nil
	}
	npc.shelf[key] = ticks[n:]
	return nil
}

// releaseAged sells to the Counter every shelf copy that has waited the hold, and returns how many.
func (c *Crowd) releaseAged() (int, error) {
	var released int
	for _, npc := range c.npcs {
		if len(npc.shelf) == 0 {
			continue
		}
		for _, key := range slices.SortedFunc(maps.Keys(npc.shelf), cmpKey) {
			ticks := npc.shelf[key]
			aged := 0
			for aged < len(ticks) && c.tick-ticks[aged] >= c.holdTicks {
				aged++
			}
			if aged == 0 {
				continue
			}
			if err := c.counter.AcceptSurplus(key, aged); err != nil {
				return released, fmt.Errorf("NPC %d: sell surplus of %q: %w", npc.ID, key.ID, err)
			}
			if err := c.shelfTake(npc, key, aged); err != nil {
				return released, fmt.Errorf("NPC %d: %w", npc.ID, err)
			}
			c.held -= aged
			released += aged
		}
	}
	return released, nil
}

// applyTrade moves the cards of a trade on the NPC's side: it gives the lines in gives from its
// shelf and receives the lines in gets. A received copy above the keep threshold goes onto the
// shelf. The NPC's wishes are refreshed afterwards. The caller has checked the NPC holds
// everything it gives.
func (c *Crowd) applyTrade(npc *NPC, gives, gets []TradeLine) error {
	for _, l := range gives {
		if err := c.shelfTake(npc, l.Key, l.Qty); err != nil {
			return err
		}
		c.held -= l.Qty
	}
	for _, l := range gets {
		if err := npc.collection.Add(l.Key, l.Qty); err != nil {
			return err
		}
		c.held += l.Qty
		for range l.Qty {
			if npc.collection.Copies(l.Key.ID) <= npc.KeepCopies {
				break
			}
			if err := npc.collection.Remove(l.Key, 1); err != nil {
				return err
			}
			c.shelfPut(npc, l.Key)
		}
	}
	c.refreshWishes(npc)
	return nil
}

// refreshWishes replaces the wishes the NPC has fulfilled. It asks the behaviour for more only
// when a wish was dropped: the NPC only gains cards, so a list that came up short when it was last
// filled, because nothing was left to wish for, stays short, and rescanning the scope after every
// booster would cost more than everything else trades add to a run.
func (c *Crowd) refreshWishes(npc *NPC) {
	if npc.Behaviour == nil || npc.WishSize <= 0 {
		return
	}
	if c.dropFulfilledWishes(npc) == 0 {
		return
	}
	c.fillWishes(npc)
}

// dropFulfilledWishes removes the wishes the NPC now holds enough copies of and returns how many.
func (c *Crowd) dropFulfilledWishes(npc *NPC) int {
	kept := npc.wishes[:0]
	for _, w := range npc.wishes {
		if npc.collection.Copies(w.ID) >= w.Want {
			indexRemove(c.wishers, w.ID, npc.ID)
			continue
		}
		kept = append(kept, w)
	}
	dropped := len(npc.wishes) - len(kept)
	npc.wishes = kept
	return dropped
}

// fillWishes asks the NPC's behaviour for wishes until the list reaches the NPC's wish size.
func (c *Crowd) fillWishes(npc *NPC) {
	if npc.Behaviour == nil || npc.WishSize <= 0 || len(npc.wishes) >= npc.WishSize {
		return
	}
	wished := make(map[CardID]struct{}, len(npc.wishes))
	for _, w := range npc.wishes {
		wished[w.ID] = struct{}{}
	}
	ctx := WishContext{NPC: npc, Scope: c.scopeOf(npc), Wished: wished}
	for _, w := range npc.Behaviour.PickWishes(ctx, npc.WishSize-len(npc.wishes), c.rng) {
		npc.wishes = append(npc.wishes, w)
		indexAdd(c.wishers, w.ID, npc.ID)
	}
}

// scopeOf returns the cards npc may wish for.
// When the NPC has a colour restriction, the base scope (scopeBySet or scopeAll) is filtered
// once and the result is memoised under the (focus set, colour set) key so that all NPCs with
// the same colours and focus set share one backing array.
func (c *Crowd) scopeOf(npc *NPC) []Card {
	var base []Card
	if npc.FocusSet != "" {
		base = c.scopeBySet[npc.FocusSet]
	} else {
		base = c.scopeAll
	}
	colours := colourSetOf(npc.Colors)
	// An empty list (colours==0) or all five colours (allColours) means no restriction: return
	// the base scope directly so all such NPCs share the same underlying array.
	if colours == 0 || colours == allColours {
		return base
	}
	key := scopeKey{focus: npc.FocusSet, colours: colours}
	if filtered, ok := c.scopes[key]; ok {
		return filtered
	}
	filtered := filterByColours(base, colours)
	c.scopes[key] = filtered
	return filtered
}
