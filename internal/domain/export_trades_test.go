package domain

import (
	"maps"
	"slices"
)

// ShelveForTest puts n copies of key on the shelf of NPC npcID, stamped with the current tick.
func (c *Crowd) ShelveForTest(npcID int, key CardKey, n int) {
	npc := c.byID[npcID]
	for range n {
		c.shelfPut(npc, key)
	}
}

// UnshelveForTest removes n copies of key from the shelf of NPC npcID.
func (c *Crowd) UnshelveForTest(npcID int, key CardKey, n int) error {
	npc := c.byID[npcID]
	if err := c.shelfTake(npc, key, n); err != nil {
		return err
	}
	c.held -= n
	return nil
}

// SetWishesForTest replaces the wish list of NPC npcID and keeps the wishers index in step.
func (c *Crowd) SetWishesForTest(npcID int, wishes []Wish) {
	npc := c.byID[npcID]
	for _, w := range npc.wishes {
		indexRemove(c.wishers, w.ID, npcID)
	}
	npc.wishes = slices.Clone(wishes)
	for _, w := range wishes {
		indexAdd(c.wishers, w.ID, npcID)
	}
}

// WishersForTest returns the IDs of the NPCs that wish for id, ascending.
func (c *Crowd) WishersForTest(id CardID) []int {
	return slices.Sorted(maps.Keys(c.wishers[id]))
}
