package play_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// ladder is five sets that rank in their order, A the quickest binder: A gives its one card in one
// booster, and each next set has one card more to collect one at a time.
func ladder() []domain.SetCatalogue {
	return []domain.SetCatalogue{playtest.TinySet("A", 1, 1), playtest.TinySet("B", 2, 1), playtest.TinySet("C", 3, 1), playtest.TinySet("D", 4, 1), playtest.TinySet("E", 5, 1)}
}

// stagedGame starts a game on sets, given in the reverse of their rank so that the order NewGame
// gets does not decide the stages, cut by rules, whose unlocks draw from seed.
func stagedGame(t *testing.T, rules play.StageRules, seed uint64, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	reversed := slices.Clone(sets)
	slices.Reverse(reversed)
	g := newGame(t, reversed...)
	if err := g.Stage(rules, play.NewStream(seed)); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	return g
}

// locked lists the locked sets of g, in the order Packs gives them.
func locked(g *play.Game) []string {
	var out []string
	for _, p := range g.Packs() {
		if p.Locked {
			out = append(out, p.SetCode)
		}
	}
	return out
}

func unlockedCodes(o play.Opening) []string {
	out := make([]string, len(o.Unlocked))
	for i, s := range o.Unlocked {
		out[i] = s.Code
	}
	return out
}

func TestStage_ranksTheSetsFromTheQuickestBinderAndCutsTheStages(t *testing.T) {
	t.Parallel()
	g := stagedGame(t, play.StageRules{Threshold: 0.9, SetsPerUnlock: 1, Stages: 2, StartSets: 1}, 1, ladder()...)

	var got []string
	for _, p := range g.Packs() {
		got = append(got, fmt.Sprintf("%s:%d", p.SetCode, p.Stage))
	}
	// The four sets after the first stage make two stages of two.
	want := []string{"A:1", "B:2", "C:2", "D:3", "E:3"}
	if !slices.Equal(got, want) {
		t.Errorf("packs by stage = %v, want %v", got, want)
	}
	if got := locked(g); !slices.Equal(got, []string{"B", "C", "D", "E"}) {
		t.Errorf("locked = %v, want every set but A", got)
	}
}

