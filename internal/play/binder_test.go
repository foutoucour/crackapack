package play_test

import (
	"errors"
	"slices"
	"sort"
	"strconv"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func nonBasicCount(cat domain.SetCatalogue) int {
	n := 0
	for _, c := range cat.Cards {
		if !c.IsBasicLand {
			n++
		}
	}
	return n
}

func TestBinders_listEachSetInOrderWithNothingOwnedAtTheStart(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	g := newGame(t, rav, tsp)

	binders := g.Binders()

	if len(binders) != 2 || binders[0].SetCode != rav.Code || binders[1].SetCode != tsp.Code {
		t.Fatalf("Binders() = %+v, want %s then %s", binders, rav.Code, tsp.Code)
	}
	if binders[0].SetName != rav.Name {
		t.Errorf("SetName = %q, want %q", binders[0].SetName, rav.Name)
	}
	if got, want := binders[0].Progress.Total, nonBasicCount(rav); got != want || binders[0].Progress.Owned != 0 {
		t.Errorf("progress = %d/%d, want 0/%d with basic lands left out", binders[0].Progress.Owned, got, want)
	}
}

func TestBinder_hasASlotForEveryNonBasicCardInCollectorNumberOrder(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	binder, err := g.Binder(rav.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	if len(binder.Slots) != nonBasicCount(rav) {
		t.Fatalf("%d slots, want %d: one per non-basic card", len(binder.Slots), nonBasicCount(rav))
	}
	numbers := make([]int, len(binder.Slots))
	for i, s := range binder.Slots {
		n, err := strconv.Atoi(s.Card.Number)
		if err != nil {
			t.Fatalf("card %q has number %q, the test catalogue uses plain numbers", s.Card.ID, s.Card.Number)
		}
		numbers[i] = n
		if s.Regular != 0 || s.Foil != 0 {
			t.Errorf("slot %s holds %d+%d copies before anything was opened", s.Card.ID, s.Regular, s.Foil)
		}
	}
	if !sort.IntsAreSorted(numbers) {
		t.Errorf("slots are not in collector number order: %v", numbers)
	}
}

func TestBinder_numbersSortAsNumbersNotAsText(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	for id, card := range cat.Cards {
		switch id {
		case "rav-c-000":
			card.Number = "10"
		case "rav-c-001":
			card.Number = "9"
		case "rav-c-002":
			card.Number = "100"
		}
		cat.Cards[id] = card
	}
	g := newGame(t, cat)

	binder, err := g.Binder(cat.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	position := make(map[domain.CardID]int)
	for i, s := range binder.Slots {
		position[s.Card.ID] = i
	}
	if position["rav-c-001"] >= position["rav-c-000"] || position["rav-c-000"] >= position["rav-c-002"] {
		t.Errorf("9, 10 and 100 came out as %d, %d, %d: want 9 < 10 < 100", position["rav-c-001"], position["rav-c-000"], position["rav-c-002"])
	}
}

func TestBinder_countsWhatWasOpenedByFinish(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	var openedRegular, openedFoil int
	for range play.WelcomeBoosters {
		opening, err := g.Open(rav.Code)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		for _, c := range opening.Cards {
			if c.Finish == domain.FinishFoil {
				openedFoil++
			} else {
				openedRegular++
			}
		}
	}

	binder, err := g.Binder(rav.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}
	var regular, foil, distinct int
	for _, s := range binder.Slots {
		regular += s.Regular
		foil += s.Foil
		if s.Regular+s.Foil > 0 {
			distinct++
		}
	}
	if regular != openedRegular || foil != openedFoil {
		t.Errorf("binder holds %d regular and %d foil, opened %d and %d", regular, foil, openedRegular, openedFoil)
	}
	if binder.Progress.Owned != distinct || binder.Progress.Owned == 0 {
		t.Errorf("progress owned = %d, want the %d distinct cards in the slots", binder.Progress.Owned, distinct)
	}
}

func TestBinder_aBasicLandShowsOnlyOnceOwnedAndAfterTheSlots(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(1)
	// Every booster draws the whole common sheet, which holds the basic lands, so one is always owned.
	cat.BoosterOptions = []domain.BoosterOption{{Contents: map[string]int{"common": len(cat.Sheets["common"].Cards)}, Weight: 1}}
	g := newGame(t, cat)
	before, err := g.Binder(cat.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}
	for _, s := range before.Slots {
		if s.Card.IsBasicLand {
			t.Fatalf("basic land %s is shown before it is owned", s.Card.ID)
		}
	}
	if _, err := g.Open(cat.Code); err != nil {
		t.Fatalf("Open: %v", err)
	}

	binder, err := g.Binder(cat.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	seenBasic := false
	for _, s := range binder.Slots {
		if s.Card.IsBasicLand {
			seenBasic = true
			if s.Regular+s.Foil == 0 {
				t.Errorf("basic land %s is shown without being owned", s.Card.ID)
			}
		} else if seenBasic {
			t.Errorf("non-basic card %s comes after a basic land", s.Card.ID)
		}
	}
	if !seenBasic {
		t.Error("no basic land in the binder after a booster that always holds them")
	}
	if binder.Progress.Total != nonBasicCount(cat) {
		t.Errorf("progress total = %d, want %d: basic lands never count", binder.Progress.Total, nonBasicCount(cat))
	}
}

// pooledIDs returns the cards of pool that the sheets of cat can draw.
func pooledIDs(cat, pool domain.SetCatalogue) map[domain.CardID]bool {
	ids := make(map[domain.CardID]bool)
	for _, sheet := range cat.Sheets {
		for _, sc := range sheet.Cards {
			if _, inPool := pool.Cards[sc.ID]; inPool {
				ids[sc.ID] = true
			}
		}
	}
	return ids
}

func newTimeSpiralGame(t *testing.T, tsp, tsb domain.SetCatalogue) *play.Game {
	t.Helper()
	return newPooledGame(t, tsp, tsb)
}

func TestBinder_theTimeshiftedCardsAreSlotsOfTheBinderEvenBeforeAnyIsOwned(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	g := newTimeSpiralGame(t, tsp, tsb)
	drawable := pooledIDs(tsp, tsb)

	binder, err := g.Binder(tsp.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	if len(binder.Pools) != 1 || binder.Pools[0].Name != tsb.Name {
		t.Fatalf("pools = %+v, want one section named %q", binder.Pools, tsb.Name)
	}
	if got := len(binder.Pools[0].Slots); got != len(drawable) || got == 0 {
		t.Errorf("%d timeshifted slots, want the %d the boosters can draw", got, len(drawable))
	}
	for _, s := range binder.Pools[0].Slots {
		if s.Regular+s.Foil != 0 {
			t.Errorf("slot %s holds copies before anything was opened", s.Card.ID)
		}
	}
	if want := nonBasicCount(tsp) + len(drawable); binder.Progress.Total != want {
		t.Errorf("progress total = %d, want %d: the set's cards plus the timeshifted ones", binder.Progress.Total, want)
	}
	if got := binder.Progress.ByRarity[domain.RaritySpecial].Total; got != len(drawable) {
		t.Errorf("special cards in the progress = %d, want %d", got, len(drawable))
	}
	for _, s := range binder.Slots {
		if _, inSet := tsp.Cards[s.Card.ID]; !inSet {
			t.Errorf("main slot %s is not a card of the set: timeshifted cards have a section of their own", s.Card.ID)
		}
	}
}

func TestBinder_aTimeshiftedCardOwnedCountsInTheProgressAndTheSummary(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	g := newTimeSpiralGame(t, tsp, tsb)
	for range play.WelcomeBoosters {
		if _, err := g.Open(tsp.Code); err != nil {
			t.Fatalf("Open: %v", err)
		}
	}

	binder, err := g.Binder(tsp.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	ownedPool := 0
	for _, s := range binder.Pools[0].Slots {
		if s.Regular+s.Foil > 0 {
			ownedPool++
		}
	}
	ownedMain := 0
	for _, s := range binder.Slots {
		if !s.Card.IsBasicLand && s.Regular+s.Foil > 0 {
			ownedMain++
		}
	}
	if ownedPool == 0 {
		t.Fatal("no timeshifted card after ten boosters, each of which holds one")
	}
	if binder.Progress.Owned != ownedMain+ownedPool {
		t.Errorf("progress owned = %d, want %d from the set plus %d timeshifted", binder.Progress.Owned, ownedMain, ownedPool)
	}
	if got := binder.Progress.ByRarity[domain.RaritySpecial].Owned; got != ownedPool {
		t.Errorf("special cards owned in the progress = %d, want %d", got, ownedPool)
	}
	summary := g.Binders()[0].Progress
	if summary.Owned != binder.Progress.Owned || summary.Total != binder.Progress.Total {
		t.Errorf("the list of binders says %d/%d, the binder says %d/%d", summary.Owned, summary.Total, binder.Progress.Owned, binder.Progress.Total)
	}
}

func TestBinder_aPoolCardNoBoosterCanDrawIsNotASlot(t *testing.T) {
	t.Parallel()
	tsp, tsb := domaintest.NewTimeSpiralCatalogues()
	// The real timeshifted set holds one card that is in no booster: nobody can get it, so it
	// must not stand in the way of a complete binder.
	tsb.Cards["tsp-tsb-unreachable"] = domain.Card{ID: "tsp-tsb-unreachable", Name: "Unreachable", Number: "999", Rarity: domain.RaritySpecial}
	g := newTimeSpiralGame(t, tsp, tsb)

	binder, err := g.Binder(tsp.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	for _, s := range binder.Pools[0].Slots {
		if s.Card.ID == "tsp-tsb-unreachable" {
			t.Error("a card no booster draws is a slot of the binder")
		}
	}
	if got, want := binder.Progress.Total, nonBasicCount(tsp)+len(pooledIDs(tsp, tsb)); got != want {
		t.Errorf("progress total = %d, want %d", got, want)
	}
}

func TestBinder_aSetWithNoPoolHasNoSection(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	binder, err := g.Binder(rav.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	if len(binder.Pools) != 0 {
		t.Errorf("pools = %+v, want none for a set whose cards are all its own", binder.Pools)
	}
}

func TestBinder_refusesAnUnknownSet(t *testing.T) {
	t.Parallel()
	g := newGame(t, domaintest.NewRavnicaCatalogue())

	if _, err := g.Binder("NOPE"); !errors.Is(err, play.ErrUnknownSet) {
		t.Fatalf("Binder(NOPE) error = %v, want ErrUnknownSet", err)
	}
}

func TestOverviewOf_countsTheUnlockedSetsAndTheBindersThatAreComplete(t *testing.T) {
	t.Parallel()
	summary := func(owned, total int, locked bool) play.BinderSummary {
		return play.BinderSummary{Progress: domain.Progress{Owned: owned, Total: total}, Locked: locked}
	}
	tests := []struct {
		name    string
		binders []play.BinderSummary
		want    play.Overview
	}{
		{name: "no binders", want: play.Overview{}},
		{
			name:    "cards of the unlocked sets only, and the sets unlocked out of all",
			binders: []play.BinderSummary{summary(3, 10, false), summary(10, 10, false), summary(7, 20, true)},
			want:    play.Overview{Owned: 13, Total: 20, BindersComplete: 1, SetsUnlocked: 2, SetsTotal: 3},
		},
		{
			name:    "a binder one card short is not complete",
			binders: []play.BinderSummary{summary(9, 10, false)},
			want:    play.Overview{Owned: 9, Total: 10, SetsUnlocked: 1, SetsTotal: 1},
		},
		{
			name:    "a binder with nothing to collect is not complete",
			binders: []play.BinderSummary{summary(0, 0, false)},
			want:    play.Overview{SetsUnlocked: 1, SetsTotal: 1},
		},
		{
			name:    "a locked binder is not counted complete",
			binders: []play.BinderSummary{summary(5, 5, true)},
			want:    play.Overview{SetsTotal: 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := play.OverviewOf(tc.binders); got != tc.want {
				t.Errorf("OverviewOf = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestBinders_markTheBindersThatPassedTheUnlockThresholdAndAReloadKeepsTheMark(t *testing.T) {
	t.Parallel()
	// A gives its whole binder in one booster, F does not.
	rules := play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 2}
	store := newFileStore(t)
	g := stagedSaved(t, store, rules, 3, oneShot()...)
	passed := func(g *play.Game) []string {
		var out []string
		for _, b := range g.Binders() {
			if b.PastThreshold {
				out = append(out, b.SetCode)
			}
		}
		return out
	}
	if got := passed(g); len(got) != 0 {
		t.Fatalf("binders marked before any booster = %v, want none", got)
	}

	mustOpen(t, g, "A")

	if got := passed(g); !slices.Equal(got, []string{"A"}) {
		t.Errorf("binders marked after A's booster = %v, want [A]", got)
	}
	binder, err := g.Binder("A")
	if err != nil || !binder.PastThreshold {
		t.Errorf("Binder(A).PastThreshold = %v (err %v), want true", binder.PastThreshold, err)
	}
	if got := passed(stagedSaved(t, store, rules, 3, oneShot()...)); !slices.Equal(got, []string{"A"}) {
		t.Errorf("binders marked after a reload = %v, want [A]", got)
	}
}

func TestBinders_aGameWithoutStagesHasNoUnlockThresholdToPass(t *testing.T) {
	t.Parallel()
	g := newGame(t, playtest.TinySet("A", 1, 1))
	mustOpen(t, g, "A")

	if b := g.Binders()[0]; b.Progress.Owned != 1 || b.PastThreshold {
		t.Errorf("binder = %+v, want it complete and not marked: there is no threshold", b)
	}
}
