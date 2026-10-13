package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestCollectionQuantityCountsOnlyTheGivenFinish(t *testing.T) {
	t.Parallel()
	collection := domain.NewCollection()
	nonFoil := domain.CardKey{ID: "a", Finish: domain.FinishNonFoil}
	foil := domain.CardKey{ID: "a", Finish: domain.FinishFoil}
	if err := collection.Add(nonFoil, 2); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := collection.Add(foil, 1); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if got := collection.Quantity(nonFoil); got != 2 {
		t.Errorf("Quantity(non-foil) = %d, want 2", got)
	}
	if got := collection.Quantity(foil); got != 1 {
		t.Errorf("Quantity(foil) = %d, want 1", got)
	}
	if got := collection.Quantity(domain.CardKey{ID: "b", Finish: domain.FinishNonFoil}); got != 0 {
		t.Errorf("Quantity(absent) = %d, want 0", got)
	}
}