// A dearer booster takes more energy to the threshold, so it ranks after a set that needs more
// boosters of a cheaper one.
func TestStage_ranksByTheEnergyToTheThreshold(t *testing.T) {
	t.Parallel()
	quickButDear, slowButCheap := playtest.TinySet("DEAR", 2, 1), playtest.TinySet("CHEAP", 3, 1)
	rules := playtest.Rules(quickButDear, slowButCheap)
	rules.Costs["DEAR"] = 10 * playtest.Cost
	g, err := play.NewGame([]domain.SetCatalogue{quickButDear, slowButCheap}, nil, newOpener(), rules, playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := g.Stage(play.StageRules{Threshold: 0.9, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, play.NewStream(1)); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if got := locked(g); !slices.Equal(got, []string{"DEAR"}) {
		t.Errorf("locked = %v, want the dear set", got)
	}
}

func TestStage_aLockedSetCannotBeOpenedAndGetsNoWelcomeBooster(t *testing.T) {
	t.Parallel()
	g := stagedGame(t, play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 2}, 1, ladder()...)

	if _, err := g.Open("E"); !errors.Is(err, play.ErrLocked) {
		t.Errorf("Open(E) error = %v, want ErrLocked", err)
	}
	counts := packCounts(g)
	if counts["A"]+counts["B"] != play.WelcomeBoosters || counts["A"] != counts["B"] {
		t.Errorf("welcome boosters = %v, want %d split across A and B", counts, play.WelcomeBoosters)
	}
	if binders := g.Binders(); !binders[4].Locked || binders[0].Locked {
		t.Errorf("binders = %+v, want E locked and A open", binders)
	}
}

func TestOpen_unlocksSetsWhenABinderReachesTheThreshold(t *testing.T) {
	t.Parallel()
	// B and C, after the first stage, are B half collected after one booster.
	sets := []domain.SetCatalogue{playtest.TinySet("B", 2, 1), playtest.TinySet("C", 3, 1), playtest.TinySet("D", 4, 1)}
	seven := playtest.TinySet("SEVEN", 25, 7)
	// SEVEN's booster is the same 7 of its 25 cards, so one booster holds 28% of its binder.
	seven.Sheets["common"] = domain.Sheet{Name: "common", Cards: seven.Sheets["common"].Cards[:7], TotalWeight: 7}
	tests := []struct {
		name      string
		sets      []domain.SetCatalogue
		open      string
		threshold float64
		want      int
	}{
		{name: "at the threshold", sets: sets, open: "B", threshold: 0.5, want: 1},
		{name: "below the threshold", sets: sets, open: "B", threshold: 0.6, want: 0},
		// 0.28 of 25 is not 7 in floating point. SEVEN ranks before the 1000 cards of NEVER.
		{name: "at a threshold that floats round", sets: []domain.SetCatalogue{seven, playtest.TinySet("NEVER", 1000, 1)}, open: "SEVEN", threshold: 0.28, want: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := stagedGame(t, play.StageRules{Threshold: tc.threshold, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, 1, tc.sets...)
			before := len(locked(g))

			opening := mustOpen(t, g, tc.open)

			if len(opening.Unlocked) != tc.want || len(locked(g)) != before-tc.want {
				t.Errorf("unlocked = %v and %d locked left, want %d unlocked of %d locked", opening.Unlocked, len(locked(g)), tc.want, before)
			}
			for _, s := range opening.Unlocked {
				if s.Name != "Set "+s.Code {
					t.Errorf("unlocked set %+v, want its name", s)
				}
			}
		})
	}
}

// oneShot is five sets of which A and F give their whole binder in one booster, and rank first in
// that order; C, D and E follow, as in the ladder.
func oneShot() []domain.SetCatalogue {
	return []domain.SetCatalogue{playtest.TinySet("A", 1, 1), playtest.TinySet("F", 2, 2), playtest.TinySet("C", 3, 1), playtest.TinySet("D", 4, 1), playtest.TinySet("E", 5, 1)}
}

func TestOpen_aBinderUnlocksOnceAndEachBinderUnlocks(t *testing.T) {
	t.Parallel()
	// One set a stage: A, then F, then C, D and E.
	g := stagedGame(t, play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 4, StartSets: 1}, 1, oneShot()...)

	if got := unlockedCodes(mustOpen(t, g, "A")); !slices.Equal(got, []string{"F"}) {
		t.Fatalf("first A unlocked %v, want F, the next stage", got)
	}
	if got := mustOpen(t, g, "A").Unlocked; len(got) != 0 {
		t.Errorf("second A unlocked %v, want nothing: a binder unlocks once", got)
	}
	if got := unlockedCodes(mustOpen(t, g, "F")); !slices.Equal(got, []string{"C"}) {
		t.Errorf("F unlocked %v, want C", got)
	}
}

func TestOpen_anUnlockFillsUpFromTheStagesAfter(t *testing.T) {
	t.Parallel()
	// The stages after A are B and C, then D and E: three sets take both of the next and one after.
	g := stagedGame(t, play.StageRules{Threshold: 1, SetsPerUnlock: 3, Stages: 2, StartSets: 1}, 1, ladder()...)

	got := unlockedCodes(mustOpen(t, g, "A"))

	if len(got) != 3 || !slices.Contains(got, "B") || !slices.Contains(got, "C") {
		t.Fatalf("unlocked = %v, want B, C and one of D and E", got)
	}
	if !slices.Contains(got, "D") && !slices.Contains(got, "E") {
		t.Errorf("unlocked = %v, want one of D and E", got)
	}
	if left := locked(g); len(left) != 1 {
		t.Errorf("locked = %v, want one set left", left)
	}
}

// The draw of an unlock is the seed's: the same seed opens the same sets, and some other seed opens
// others.
func TestOpen_theSetsAnUnlockDrawsFollowTheSeed(t *testing.T) {
	t.Parallel()
	rules := play.StageRules{Threshold: 1, SetsPerUnlock: 2, Stages: 1, StartSets: 1}
	draw := func(seed uint64) []string {
		return unlockedCodes(mustOpen(t, stagedGame(t, rules, seed, ladder()...), "A"))
	}
	first := draw(7)
	if again := draw(7); !slices.Equal(again, first) {
		t.Errorf("seed 7 drew %v, then %v", first, again)
	}
	for seed := range uint64(50) {
		if !slices.Equal(draw(seed), first) {
			return
		}
	}
	t.Errorf("every seed drew %v", first)
}

// stagedSaved is stagedGame wired to store, with the boosters and the unlocks of seed.
func stagedSaved(t *testing.T, store play.Store, rules play.StageRules, seed uint64, sets ...domain.SetCatalogue) *play.Game {
	t.Helper()
	stream := play.NewStream(seed)
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()},
		rand.New(stream),
	)
	g, err := play.NewGame(sets, nil, opener, playtest.Rules(sets...), playtest.NewClock().Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := g.Stage(rules, play.NewStream(seed+1)); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := g.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	return g
}

