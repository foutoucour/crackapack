package play

import (
	"errors"
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ErrCopyLocked is returned when the player would sell or give a copy that a milestone locks.
var ErrCopyLocked = errors.New("play: the copy is locked by a milestone")

// MilestoneRules is what the milestones of a game pay, see docs/core-features.md#milestones. A
// reward is energy, in percent of the reserve cap: with a reserve of 10, 50 pays 5, and 200 pays 20.
// It follows the cap when the cap changes. A milestone that pays 0, or whose kind is missing from
// the rules, pays nothing but still locks its cards.
type MilestoneRules struct {
	// Rarity is what completing every card of a rarity pays, by rarity.
	Rarity map[domain.Rarity]domain.Percent
	// Colour is what completing every card of a colour group pays.
	Colour domain.Percent
}

// energyFor is the energy that the milestone pays under rules at a reserve of reserveCap.
func (r MilestoneRules) energyFor(m domain.Milestone, reserveCap int) int {
	var percent domain.Percent
	switch m.Kind {
	case domain.MilestoneRarity:
		percent = r.Rarity[domain.Rarity(m.Group)]
	case domain.MilestoneColour:
		percent = r.Colour
	case domain.MilestoneBinder:
		// The whole binder opens sets instead of paying energy.
	}
	return int(int64(reserveCap) * int64(percent) / 100)
}

// EarnedMilestone is a milestone the player has just earned, and what it gave.
type EarnedMilestone struct {
	Milestone domain.Milestone
	SetName   string
	// Energy is the energy the reward added to the savings, less than the reward when the savings
	// were nearly full.
	Energy int
	// Unlocked are the sets the milestone opened, which only the milestone of a whole binder does.
	Unlocked []UnlockedSet
}

// milestoneBook is the milestones a game has earned, and the ones the player has not been told of.
type milestoneBook struct {
	rules   MilestoneRules
	earned  map[domain.Milestone]bool
	pending []EarnedMilestone
}

// newMilestoneBook returns the book for rules, or an error for a reward below zero.
func newMilestoneBook(rules MilestoneRules) (*milestoneBook, error) {
	rewards := map[string]domain.Percent{"colour": rules.Colour}
	for rarity, percent := range rules.Rarity {
		rewards[string(rarity)] = percent
	}
	for name, percent := range rewards {
		if percent < 0 {
			return nil, fmt.Errorf("play: the milestone reward %q is %d, which is below zero", name, percent)
		}
	}
	return &milestoneBook{rules: rules, earned: make(map[domain.Milestone]bool)}, nil
}

// MilestoneStatus is a goal of a binder, whether the player has earned it and what it pays.
type MilestoneStatus struct {
	Milestone domain.Milestone
	Done      bool
	// Reward is the energy the milestone pays, which is 0 for the whole binder: it opens sets instead.
	Reward int
}

// Milestones lists the goals of the binder of setCode, the rarities first, then the colour groups
// and the whole binder. It lists none in a game without milestones, and returns ErrUnknownSet,
// wrapped, for a set the game does not have.
func (g *Game) Milestones(setCode string) ([]MilestoneStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	binder, ok := g.binders[setCode]
	if !ok {
		return nil, fmt.Errorf("milestones of %q: %w", setCode, ErrUnknownSet)
	}
	if g.milestones == nil {
		return nil, nil
	}
	var out []MilestoneStatus
	for _, m := range domain.MilestonesOf(binder) {
		out = append(out, MilestoneStatus{
			Milestone: m, Done: g.milestones.earned[m],
			Reward: g.milestones.rules.energyFor(m, g.rules.Energy.ReserveCap),
		})
	}
	return out, nil
}

// TakeMilestones returns the milestones earned since the last call, oldest first, and forgets them:
// a client tells the player of each once. It returns none in a game without milestones.
func (g *Game) TakeMilestones() []EarnedMilestone {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.milestones == nil {
		return nil
	}
	taken := g.milestones.pending
	g.milestones.pending = nil
	return taken
}

// Locked returns how many copies of the card id in finish a milestone locks: 1 for the copy kept
// when the card entered a milestone, and 0 for any other.
func (g *Game) Locked(id domain.CardID, finish domain.Finish) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.collection.Locked(domain.CardKey{ID: id, Finish: finish})
}

// unspendable is the copies of key that the player cannot sell or give: the ones the market holds for a
// listing or a counter, and the one a milestone locks. g.mu must be held.
func (g *Game) unspendable(key domain.CardKey) int {
	return g.reserved(key) + g.collection.Locked(key)
}

// settleMilestones earns the milestones that the collection now completes, once each. It locks one
// copy of every card of a milestone, pays its reward and queues it for TakeMilestones. A card whose
// every copy the market holds does not count, so that the player cannot list the last copy of a
// card and keep the reward. Every save settles, since every method that changes the collection
// saves. It reports whether anything was earned. g.mu must be held.
func (g *Game) settleMilestones() bool {
	book := g.milestones
	if book == nil {
		return false
	}
	var earned bool
	for _, cat := range g.sets {
		binder := g.binders[cat.Code]
		for _, m := range domain.MilestonesOf(binder) {
			if book.earned[m] {
				continue
			}
			locks, complete := g.copiesToLock(m.Cards(binder))
			if !complete {
				continue
			}
			for _, key := range locks {
				// Lock cannot fail: copiesToLock found the copy held.
				_ = g.collection.Lock(key)
			}
			book.earned[m] = true
			book.pending = append(book.pending, g.payMilestone(m, cat.Name))
			earned = true
		}
	}
	return earned
}

// copiesToLock lists the copy to lock of each of cards that no milestone locked yet, the non-foil
// first so that a foil stays free to trade, and reports whether the player holds all of them. g.mu
// must be held.
func (g *Game) copiesToLock(cards []domain.Card) (locks []domain.CardKey, complete bool) {
	for _, card := range cards {
		if g.collection.IsLocked(card.ID) {
			continue
		}
		var found bool
		for _, finish := range []domain.Finish{domain.FinishNonFoil, domain.FinishFoil} {
			key := domain.CardKey{ID: card.ID, Finish: finish}
			if g.collection.Quantity(key)-g.reserved(key) >= 1 {
				locks, found = append(locks, key), true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return locks, true
}

// payMilestone gives the reward of m: its energy, and for the whole binder the sets it unlocks. g.mu
// must be held.
func (g *Game) payMilestone(m domain.Milestone, setName string) EarnedMilestone {
	earned := EarnedMilestone{Milestone: m, SetName: setName}
	if reward := g.milestones.rules.energyFor(m, g.rules.Energy.ReserveCap); reward > 0 {
		g.sync()
		// Grant fails only for a reward below one, which the test above excludes.
		earned.Energy, _ = g.energy.Grant(reward)
	}
	if m.Kind == domain.MilestoneBinder && g.staging != nil {
		if earned.Unlocked = g.unlockDrawn(); len(earned.Unlocked) > 0 {
			g.openCrowdSets(g.economy)
		}
	}
	return earned
}
