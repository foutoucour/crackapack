package domain

import (
	"fmt"
	"slices"
)

// CrowdState is what the NPCs of a crowd hold and remember, so that a save can keep it and Restore
// can bring it back. Who the NPCs are comes from GeneratePopulation, and the random stream is the
// caller's, saved apart.
type CrowdState struct {
	// Tick is the ticks the crowd has lived, which the shelves are stamped with.
	Tick int
	// NPCs are the NPCs in ID order.
	NPCs []NPCState
}

// NPCState is what one NPC holds and remembers.
type NPCState struct {
	// Credit is the NPC's progress toward its next booster, in ticks of a day.
	Credit int
	// Cards are the copies the NPC keeps, by card.
	Cards map[CardKey]int
	// Shelf is the surplus the NPC can trade away, by card: the tick each copy arrived on, oldest
	// first. It is empty in a crowd that does not hold its surplus.
	Shelf map[CardKey][]int
	// Wishes is the wish list, in the order the behaviour picked it.
	Wishes []Wish
	// DeclineStreak is the declines since the last accepted trade, see NPC.DeclineStreak.
	DeclineStreak int
	// ChanceTilt is the offer chance the gambler trait adds while the streak lasts.
	ChanceTilt int
	// Retry is where the ego trait's follow-ups stand.
	Retry RetryState
}

// RetryState is where an NPC's follow-ups after a decline stand, see ADR-0025. The zero value is
// none armed.
type RetryState struct {
	// Limit is the follow-ups a day; 0 means the NPC never retried.
	Limit int
	// Pending is true while a decline waits for its follow-up.
	Pending bool
	// Used is the follow-ups made on Day, a day of the Exchange.
	Used, Day int
	// Avoid is the cards of the gives the player declined in this chain.
	Avoid []CardKey
}

// State returns what the crowd holds and remembers.
func (c *Crowd) State() (CrowdState, error) {
	s := CrowdState{Tick: c.tick, NPCs: make([]NPCState, len(c.npcs))}
	for i, npc := range c.npcs {
		n := NPCState{
			Credit:        npc.credit,
			Cards:         npc.collection.Snapshot(),
			Wishes:        slices.Clone(npc.wishes),
			DeclineStreak: npc.declineStreak,
			ChanceTilt:    npc.chanceTilt,
			Retry: RetryState{
				Limit: npc.retry.limit, Pending: npc.retry.pending, Used: npc.retry.used, Day: npc.retry.day,
				Avoid: slices.Clone(npc.retry.avoid),
			},
		}
		if len(npc.shelf) > 0 {
			n.Shelf = make(map[CardKey][]int, len(npc.shelf))
			for key, ticks := range npc.shelf {
				n.Shelf[key] = slices.Clone(ticks)
			}
		}
		s.NPCs[i] = n
	}
	return s, nil
}

// Restore sets the crowd to s, which State returned. It returns an error, and changes nothing,
// when s cannot be the state of this crowd: another number of NPCs, a negative tick, a credit
// outside a day, a card held no times, a shelf copy stamped outside the crowd's life or out of
// order, a wish for no copy or twice, or a negative streak.
func (c *Crowd) Restore(s CrowdState) error {
	if len(s.NPCs) != len(c.npcs) {
		return fmt.Errorf("crowd: the state has %d NPCs, the crowd %d", len(s.NPCs), len(c.npcs))
	}
	if s.Tick < 0 {
		return fmt.Errorf("crowd: tick %d is negative", s.Tick)
	}
	collections := make([]*Collection, len(s.NPCs))
	held := 0
	for i, n := range s.NPCs {
		cards, err := checkNPCState(n, s.Tick)
		if err != nil {
			return fmt.Errorf("crowd: NPC %d: %w", i, err)
		}
		if collections[i], err = collectionOf(n.Cards); err != nil {
			return fmt.Errorf("crowd: NPC %d: %w", i, err)
		}
		held += cards
	}
	c.wishers = make(map[CardID]map[int]struct{})
	c.holders = make(map[CardKey]map[int]struct{})
	for i, npc := range c.npcs {
		n := s.NPCs[i]
		npc.credit, npc.collection = n.Credit, collections[i]
		npc.wishes = slices.Clone(n.Wishes)
		npc.declineStreak, npc.chanceTilt = n.DeclineStreak, n.ChanceTilt
		npc.retry = retryState{limit: n.Retry.Limit, pending: n.Retry.Pending, used: n.Retry.Used, day: n.Retry.Day, avoid: slices.Clone(n.Retry.Avoid)}
		npc.shelf = make(map[CardKey][]int, len(n.Shelf))
		for key, ticks := range n.Shelf {
			npc.shelf[key] = slices.Clone(ticks)
			indexAdd(c.holders, key, npc.ID)
		}
		for _, w := range npc.wishes {
			indexAdd(c.wishers, w.ID, npc.ID)
		}
	}
	c.tick, c.held = s.Tick, held
	return nil
}

// checkNPCState checks the parts of n that a collection does not, against the crowd's tick, and
// returns the cards n holds, kept and shelved.
func checkNPCState(n NPCState, tick int) (int, error) {
	switch {
	case n.Credit < 0 || n.Credit >= TicksPerDay:
		return 0, fmt.Errorf("credit %d is outside 0..%d", n.Credit, TicksPerDay-1)
	case n.DeclineStreak < 0:
		return 0, fmt.Errorf("decline streak %d is negative", n.DeclineStreak)
	}
	held := 0
	for _, qty := range n.Cards {
		held += qty
	}
	for key, ticks := range n.Shelf {
		if len(ticks) == 0 {
			return 0, fmt.Errorf("shelf of %q holds no copy", key.ID)
		}
		for i, t := range ticks {
			if t < 0 || t > tick || (i > 0 && t < ticks[i-1]) {
				return 0, fmt.Errorf("shelf of %q: copies arrived on ticks %v, want ascending within 0..%d", key.ID, ticks, tick)
			}
		}
		held += len(ticks)
	}
	wished := make(map[CardID]bool, len(n.Wishes))
	for _, w := range n.Wishes {
		if w.Want < 1 || w.Want > PlaysetSize || wished[w.ID] {
			return 0, fmt.Errorf("wish for %q x%d is not one copy to a playset, or comes twice", w.ID, w.Want)
		}
		wished[w.ID] = true
	}
	return held, nil
}

func collectionOf(cards map[CardKey]int) (*Collection, error) {
	c := NewCollection()
	for key, qty := range cards {
		if err := c.Add(key, qty); err != nil {
			return nil, fmt.Errorf("card %q: %w", key.ID, err)
		}
	}
	return c, nil
}
