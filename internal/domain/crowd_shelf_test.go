package domain_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func shelfTotal(npc *domain.NPC) int {
	var total int
	for _, line := range npc.Shelf() {
		total += line.Qty
	}
	return total
}

func TestCrowdKeepsSurplusOnTheShelfForTheHold(t *testing.T) {
	t.Parallel()
	crowd, npcs, counter := newTestCrowd(t, 1, fixedSpec(10, 1), crowdSets(), 1, domain.WithSurplusHold(2))

	for day := 1; day <= 2; day++ {
		runDays(t, crowd, 1)
		if got := totalStock(counter); got != 0 {
			t.Fatalf("day %d: the Counter holds %d cards, want none: surplus is held for 2 days", day, got)
		}
	}
	if shelfTotal(npcs[0]) == 0 {
		t.Fatal("no surplus on the shelf after two days")
	}

	runDays(t, crowd, 1)
	if totalStock(counter) == 0 {
		t.Error("day 3: nothing reached the Counter, but the day-1 surplus is past its hold")
	}
}

func TestCrowdCardsHeldCountsCollectionsAndShelves(t *testing.T) {
	t.Parallel()
	for _, hold := range []int{0, 2} {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			crowd, npcs, _ := newTestCrowd(t, 2, fixedSpec(8, 1), crowdSets(), 3, domain.WithSurplusHold(hold))
			var drawn, sold int
			for range 4 {
				tick := runDays(t, crowd, 1)
				drawn += tick.CardsDrawn
				sold += tick.CardsSold
			}

			var held int
			for _, npc := range npcs {
				held += totalCopies(npc.Collection()) + shelfTotal(npc)
			}
			if got := crowd.CardsHeld(); got != held {
				t.Errorf("hold %d: CardsHeld = %d, want %d (collections plus shelves)", hold, got, held)
			}
			if drawn-sold != held {
				t.Errorf("hold %d: drawn %d - sold %d = %d, want %d held", hold, drawn, sold, drawn-sold, held)
			}
		})
	}
}

func TestCrowdHoldersIndexMatchesTheShelves(t *testing.T) {
	t.Parallel()
	crowd, npcs, _ := newTestCrowd(t, 5, fixedSpec(10, 1), crowdSets(), 4, domain.WithSurplusHold(1))
	runDays(t, crowd, 4)

	for _, cat := range crowdSets() {
		for id := range cat.Cards {
			for _, finish := range []domain.Finish{domain.FinishNonFoil, domain.FinishFoil} {
				key := domain.CardKey{ID: id, Finish: finish}
				var want []int
				for _, npc := range npcs {
					if npc.ShelfQuantity(key) > 0 {
						want = append(want, npc.ID)
					}
				}
				if got := crowd.HoldersOf(key); !slices.Equal(got, want) {
					t.Errorf("HoldersOf(%v) = %v, want %v", key, got, want)
				}
			}
		}
	}
}

func TestCrowdFillsAndRefillsWishLists(t *testing.T) {
	t.Parallel()
	spec := fixedSpec(10, 1)
	spec.Behaviour = "casual"
	spec.WishSize = 3
	crowd, npcs, _ := newTestCrowd(t, 4, spec, crowdSets(), 5, domain.WithSurplusHold(2))

	for _, npc := range npcs {
		if got := len(npc.Wishes()); got != 3 {
			t.Fatalf("NPC %d starts with %d wishes, want 3", npc.ID, got)
		}
	}
	runDays(t, crowd, 5)

	wishedBy := map[domain.CardID][]int{}
	for _, npc := range npcs {
		wishes := npc.Wishes()
		if len(wishes) > 3 {
			t.Errorf("NPC %d has %d wishes, want at most 3", npc.ID, len(wishes))
		}
		for _, w := range wishes {
			if npc.Collection().Copies(w.ID) >= w.Want {
				t.Errorf("NPC %d still wishes %q, which it holds %d times", npc.ID, w.ID, npc.Collection().Copies(w.ID))
			}
			wishedBy[w.ID] = append(wishedBy[w.ID], npc.ID)
		}
	}
	for _, cat := range crowdSets() {
		for id := range cat.Cards {
			if got := crowd.WishersForTest(id); !slices.Equal(got, wishedBy[id]) {
				t.Errorf("wishers index of %q = %v, want %v", id, got, wishedBy[id])
			}
		}
	}
}

func TestCrowdWithoutBehavioursHasNoWishesAndNoShelf(t *testing.T) {
	t.Parallel()
	plainCrowd, plainNPCs, _ := newTestCrowd(t, 3, fixedSpec(10, 1), crowdSets(), 8)
	runDays(t, plainCrowd, 3)

	for _, npc := range plainNPCs {
		if len(npc.Wishes()) != 0 {
			t.Errorf("NPC %d has wishes without a behaviour", npc.ID)
		}
		if len(npc.Shelf()) != 0 {
			t.Errorf("NPC %d has a shelf without a surplus hold", npc.ID)
		}
	}
}
