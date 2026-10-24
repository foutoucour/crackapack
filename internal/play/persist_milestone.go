package play

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// MilestonesSnapshot is the milestones a game has earned, and the cards it locked for them. The
// locks are listed by card, in the finish of the copy kept, since the cards run to thousands.
type MilestonesSnapshot struct {
	Earned []MilestoneSnapshot `json:"earned,omitempty"`
	// Locked are the cards whose non-foil copy is locked, and LockedFoil those whose foil is.
	Locked     []domain.CardID `json:"locked,omitempty"`
	LockedFoil []domain.CardID `json:"locked_foil,omitempty"`
}

// MilestoneSnapshot is one milestone earned: the set, the kind and the group it names.
type MilestoneSnapshot struct {
	Set   string               `json:"set"`
	Kind  domain.MilestoneKind `json:"kind"`
	Group string               `json:"group,omitempty"`
}

// snapshotMilestones is the milestones of the game, nil in a game that has none and never had any.
// A game without milestones keeps what its save held. g.mu must be held.
func (g *Game) snapshotMilestones() *MilestonesSnapshot {
	book := g.milestones
	if book == nil {
		return g.carriedMilestones
	}
	snap := &MilestonesSnapshot{}
	for m := range book.earned {
		snap.Earned = append(snap.Earned, MilestoneSnapshot{Set: m.Set, Kind: m.Kind, Group: m.Group})
	}
	slices.SortFunc(snap.Earned, func(a, b MilestoneSnapshot) int {
		return cmp.Or(cmp.Compare(a.Set, b.Set), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Group, b.Group))
	})
	for _, key := range g.collection.LockedKeys() {
		if key.Finish == domain.FinishFoil {
			snap.LockedFoil = append(snap.LockedFoil, key.ID)
		} else {
			snap.Locked = append(snap.Locked, key.ID)
		}
	}
	return snap
}

// restoreMilestones checks the milestones of snap against collection, which the game will hold, and
// returns what puts them in the game. A game without milestones keeps snap as it is. It returns an
// error when snap locks a copy that collection lacks. Nothing changes until commit is called, so
// that a save refused later leaves the game as it was. g.mu must be held.
func (g *Game) restoreMilestones(snap MilestonesSnapshot, collection *domain.Collection) (commit func(), err error) {
	if g.milestones == nil {
		return func() { g.carriedMilestones = &snap }, nil
	}
	var locks []domain.CardKey
	for _, group := range []struct {
		ids    []domain.CardID
		finish domain.Finish
	}{{snap.Locked, domain.FinishNonFoil}, {snap.LockedFoil, domain.FinishFoil}} {
		for _, id := range group.ids {
			key := domain.CardKey{ID: id, Finish: group.finish}
			if collection.Quantity(key) < 1 {
				return nil, fmt.Errorf("milestones: the save locks a copy of %q (%s) that it does not hold", id, group.finish)
			}
			locks = append(locks, key)
		}
	}
	return func() {
		for _, key := range locks {
			// Lock cannot fail: the copy is held, checked above.
			_ = collection.Lock(key)
		}
		for _, m := range snap.Earned {
			g.milestones.earned[domain.Milestone{Set: m.Set, Kind: m.Kind, Group: m.Group}] = true
		}
	}, nil
}
