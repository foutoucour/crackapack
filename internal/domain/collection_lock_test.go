package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestLockKeepsOneCopyOutOfRemove(t *testing.T) {
	t.Parallel()
	collection := domain.NewCollection()
	key := domain.CardKey{ID: "a", Finish: domain.FinishNonFoil}
	if err := collection.Add(key, 3); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := collection.Lock(key); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if got := collection.Locked(key); got != 1 {
		t.Errorf("Locked = %d, want 1", got)
	}
	if err := collection.Remove(key, 2); err != nil {
		t.Errorf("Remove of the two free copies: %v", err)
	}
	err := collection.Remove(key, 1)
	if !errors.Is(err, domain.ErrLockedCopy) {
		t.Errorf("Remove of the locked copy = %v, want ErrLockedCopy", err)
	}
	if got := collection.Quantity(key); got != 1 {
		t.Errorf("Quantity = %d, want 1: the refused Remove must change nothing", got)
	}
}

func TestLockIsOneCopyPerCardWhateverTheFinish(t *testing.T) {
	t.Parallel()
	collection := domain.NewCollection()
	nonFoil := domain.CardKey{ID: "a", Finish: domain.FinishNonFoil}
	foil := domain.CardKey{ID: "a", Finish: domain.FinishFoil}
	for _, key := range []domain.CardKey{nonFoil, foil} {
		if err := collection.Add(key, 1); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := collection.Lock(nonFoil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if err := collection.Lock(foil); err != nil {
		t.Fatalf("Lock of a card that is locked already: %v", err)
	}

	if collection.Locked(foil) != 0 || collection.Locked(nonFoil) != 1 {
		t.Errorf("Locked(foil, non-foil) = %d, %d; want 0, 1: the first lock stays", collection.Locked(foil), collection.Locked(nonFoil))
	}
}

func TestLockRefusesACopyThePlayerLacks(t *testing.T) {
	t.Parallel()
	collection := domain.NewCollection()

	err := collection.Lock(domain.CardKey{ID: "a", Finish: domain.FinishFoil})

	if !errors.Is(err, domain.ErrNotHeldToLock) {
		t.Errorf("Lock of an absent card = %v, want ErrNotHeldToLock", err)
	}
}

func TestLockedKeysListsTheLocksInKeyOrder(t *testing.T) {
	t.Parallel()
	collection := domain.NewCollection()
	b := domain.CardKey{ID: "b", Finish: domain.FinishNonFoil}
	a := domain.CardKey{ID: "a", Finish: domain.FinishFoil}
	for _, key := range []domain.CardKey{b, a} {
		if err := collection.Add(key, 1); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := collection.Lock(key); err != nil {
			t.Fatalf("Lock: %v", err)
		}
	}

	got := collection.LockedKeys()

	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Errorf("LockedKeys = %v, want [%v %v]", got, a, b)
	}
}
