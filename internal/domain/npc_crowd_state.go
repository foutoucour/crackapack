package domain

import (
	"errors"
	"fmt"
)

// CrowdState is what the NPCs of a crowd that does not trade hold, so that a save can keep it and
// Restore can bring it back. Who the NPCs are comes from GeneratePopulation, and the random stream
// is the caller's, saved apart.
type CrowdState struct {
	// Tick is the ticks the crowd has lived.
	Tick int
	// NPCs are the NPCs in ID order.
	NPCs []NPCState
}

// NPCState is what one NPC of a crowd that does not trade holds.
type NPCState struct {
	// Credit is the NPC's progress toward its next booster, in ticks of a day.
	Credit int
	// Cards are the copies the NPC keeps, by card.
	Cards map[CardKey]int
}

// errTradingCrowd is why the state of a crowd that trades is not kept: its wishes, its shelves and
// the state of its traits are not part of CrowdState yet.
var errTradingCrowd = errors.New("crowd: the state of a crowd that trades, with its wishes and shelves, is not kept yet")

// State returns what the crowd holds. It returns errTradingCrowd for a crowd that trades, whose
// state it cannot hold whole.
func (c *Crowd) State() (CrowdState, error) {
	if c.trades() {
		return CrowdState{}, errTradingCrowd
	}
	s := CrowdState{Tick: c.tick, NPCs: make([]NPCState, len(c.npcs))}
	for i, npc := range c.npcs {
		s.NPCs[i] = NPCState{Credit: npc.credit, Cards: npc.collection.Snapshot()}
	}
	return s, nil
}

// Restore sets the crowd to s, which State returned. It returns an error, and changes nothing,
// when s cannot be the state of this crowd: another number of NPCs, a negative tick, a credit
// outside a day or a card held no times. It returns errTradingCrowd for a crowd that trades.
func (c *Crowd) Restore(s CrowdState) error {
	if c.trades() {
		return errTradingCrowd
	}
	if len(s.NPCs) != len(c.npcs) {
		return fmt.Errorf("crowd: the state has %d NPCs, the crowd %d", len(s.NPCs), len(c.npcs))
	}
	if s.Tick < 0 {
		return fmt.Errorf("crowd: tick %d is negative", s.Tick)
	}
	collections := make([]*Collection, len(s.NPCs))
	held := 0
	for i, n := range s.NPCs {
		if n.Credit < 0 || n.Credit >= TicksPerDay {
			return fmt.Errorf("crowd: NPC %d: credit %d is outside 0..%d", i, n.Credit, TicksPerDay-1)
		}
		collections[i] = NewCollection()
		for key, qty := range n.Cards {
			if err := collections[i].Add(key, qty); err != nil {
				return fmt.Errorf("crowd: NPC %d: card %q: %w", i, key.ID, err)
			}
			held += qty
		}
	}
	for i, npc := range c.npcs {
		npc.credit, npc.collection = s.NPCs[i].Credit, collections[i]
	}
	c.tick, c.held = s.Tick, held
	return nil
}

// trades reports whether the crowd holds surplus on shelves or has NPCs that wish for cards.
func (c *Crowd) trades() bool {
	if c.holdTicks > 0 {
		return true
	}
	for _, npc := range c.npcs {
		if npc.Behaviour != nil {
			return true
		}
	}
	return false
}
