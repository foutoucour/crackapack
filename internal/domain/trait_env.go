package domain

// NewTraitEnv builds the environment the trait constructors get from the sets the crowd opens. A
// set with no release date has no entry in SetReleased. GeneratePopulation uses it, and a test may
// call it to build a trait with the same view of the cards.
func NewTraitEnv(sets []SetCatalogue, valuer Valuer) TraitEnv {
	cards := make(map[CardID]Card)
	setOf := make(map[CardID]string)
	released := make(map[string]string, len(sets))
	for _, cat := range sets {
		if cat.ReleaseDate != "" {
			released[cat.Code] = cat.ReleaseDate
		}
		for id, card := range cat.Cards {
			cards[id] = card
			setOf[id] = cat.Code
		}
	}
	return TraitEnv{
		Cards: func(id CardID) (Card, bool) {
			card, ok := cards[id]
			return card, ok
		},
		CardSet: func(id CardID) (string, bool) {
			code, ok := setOf[id]
			return code, ok
		},
		SetReleased: func(code string) (string, bool) {
			date, ok := released[code]
			return date, ok
		},
		Valuer: valuer,
	}
}

// anyTraits reports whether at least one archetype lists a trait, so a population without traits
// builds no environment.
func anyTraits(archetypes map[string]ArchetypeSpec) bool {
	for _, spec := range archetypes {
		if len(spec.Traits) > 0 {
			return true
		}
	}
	return false
}
