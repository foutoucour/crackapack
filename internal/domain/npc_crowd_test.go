package domain_test

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func crowdSets() []domain.SetCatalogue {
	return []domain.SetCatalogue{domaintest.NewRavnicaCatalogue(), domaintest.NewClassicCatalogue()}
}

// newTestCrowd builds a crowd of size NPCs of a single archetype that opens boosters from sets,
// together with the Counter that receives their surplus. The Counter starts with no stock.
func newTestCrowd(t *testing.T, size int, spec domain.ArchetypeSpec, sets []domain.SetCatalogue, seed uint64, opts ...domain.CrowdOption) (*domain.Crowd, []*domain.NPC, *domain.Counter) {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, seed))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: size, Archetypes: map[string]domain.ArchetypeSpec{"npc": spec}}, sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 10})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	crowd, err := domain.NewCrowd(npcs, sets, opener, counter, rng, opts...)
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	return crowd, npcs, counter
}

func runDays(t *testing.T, crowd *domain.Crowd, days int) domain.CrowdTick {
	t.Helper()
	var total domain.CrowdTick
	for range days * domain.TicksPerDay {
		tick, err := crowd.Tick()
		if err != nil {
			t.Fatalf("Tick: %v", err)
		}
		total.BoostersOpened += tick.BoostersOpened
		total.CardsDrawn += tick.CardsDrawn
		total.CardsSold += tick.CardsSold
	}
	return total
}

func fixedSpec(boostersPerDay, keep int) domain.ArchetypeSpec {
	return domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: boostersPerDay, MaxBoostersPerDay: boostersPerDay, KeepCopies: keep}
}

func totalCopies(c *domain.Collection) int {
	var total int
	for _, qty := range c.Snapshot() {
		total += qty
	}
	return total
}

func totalStock(c *domain.Counter) int {
	var total int
	for _, qty := range c.StockByRarity() {
		total += qty
	}
	return total
}

func TestCrowdTickOpensExactlyTheDailyRhythm(t *testing.T) {
	t.Parallel()

	for _, perDay := range []int{1, 10, domain.TicksPerDay, 200} {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			crowd, _, _ := newTestCrowd(t, 1, fixedSpec(perDay, 1), crowdSets(), 1)

			for day := 1; day <= 2; day++ {
				got := runDays(t, crowd, 1).BoostersOpened
				if got != perDay {
					t.Errorf("rhythm %d/day: day %d opened %d boosters, want %d", perDay, day, got, perDay)
				}
			}
		})
	}
}

func TestCrowdTickSellsOnlyTheSurplusAboveTheKeepThreshold(t *testing.T) {
	t.Parallel()

	for _, keep := range []int{1, 2, domain.PlaysetSize} {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			crowd, npcs, counter := newTestCrowd(t, 5, fixedSpec(30, keep), crowdSets(), 2)

			total := runDays(t, crowd, 1)

			if total.CardsSold == 0 {
				t.Fatalf("keep %d: nothing sold after %d boosters, the test cannot show the rule", keep, total.BoostersOpened)
			}
			var held int
			for _, n := range npcs {
				held += totalCopies(n.Collection())
				for key := range n.Collection().Snapshot() {
					if copies := n.Collection().Copies(key.ID); copies > keep {
						t.Errorf("keep %d: NPC %d holds %d copies of %s", keep, n.ID, copies, key.ID)
					}
				}
			}
			if held+total.CardsSold != total.CardsDrawn {
				t.Errorf("keep %d: held %d + sold %d != drawn %d: cards were lost or invented", keep, held, total.CardsSold, total.CardsDrawn)
			}
			if got := totalStock(counter); got != total.CardsSold {
				t.Errorf("keep %d: Counter stock = %d, want the %d cards sold", keep, got, total.CardsSold)
			}
		})
	}
}

func TestCrowdTickKeepsTheThresholdOfEveryPrintingItSold(t *testing.T) {
	t.Parallel()

	for _, keep := range []int{1, 2, domain.PlaysetSize} {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			sets := crowdSets()
			crowd, npcs, counter := newTestCrowd(t, 1, fixedSpec(60, keep), sets, 6)

			runDays(t, crowd, 1)

			var checked int
			for _, cat := range sets {
				for id := range cat.Cards {
					sold := counter.Stock(domain.CardKey{ID: id, Finish: domain.FinishNonFoil}) +
						counter.Stock(domain.CardKey{ID: id, Finish: domain.FinishFoil})
					if sold == 0 {
						continue
					}
					checked++
					if held := npcs[0].Collection().Copies(id); held != keep {
						t.Errorf("keep %d: %s was sold %d times but %d copies are held, want exactly %d kept", keep, id, sold, held, keep)
					}
				}
			}
			if checked == 0 {
				t.Fatalf("keep %d: nothing sold, the test cannot show the rule", keep)
			}
		})
	}
}

