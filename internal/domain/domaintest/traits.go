package domaintest

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ScoreTrait is a test trait that scores cards from a fixed map. Cards not in the map score 0.
type ScoreTrait struct {
	scores map[domain.CardID]int
}

// NewScoreTrait returns a TraitConstructor that builds a ScoreTrait using the provided scores map.
func NewScoreTrait(scores map[domain.CardID]int) domain.TraitConstructor {
	return func(_ map[string]int, _ domain.TraitEnv) (domain.Trait, error) {
		return &ScoreTrait{scores: scores}, nil
	}
}

func (s *ScoreTrait) Name() string { return "score" }

// WishScore returns the score for card, or 0 if not in the map.
func (s *ScoreTrait) WishScore(card domain.Card) int { return s.scores[card.ID] }

// MarginTrait is a test trait that adds a fixed number of percent to the margin on each offer.
type MarginTrait struct {
	add domain.Percent
}

// NewMarginTrait returns a TraitConstructor that builds a MarginTrait reading "add" from params.
// It rejects a negative "add" value.
func NewMarginTrait() domain.TraitConstructor {
	return func(params map[string]int, _ domain.TraitEnv) (domain.Trait, error) {
		add := params["add"]
		if add < 0 {
			return nil, fmt.Errorf("margin trait: add must be >= 0, got %d", add)
		}
		return &MarginTrait{add: domain.Percent(add)}, nil
	}
}

func (m *MarginTrait) Name() string { return "margin" }

// MarginAdd returns the fixed add for any offer.
func (m *MarginTrait) MarginAdd(_ domain.MarginContext) domain.Percent { return m.add }

// VetoTrait is a test trait that always vetoes a player counter-offer.
type VetoTrait struct{}

// NewVetoTrait returns a TraitConstructor that builds a VetoTrait.
func NewVetoTrait() domain.TraitConstructor {
	return func(_ map[string]int, _ domain.TraitEnv) (domain.Trait, error) {
		return &VetoTrait{}, nil
	}
}

func (v *VetoTrait) Name() string { return "veto" }

// VetoCounter always returns true.
func (v *VetoTrait) VetoCounter(_ domain.Offer) bool { return true }

// DeclineCall records one call to ReactToDecline.
type DeclineCall struct {
	Context domain.DeclineContext
	// Streak is ctx.NPC.DeclineStreak() captured at call time, proving increment-then-notify.
	// It is 0 when ctx.NPC is nil.
	Streak int
}

// DeclineSpy is a test trait that records every DeclineContext passed to ReactToDecline.
type DeclineSpy struct {
	Calls []DeclineCall
}

// NewDeclineSpy returns a TraitConstructor that records calls into spy. The same spy pointer is
// returned each time, so tests can reach it after the behaviour is built.
func NewDeclineSpy(spy *DeclineSpy) domain.TraitConstructor {
	return func(_ map[string]int, _ domain.TraitEnv) (domain.Trait, error) {
		return spy, nil
	}
}

func (d *DeclineSpy) Name() string { return "decline_spy" }

// ReactToDecline records the context and the streak captured at call time.
func (d *DeclineSpy) ReactToDecline(ctx domain.DeclineContext) {
	var streak int
	if ctx.NPC != nil {
		streak = ctx.NPC.DeclineStreak()
	}
	d.Calls = append(d.Calls, DeclineCall{Context: ctx, Streak: streak})
}

// TraitRegistry builds a TraitRegistry from the provided name→constructor pairs.
func TraitRegistry(pairs ...TraitEntry) domain.TraitRegistry {
	r := make(domain.TraitRegistry, len(pairs))
	for _, p := range pairs {
		r[p.Name] = p.Constructor
	}
	return r
}

// TraitEntry is a name and constructor pair for TraitRegistry.
type TraitEntry struct {
	Name        string
	Constructor domain.TraitConstructor
}
