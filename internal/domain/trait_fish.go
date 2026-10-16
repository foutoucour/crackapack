package domain

import (
	"fmt"
	"maps"
	"slices"
)

// fishTrait scores cards by price and shininess for the NPC's wish list. It implements WishScorer
// only; a fish NPC's margin comes entirely from its archetype file (margin_pct 0). The speculator
// base then asks for the dearest duplicates first (its Propose sorts ctx.Wanted by price desc).
// See ADR-0021 for how scorers compose with the base behaviour.
type fishTrait struct {
	valuer         Valuer
	priceUnitCents int
	rarityPoints   int
	foilPoints     int
}

// newFishTrait builds a fishTrait from params and env.
// Parameters:
//   - price_unit_cents: price is divided by this to get a score unit (default 100; explicit 0
//     is rejected because it would divide by zero)
//   - rarity_points: collectorRank(rarity) is multiplied by this and added to the score (default 0)
//   - foil_points: added when the card has a foil finish (default 0)
//
// Errors: negative value, price_unit_cents explicitly 0, unknown parameter name.
func newFishTrait(params map[string]int, env TraitEnv) (Trait, error) {
	knownNames := []string{"foil_points", "price_unit_cents", "rarity_points"}
	for _, name := range slices.Sorted(maps.Keys(params)) {
		if !slices.Contains(knownNames, name) {
			return nil, fmt.Errorf("fish: unknown parameter %q, known: %v", name, knownNames)
		}
		if params[name] < 0 {
			return nil, fmt.Errorf("fish: parameter %q must be >= 0, got %d", name, params[name])
		}
	}
	priceUnit := 100
	if v, ok := params["price_unit_cents"]; ok {
		if v == 0 {
			return nil, fmt.Errorf("fish: price_unit_cents must be > 0")
		}
		priceUnit = v
	}
	return &fishTrait{
		valuer:         env.Valuer,
		priceUnitCents: priceUnit,
		rarityPoints:   params["rarity_points"],
		foilPoints:     params["foil_points"],
	}, nil
}

func (f *fishTrait) Name() string { return "fish" }

// WishScore returns the score for a card: its price divided by price_unit_cents, plus
// collectorRank(rarity) * rarity_points, plus foil_points when the card has a foil finish.
func (f *fishTrait) WishScore(card Card) int {
	score := int(f.valuer.Price(card.ID)) / f.priceUnitCents
	score += collectorRank(card.Rarity) * f.rarityPoints
	for _, fin := range card.Finishes {
		if fin == FinishFoil {
			score += f.foilPoints
			break
		}
	}
	return score
}