func TestCrowdTickSendsTheSurplusAsFoilWhenTheDrawnCopyIsFoil(t *testing.T) {
	t.Parallel()

	crowd, _, counter := newTestCrowd(t, 5, fixedSpec(40, 1), crowdSets(), 3)

	runDays(t, crowd, 1)

	var foils int
	for _, cat := range crowdSets() {
		for id := range cat.Cards {
			foils += counter.Stock(domain.CardKey{ID: id, Finish: domain.FinishFoil})
		}
	}
	if foils == 0 {
		t.Error("no foil reached the Counter: the sold copy must keep the finish it was drawn in")
	}
}

func TestCrowdTickFocusNPCsOpenOnlyTheirSet(t *testing.T) {
	t.Parallel()

	sets := crowdSets()
	spec := fixedSpec(20, 1)
	spec.FocusSet = domain.FocusSetRandom
	crowd, npcs, _ := newTestCrowd(t, 12, spec, sets, 4)

	runDays(t, crowd, 1)

	byCode := map[string]domain.SetCatalogue{}
	for _, cat := range sets {
		byCode[cat.Code] = cat
	}
	focuses := map[string]bool{}
	for _, n := range npcs {
		focuses[n.FocusSet] = true
		cat := byCode[n.FocusSet]
		for key := range n.Collection().Snapshot() {
			if _, ok := cat.Cards[key.ID]; !ok {
				t.Errorf("NPC %d focused on %s holds %s from another set", n.ID, n.FocusSet, key.ID)
			}
		}
	}
	if len(focuses) != len(sets) {
		t.Fatalf("focus sets drawn = %v, want both sets covered so the test is meaningful", focuses)
	}
}

func TestCrowdTickUnfocusedNPCsOpenEverySet(t *testing.T) {
	t.Parallel()

	sets := crowdSets()
	crowd, npcs, _ := newTestCrowd(t, 1, fixedSpec(100, domain.PlaysetSize), sets, 5)

	runDays(t, crowd, 1)

	for _, cat := range sets {
		var held bool
		for key := range npcs[0].Collection().Snapshot() {
			if _, ok := cat.Cards[key.ID]; ok {
				held = true
			}
		}
		if !held {
			t.Errorf("after 100 boosters the NPC holds nothing from %s", cat.Code)
		}
	}
}

// A crowd told to open only some sets opens only those, unless an NPC focuses on another, and
// every set again once told none.
func TestCrowdOpenOnly(t *testing.T) {
	t.Parallel()
	sets := crowdSets()
	crowd, npcs, _ := newTestCrowd(t, 1, fixedSpec(100, domain.PlaysetSize), sets, 5)
	crowd.OpenOnly(func(code string) bool { return code == sets[1].Code })
	runDays(t, crowd, 1)
	for key := range npcs[0].Collection().Snapshot() {
		if _, ok := sets[1].Cards[key.ID]; !ok {
			t.Fatalf("the NPC holds %s, from a set it was not to open", key.ID)
		}
	}
	crowd.OpenOnly(func(string) bool { return false })
	runDays(t, crowd, 1)
	held := false
	for key := range npcs[0].Collection().Snapshot() {
		if _, ok := sets[0].Cards[key.ID]; ok {
			held = true
		}
	}
	if !held {
		t.Error("told no set, the NPC still opens none of the first set")
	}
}

func TestCrowdTickIsReproducibleFromTheSeed(t *testing.T) {
	t.Parallel()

	run := func(seed uint64) (map[domain.Rarity]int, map[domain.CardKey]int) {
		crowd, npcs, counter := newTestCrowd(t, 4, fixedSpec(12, 2), crowdSets(), seed)
		runDays(t, crowd, 2)
		return counter.StockByRarity(), npcs[3].Collection().Snapshot()
	}

	stockA, heldA := run(9)
	stockB, heldB := run(9)

	if !reflect.DeepEqual(stockA, stockB) || !reflect.DeepEqual(heldA, heldB) {
		t.Error("same seed produced different crowds")
	}
}

func TestNewCrowdRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	sets := crowdSets()
	rng := rand.New(rand.NewPCG(1, 1))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"npc": fixedSpec(1, 1)}}, sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	focused := []*domain.NPC{{ID: 0, Archetype: "npc", BoostersPerDay: 1, KeepCopies: 1, FocusSet: "ZZZ"}}

	tests := []struct {
		name    string
		npcs    []*domain.NPC
		sets    []domain.SetCatalogue
		opener  *domain.Opener
		counter *domain.Counter
		rng     *rand.Rand
	}{
		{name: "nil opener", npcs: npcs, sets: sets, counter: counter, rng: rng},
		{name: "nil counter", npcs: npcs, sets: sets, opener: opener, rng: rng},
		{name: "nil rng", npcs: npcs, sets: sets, opener: opener, counter: counter},
		{name: "NPCs without any set", npcs: npcs, opener: opener, counter: counter, rng: rng},
		{name: "focus set not among the sets", npcs: focused, sets: sets, opener: opener, counter: counter, rng: rng},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.NewCrowd(tt.npcs, tt.sets, tt.opener, tt.counter, tt.rng); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}
