package domain_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func tiers() []domain.CostTier {
	return []domain.CostTier{{Threshold: 50, Cost: 3}, {Threshold: 100, Cost: 5}, {Threshold: 200, Cost: 8}, {Threshold: 400, Cost: 12}}
}

func TestBoosterEnergyCosts_followEachBoostersValueAgainstTheReferenceMedian(t *testing.T) {
	t.Parallel()
	rav, cls := domaintest.NewRavnicaCatalogue(), domaintest.NewClassicCatalogue()
	sets := []domain.SetCatalogue{rav, cls}
	prices := domaintest.MergePrices(domaintest.NewPrices(rav, 1), domaintest.NewPrices(cls, 2))
	const fallback = domain.USDCents(1)

	got, err := domain.BoosterEnergyCosts(sets, sets, prices, fallback, tiers())
	if err != nil {
		t.Fatalf("BoosterEnergyCosts: %v", err)
	}

	// The same computation by hand, with the building blocks it is made of.
	values := make([]domain.USDCents, len(sets))
	for i, cat := range sets {
		if values[i], err = domain.BoosterAverageValue(cat, prices, fallback); err != nil {
			t.Fatalf("BoosterAverageValue: %v", err)
		}
	}
	median, err := domain.MedianValue(values)
	if err != nil {
		t.Fatalf("MedianValue: %v", err)
	}
	for i, cat := range sets {
		want, err := domain.EnergyCost(values[i], median, tiers())
		if err != nil {
			t.Fatalf("EnergyCost: %v", err)
		}
		if got[cat.Code] != want {
			t.Errorf("cost of %s = %d, want %d", cat.Code, got[cat.Code], want)
		}
	}
	if len(got) != len(sets) {
		t.Errorf("%d costs for %d sets: %v", len(got), len(sets), got)
	}
}

func TestBoosterEnergyCosts_theReferenceDecidesNotTheSetsBeingPlayed(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	cls := domaintest.NewClassicCatalogue()
	prices := domaintest.MergePrices(domaintest.NewPrices(rav, 1), domaintest.NewPrices(cls, 2))
	const fallback = domain.USDCents(1)

	alone, err := domain.BoosterEnergyCosts([]domain.SetCatalogue{rav}, nil, prices, fallback, tiers())
	if err != nil {
		t.Fatalf("alone: %v", err)
	}
	// A set that is its own reference is always at ratio 1, the x1 tier.
	if alone[rav.Code] != 5 {
		t.Errorf("a set with no reference costs %d, want the x1 tier, 5", alone[rav.Code])
	}

	// Against a reference of cheap sets, the same booster is dear.
	cheap := cls
	cheapPrices := domain.PriceSnapshot{}
	for id := range cheap.Cards {
		cheapPrices[id] = 1
	}
	for id, p := range domaintest.NewPrices(rav, 1) {
		cheapPrices[id] = p * 1000
	}
	dear, err := domain.BoosterEnergyCosts([]domain.SetCatalogue{rav}, []domain.SetCatalogue{cls, cls, rav}, cheapPrices, fallback, tiers())
	if err != nil {
		t.Fatalf("dear: %v", err)
	}
	if dear[rav.Code] <= alone[rav.Code] {
		t.Errorf("against a cheaper reference RAV costs %d, want more than %d", dear[rav.Code], alone[rav.Code])
	}
}

func TestBoosterEnergyCosts_namesTheSetThatCannotBeValued(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	rav.BoosterOptions = nil

	_, err := domain.BoosterEnergyCosts([]domain.SetCatalogue{rav}, nil, domain.PriceSnapshot{}, 1, tiers())

	if err == nil {
		t.Fatal("a set with no booster definition returned no error")
	}
	if got := err.Error(); !strings.Contains(got, rav.Code) {
		t.Errorf("error %q does not name the set %s", got, rav.Code)
	}
}
