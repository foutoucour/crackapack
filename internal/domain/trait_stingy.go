package domain

import (
	"fmt"
	"maps"
	"slices"
)

// stingyTrait adds a margin based on the rarity of the highest-ranked card the NPC gives.
// It implements MarginAdder only; it does not implement WishScorer, CounterVeto or DeclineReactor.
// The add is the MAX across all cards in the offer's NPCGives, so giving one mythic beats giving
// ten commons and the per-offer add does not grow unpredictably with card count.
type stingyTrait struct {
	cards func(CardID) (Card, bool)
	// table maps each rarity to the percent add the trait applies.
	table map[Rarity]Percent
}

// newStingyTrait builds a stingyTrait from params and env.
// Accepted parameters (percent adds; default 0 when absent): common_pct, uncommon_pct, rare_pct,
// special_pct, mythic_pct. RaritySpecial is treated as the rare tier (collectorRank 2).
// Errors: negative value, unknown parameter name, nil env.Cards.
func newStingyTrait(params map[string]int, env TraitEnv) (Trait, error) {
	if env.Cards == nil {
		return nil, fmt.Errorf("stingy: env.Cards is nil")
	}
	knownNames := []string{"common_pct", "mythic_pct", "rare_pct", "special_pct", "uncommon_pct"}
	for _, name := range slices.Sorted(maps.Keys(params)) {
		if !slices.Contains(knownNames, name) {
			return nil, fmt.Errorf("stingy: unknown parameter %q, known: %v", name, knownNames)
		}
		if params[name] < 0 {
			return nil, fmt.Errorf("stingy: parameter %q must be >= 0, got %d", name, params[name])
		}
	}
	table := map[Rarity]Percent{
		RarityCommon:   Percent(params["common_pct"]),
		RarityUncommon: Percent(params["uncommon_pct"]),
		RarityRare:     Percent(params["rare_pct"]),
		RaritySpecial:  Percent(params["special_pct"]),
		RarityMythic:   Percent(params["mythic_pct"]),
	}
	return &stingyTrait{cards: env.Cards, table: table}, nil
}

func (s *stingyTrait) Name() string { return "stingy" }

// MarginAdd returns the maximum rarity-based percent add across the cards the NPC gives.
// A card not found in the catalogue adds 0. An unknown rarity maps to 0. An empty NPCGives
// returns 0. ctx.NPC is never read; nil is fine.
func (s *stingyTrait) MarginAdd(ctx MarginContext) Percent {
	var highest Percent
	for _, l := range ctx.Offer.NPCGives {
		card, ok := s.cards(l.Key.ID)
		if !ok {
			continue
		}
		highest = max(highest, s.table[card.Rarity])
	}
	return highest
}
