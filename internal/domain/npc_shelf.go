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

// refreshWishes drops the wishes the NPC has fulfilled and, when the list is below the NPC's
// wish size, asks its behaviour for more.
func (c *Crowd) refreshWishes(npc *NPC) {
	if npc.Behaviour == nil || npc.WishSize <= 0 {
		return
	}
	kept := npc.wishes[:0]
	for _, w := range npc.wishes {
		if npc.collection.Copies(w.ID) >= w.Want {
			indexRemove(c.wishers, w.ID, npc.ID)
			continue
		}
		kept = append(kept, w)
	}
	npc.wishes = kept
	if len(kept) >= npc.WishSize {
		return
	}
	wished := make(map[CardID]struct{}, len(kept))
	for _, w := range kept {
		wished[w.ID] = struct{}{}
	}
	ctx := WishContext{NPC: npc, Scope: c.scopeOf(npc), Wished: wished}
	for _, w := range npc.Behaviour.PickWishes(ctx, npc.WishSize-len(kept), c.rng) {
		npc.wishes = append(npc.wishes, w)
		indexAdd(c.wishers, w.ID, npc.ID)
	}
}

// scopeOf returns the cards npc may wish for.
func (c *Crowd) scopeOf(npc *NPC) []Card {
	if npc.FocusSet != "" {
		return c.scopeBySet[npc.FocusSet]
	}
	return c.scopeAll
}
