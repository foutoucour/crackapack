package play_test

import (
	"errors"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const testSeed = 7

func newOpener() *domain.Opener {
	return domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{
			domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
		},
		rand.New(rand.NewPCG(testSeed, testSeed)),
	)
}

func newGame(t *testing.T, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	g, _ := newGameWithClock(t, sets...)
	return g
}

func packCounts(g *play.Game) map[string]int {
	counts := make(map[string]int)
	for _, p := range g.Packs() {
		counts[p.SetCode] = p.Count
	}
	return counts
}

func TestNewGame_grantsTheWelcomeBoostersRoundRobinAcrossSets(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	pc := domaintest.NewPlanarChaosCatalogue()

	tests := []struct {
		name string
		sets []domain.SetCatalogue
		want map[string]int
	}{
		{"one set gets them all", []domain.SetCatalogue{rav}, map[string]int{rav.Code: 10}},
		{"two sets share them equally", []domain.SetCatalogue{rav, tsp}, map[string]int{rav.Code: 5, tsp.Code: 5}},
		{"the first sets take the remainder", []domain.SetCatalogue{rav, tsp, pc}, map[string]int{rav.Code: 4, tsp.Code: 3, pc.Code: 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, err := play.NewGame(tc.sets, nil, newOpener(), playtest.Rules(tc.sets...), playtest.NewClock().Now)
			if err != nil {
				t.Fatalf("NewGame: %v", err)
			}
			got := packCounts(g)
			for code, want := range tc.want {
				if got[code] != want {
					t.Errorf("packs of %s = %d, want %d (all: %v)", code, got[code], want, got)
				}
			}
		})
	}
}

func TestPacks_listsSetsInTheOrderGivenWithTheirNames(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	g := newGame(t, rav, tsp)

	packs := g.Packs()
	if len(packs) != 2 || packs[0].SetCode != rav.Code || packs[1].SetCode != tsp.Code {
		t.Fatalf("Packs() = %+v, want %s then %s", packs, rav.Code, tsp.Code)
	}
	if packs[1].SetName != tsp.Name {
		t.Errorf("SetName = %q, want %q", packs[1].SetName, tsp.Name)
	}
}

func TestNewGame_rejectsNoSets(t *testing.T) {
	t.Parallel()
	if _, err := play.NewGame(nil, nil, newOpener(), playtest.Rules(), playtest.NewClock().Now); err == nil {
		t.Fatal("NewGame with no set returned nil error")
	}
}

func TestOpen_spendsOnePackAndRevealsNamedCardsRareLast(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	opening, err := g.Open(rav.Code)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if got := packCounts(g)[rav.Code]; got != play.WelcomeBoosters-1 {
		t.Errorf("packs left = %d, want %d", got, play.WelcomeBoosters-1)
	}
	if opening.SetCode != rav.Code || opening.SetName != rav.Name {
		t.Errorf("opening set = %q/%q, want %q/%q", opening.SetCode, opening.SetName, rav.Code, rav.Name)
	}
	if len(opening.Cards) == 0 {
		t.Fatal("opening has no cards")
	}
	for _, c := range opening.Cards {
		if c.Card.Name == "" {
			t.Errorf("card %q has no name", c.Card.ID)
		}
	}
	// Every Ravnica booster of the test catalogue holds exactly one rare; the generator puts it last.
	rarity := opening.Cards[len(opening.Cards)-1].Card.Rarity
	if rarity != domain.RarityRare {
		t.Errorf("last card rarity = %q, want %q", rarity, domain.RarityRare)
	}
}

func TestOpen_marksOnlyTheFirstCopyEverSeenAsNew(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	seen := make(map[domain.CardID]bool)
	for range play.WelcomeBoosters {
		opening, err := g.Open(rav.Code)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		for _, c := range opening.Cards {
			wantNew := !seen[c.Card.ID]
			if c.New != wantNew {
				t.Errorf("card %q New = %v, want %v", c.Card.ID, c.New, wantNew)
			}
			seen[c.Card.ID] = true
		}
	}
}

func TestOpen_refusesAnUnknownSet(t *testing.T) {
	t.Parallel()
	g := newGame(t, domaintest.NewRavnicaCatalogue())

	_, err := g.Open("NOPE")
	if !errors.Is(err, play.ErrUnknownSet) {
		t.Fatalf("Open(NOPE) error = %v, want ErrUnknownSet", err)
	}
}

func TestOpen_afterTheWelcomeBoostersTheStockStaysAtZeroAndEnergyPays(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)
	for range play.WelcomeBoosters {
		if _, err := g.Open(rav.Code); err != nil {
			t.Fatalf("Open: %v", err)
		}
	}

	if _, err := g.Open(rav.Code); err != nil {
		t.Fatalf("Open after the welcome boosters: %v", err)
	}

	if got := packCounts(g)[rav.Code]; got != 0 {
		t.Errorf("welcome boosters left = %d, want 0: a booster paid for with energy is not one of them", got)
	}
}

func TestOpen_namesCardsDrawnFromASupplementaryPool(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	g := newPooledGame(t, tsp, tsb)

	sawPoolCard := false
	for range play.WelcomeBoosters {
		opening, err := g.Open(tsp.Code)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		for _, c := range opening.Cards {
			if c.Card.Name == "" {
				t.Errorf("card %q has no name", c.Card.ID)
			}
			if _, inSet := tsp.Cards[c.Card.ID]; !inSet {
				sawPoolCard = true
			}
		}
	}
	if !sawPoolCard {
		t.Error("no card of the supplementary pool was drawn: the test does not cover the lookup")
	}
}

func TestLastOpening_isEmptyUntilAPackIsOpenedThenHoldsTheLatest(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	if _, ok := g.LastOpening(); ok {
		t.Fatal("LastOpening reported an opening before any pack was opened")
	}

	opened, err := g.Open(rav.Code)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	last, ok := g.LastOpening()
	if !ok {
		t.Fatal("LastOpening reported none after an Open")
	}
	if len(last.Cards) != len(opened.Cards) || last.Cards[0].Card.ID != opened.Cards[0].Card.ID {
		t.Errorf("LastOpening = %+v, want the opening just returned", last)
	}
}

func TestOpen_isSafeFromConcurrentRequests(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	var wg sync.WaitGroup
	for range play.WelcomeBoosters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := g.Open(rav.Code); err != nil {
				t.Errorf("Open: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := packCounts(g)[rav.Code]; got != 0 {
		t.Errorf("packs left = %d, want 0", got)
	}
}
