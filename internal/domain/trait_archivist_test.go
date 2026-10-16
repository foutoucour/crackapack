package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// archivistEnv builds a TraitEnv where cards belong to one of two sets: an old one
// (oldDate) and a new one (newDate). cardInOld belongs to the old set; cardInNew to the new set.
func archivistEnv(oldDate, newDate string) (env domain.TraitEnv, cardInOld, cardInNew domain.Card) {
	oldCat := domaintest.NewRavnicaCatalogue()
	oldCat.ReleaseDate = oldDate

	newCat := domaintest.NewClassicCatalogue()
	newCat.ReleaseDate = newDate

	sets := []domain.SetCatalogue{oldCat, newCat}
	env = domain.NewTraitEnv(sets, domain.Valuer{})

	// Pick one card from each catalogue.
	for id := range oldCat.Cards {
		cardInOld = oldCat.Cards[id]
		break
	}
	for id := range newCat.Cards {
		cardInNew = newCat.Cards[id]
		break
	}
	return env, cardInOld, cardInNew
}

// TestArchivistTrait_olderScoresHigher verifies that a card from an older set scores
// higher than a card from a newer set.
func TestArchivistTrait_olderScoresHigher(t *testing.T) {
	t.Parallel()

	env, cardInOld, cardInNew := archivistEnv("2000-01-01", "2020-01-01")

	reg := domain.DefaultTraits()
	trait, err := reg["archivist"](nil, env)
	if err != nil {
		t.Fatalf("newArchivistTrait: %v", err)
	}

	scorer, ok := trait.(domain.WishScorer)
	if !ok {
		t.Fatal("archivistTrait does not implement WishScorer")
	}

	scoreOld := scorer.WishScore(cardInOld)
	scoreNew := scorer.WishScore(cardInNew)

	if scoreOld <= scoreNew {
		t.Errorf("scoreOld(%s) = %d, scoreNew(%s) = %d: old set should score higher",
			cardInOld.ID, scoreOld, cardInNew.ID, scoreNew)
	}
}

// TestArchivistTrait_undatedReturnsZero verifies that a card whose set has no release
// date returns a WishScore of 0.
func TestArchivistTrait_undatedReturnsZero(t *testing.T) {
	t.Parallel()

	// Build an env where the set has no date.
	cat := domaintest.NewRavnicaCatalogue()
	cat.ReleaseDate = "" // explicitly clear the date
	env := domain.NewTraitEnv([]domain.SetCatalogue{cat}, domain.Valuer{})

	var card domain.Card
	for id := range cat.Cards {
		card = cat.Cards[id]
		break
	}

	reg := domain.DefaultTraits()
	trait, err := reg["archivist"](nil, env)
	if err != nil {
		t.Fatalf("newArchivistTrait: %v", err)
	}
	scorer := trait.(domain.WishScorer)
	if got := scorer.WishScore(card); got != 0 {
		t.Errorf("WishScore for undated set = %d, want 0", got)
	}
}

// TestArchivistTrait_unknownCardReturnsZero verifies that a card not in any set
// returns WishScore 0.
func TestArchivistTrait_unknownCardReturnsZero(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewRavnicaCatalogue()
	cat.ReleaseDate = "2005-10-07"
	env := domain.NewTraitEnv([]domain.SetCatalogue{cat}, domain.Valuer{})

	reg := domain.DefaultTraits()
	trait, err := reg["archivist"](nil, env)
	if err != nil {
		t.Fatalf("newArchivistTrait: %v", err)
	}
	scorer := trait.(domain.WishScorer)
	unknown := domain.Card{ID: domain.CardID("no-such-card")}
	if got := scorer.WishScore(unknown); got != 0 {
		t.Errorf("WishScore for unknown card = %d, want 0", got)
	}
}

// TestArchivistTrait_nilEnvRejected verifies that newArchivistTrait rejects a TraitEnv
// whose CardSet or SetReleased functions are nil.
func TestArchivistTrait_nilEnvRejected(t *testing.T) {
	t.Parallel()

	reg := domain.DefaultTraits()

	// No CardSet.
	_, err := reg["archivist"](nil, domain.TraitEnv{SetReleased: func(string) (string, bool) { return "", false }})
	if err == nil {
		t.Error("want error when CardSet is nil, got nil")
	}

	// No SetReleased.
	_, err = reg["archivist"](nil, domain.TraitEnv{CardSet: func(domain.CardID) (string, bool) { return "", false }})
	if err == nil {
		t.Error("want error when SetReleased is nil, got nil")
	}

	// Both nil.
	_, err = reg["archivist"](nil, domain.TraitEnv{})
	if err == nil {
		t.Error("want error when both CardSet and SetReleased are nil, got nil")
	}
}

// TestArchivistTrait_unknownParameterRejected verifies that newArchivistTrait rejects
// any parameter key.
func TestArchivistTrait_unknownParameterRejected(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewRavnicaCatalogue()
	cat.ReleaseDate = "2005-10-07"
	env := domain.NewTraitEnv([]domain.SetCatalogue{cat}, domain.Valuer{})

	reg := domain.DefaultTraits()
	_, err := reg["archivist"](map[string]int{"strength": 10}, env)
	if err == nil {
		t.Error("want error for unknown parameter, got nil")
	}
}

// TestArchivistTrait_nameIsArchivist verifies the Trait.Name() contract.
func TestArchivistTrait_nameIsArchivist(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewRavnicaCatalogue()
	cat.ReleaseDate = "2005-10-07"
	env := domain.NewTraitEnv([]domain.SetCatalogue{cat}, domain.Valuer{})

	reg := domain.DefaultTraits()
	trait, err := reg["archivist"](nil, env)
	if err != nil {
		t.Fatalf("newArchivistTrait: %v", err)
	}
	if got := trait.Name(); got != "archivist" {
		t.Errorf("Name() = %q, want archivist", got)
	}
}

// TestArchivistTrait_integrationWithGeneratePopulation verifies that an archetype with
// the archivist trait builds NPCs without error when the crowd environment is real.
func TestArchivistTrait_integrationWithGeneratePopulation(t *testing.T) {
	t.Parallel()

	rav := domaintest.NewRavnicaCatalogue() // ReleaseDate = "2005-10-07"
	sets := []domain.SetCatalogue{rav}

	params := domain.PopulationParams{
		Size: 5,
		Archetypes: map[string]domain.ArchetypeSpec{
			"archivist": {
				Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1,
				Behaviour: "casual",
				Traits:    []domain.TraitSpec{{Name: "archivist"}},
			},
		},
		Valuer:       domain.NewValuer(domain.PriceSnapshot{}, 10),
		MaxMarginPct: 100,
	}

	npcs, err := domain.GeneratePopulation(params, sets, testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation with archivist trait: %v", err)
	}
	if len(npcs) != 5 {
		t.Errorf("got %d NPCs, want 5", len(npcs))
	}
}