// A reload keeps the sets unlocked, the binders that unlocked and where the draw stood, so it
// unlocks what a game that never stopped would have.
func TestPersist_aReloadKeepsTheUnlocksAndDoesNotRerollThem(t *testing.T) {
	t.Parallel()
	// A and F are the first stage, and each unlocks one of C, D and E.
	rules := play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 2}
	sets := oneShot()

	continuous := stagedSaved(t, play.NewFileStore(filepath.Join(t.TempDir(), "save.json")), rules, 3, sets...)
	store := newFileStore(t)
	first := stagedSaved(t, store, rules, 3, sets...)
	for _, g := range []*play.Game{continuous, first} {
		mustOpen(t, g, "A")
	}
	reloaded := stagedSaved(t, store, rules, 3, sets...)

	if got, want := locked(reloaded), locked(continuous); !slices.Equal(got, want) {
		t.Fatalf("locked after the reload = %v, want %v", got, want)
	}
	mustOpen(t, continuous, "A")
	if got := mustOpen(t, reloaded, "A").Unlocked; len(got) != 0 {
		t.Errorf("A unlocked %v again after the reload, want nothing", got)
	}
	got, want := unlockedCodes(mustOpen(t, reloaded, "F")), unlockedCodes(mustOpen(t, continuous, "F"))
	if len(want) != 1 || !slices.Equal(got, want) {
		t.Errorf("reloaded game unlocked %v, want %v as the continuous one", got, want)
	}
}

func TestPersist_aReloadKeepsTheUnlockOfTheLastOpening(t *testing.T) {
	t.Parallel()
	rules := play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}
	store := newFileStore(t)
	opened := mustOpen(t, stagedSaved(t, store, rules, 3, ladder()...), "A")

	last, ok := stagedSaved(t, store, rules, 3, ladder()...).LastOpening()

	if !ok || !slices.Equal(last.Unlocked, opened.Unlocked) || len(last.Unlocked) != 1 {
		t.Errorf("last opening unlocked %v, want %v", last.Unlocked, opened.Unlocked)
	}
}

// A save written before the stages existed loads with the first stage open and the rest locked.
func TestPersist_theVersion1SaveLoadsWithTheFirstStageOpen(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/save-v1.json")
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "save.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("copy the fixture: %v", err)
	}
	// RAV's binder is far quicker than the 1-in-a-thousand card of NEVER, so RAV is the first stage.
	never := playtest.TinySet("NEVER", 1000, 1)

	g := stagedSaved(t, play.NewFileStore(path), play.StageRules{Threshold: 0.9, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, 1,
		domaintest.NewRavnicaCatalogue(), never)

	if got := locked(g); !slices.Equal(got, []string{"NEVER"}) {
		t.Errorf("locked = %v, want NEVER only", got)
	}
	if got := packCounts(g)["RAV"]; got != 7 {
		t.Errorf("RAV welcome boosters = %d, want the 7 saved", got)
	}
}

func TestPersist_aSaveWithAnUnreadableUnlockDrawIsRefused(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "save.json")
	if err := os.WriteFile(path, []byte(`{"version": 1, "unlocks": {"rand": {"seed": 1, "state": "AAAA"}}}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	g := newGame(t, ladder()...)
	if err := g.Stage(play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}, play.NewStream(1)); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := g.Persist(play.Saves{Store: play.NewFileStore(path), Stream: play.NewStream(1)}); err == nil {
		t.Error("Persist = nil, want the bad draw refused")
	}
}

// A game with every set open, such as one on a list of sets, keeps the unlocks of the save as they
// were, so that a staged game on the same save finds them again.
func TestPersist_aGameWithoutStagesKeepsTheUnlocksOfTheSave(t *testing.T) {
	t.Parallel()
	rules := play.StageRules{Threshold: 1, SetsPerUnlock: 1, Stages: 1, StartSets: 1}
	store := newFileStore(t)
	mustOpen(t, stagedSaved(t, store, rules, 3, ladder()...), "A")

	flat, _ := openSaved(t, store, playtest.NewClock(), 3, ladder()...)
	if got := locked(flat); len(got) != 0 {
		t.Fatalf("a game without stages has %v locked", got)
	}
	mustOpen(t, flat, "E")

	if got := locked(stagedSaved(t, store, rules, 3, ladder()...)); len(got) != 3 {
		t.Errorf("locked = %v, want the three sets still locked after the unlock of the first game", got)
	}
}
