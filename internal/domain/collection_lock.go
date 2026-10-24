package domain

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrLockedCopy is returned when a copy that a milestone locked would leave the collection.
	ErrLockedCopy = errors.New("collection: the copy is locked by a milestone")
	// ErrNotHeldToLock is returned by Lock for a copy the collection does not hold.
	ErrNotHeldToLock = errors.New("collection: no copy to lock")
)

// Lock keeps one copy of key in the collection for good: Remove refuses it from now on, so the
// player cannot earn a milestone and sell what earned it. A card has one lock, whatever the
// finish: a card locked already keeps the lock it has, and Lock changes nothing. It returns
// ErrNotHeldToLock, wrapped, when the collection holds no copy of key.
func (c *Collection) Lock(key CardKey) error {
	if _, locked := c.locked[key.ID]; locked {
		return nil
	}
	if c.qty[key] < 1 {
		return fmt.Errorf("lock card %s (finish %s): %w", key.ID, key.Finish, ErrNotHeldToLock)
	}
	c.locked[key.ID] = key.Finish
	return nil
}

// Locked returns how many copies of key are locked: 1 for the finish of the card that was locked,
// and 0 for any other.
func (c *Collection) Locked(key CardKey) int {
	if finish, locked := c.locked[key.ID]; locked && finish == key.Finish {
		return 1
	}
	return 0
}

// IsLocked reports whether a copy of the card id, in either finish, is locked.
func (c *Collection) IsLocked(id CardID) bool {
	_, locked := c.locked[id]
	return locked
}

// LockedKeys returns the copy locked of every locked card, ordered by card and finish, so that a
// save of it is stable.
func (c *Collection) LockedKeys() []CardKey {
	keys := make([]CardKey, 0, len(c.locked))
	for id, finish := range c.locked {
		keys = append(keys, CardKey{ID: id, Finish: finish})
	}
	slices.SortFunc(keys, func(a, b CardKey) int {
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Finish, b.Finish))
	})
	return keys
}
