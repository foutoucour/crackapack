package play_test

import (
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// openSaved starts a game on sets, wired to store, whose boosters come from a stream of seed, and
// loads what the store holds. The clock is the test's, so that a later game can be started after a
// stretch of time.
func openSaved(t *testing.T, store play.Store, clock *playtest.Clock, seed uint64, sets ...domain.SetCatalogue) (*play.Game, *play.Stream) {
	t.Helper()
	g, stream, err := tryOpenSaved(store, clock, seed, nil, sets...)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	return g, stream
}

func tryOpenSaved(store play.Store, clock *playtest.Clock, seed uint64, failed func(error), sets ...domain.SetCatalogue) (*play.Game, *play.Stream, error) {
	stream := play.NewStream(seed)
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()},
		rand.New(stream),
	)
	g, err := play.NewGame(sets, nil, opener, playtest.Rules(sets...), clock.Now)
	if err != nil {
		return nil, nil, err
	}
	return g, stream, g.Persist(play.Saves{Store: store, Stream: stream, Failed: failed})
}

func newFileStore(t *testing.T) *play.FileStore {
	t.Helper()
	return play.NewFileStore(filepath.Join(t.TempDir(), "save.json"))
}

func cardIDs(o play.Opening) []domain.CardID {
	ids := make([]domain.CardID, len(o.Cards))
	for i, c := range o.Cards {
		ids[i] = c.Card.ID
	}
	return ids
}

func mustOpen(t *testing.T, g *play.Game, set string) play.Opening {
	t.Helper()
	opening, err := g.Open(set)
	if err != nil {
		t.Fatalf("Open(%q): %v", set, err)
	}
	return opening
}

func TestPersist_aNewGameIsSavedAtOnce(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	store := newFileStore(t)

	openSaved(t, store, playtest.NewClock(), 42, rav)

	snap, found, err := store.Load()
	if err != nil || !found {
		t.Fatalf("Load = found %v, err %v, want the new game saved", found, err)
	}
	if snap.Opener == nil || snap.Opener.Seed != 42 || snap.Welcome["RAV"] != play.WelcomeBoosters {
		t.Errorf("saved snapshot = %+v, want seed 42 and %d welcome boosters", snap, play.WelcomeBoosters)
	}
}

func TestPersist_aGameSavesAfterAnOpeningAndLoadsBackAsItWas(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	pc := domaintest.NewPlanarChaosCatalogue()
	store := newFileStore(t)
	clock := playtest.NewClock()
	first, _ := openSaved(t, store, clock, 42, rav, pc)
	opened := mustOpen(t, first, rav.Code)
	mustOpen(t, first, pc.Code)

	loaded, _ := openSaved(t, store, clock, 1, rav, pc)

	if got, want := packCounts(loaded), packCounts(first); !reflect.DeepEqual(got, want) {
		t.Errorf("packs after the load = %v, want %v", got, want)
	}
	if got, want := loaded.Binders(), first.Binders(); !reflect.DeepEqual(got, want) {
		t.Errorf("binders after the load = %+v, want %+v", got, want)
	}
	if got, want := loaded.Energy(), first.Energy(); got != want {
		t.Errorf("energy after the load = %+v, want %+v", got, want)
	}
	last, ok := loaded.LastOpening()
	want, _ := first.LastOpening()
	if !ok || !reflect.DeepEqual(last, want) {
		t.Errorf("last opening after the load = %+v (found %v), want the second opening %+v", last, ok, want)
	}
	if reflect.DeepEqual(last, opened) {
		t.Fatal("test setup: both openings are alike, the last opening proves nothing")
	}
}

func TestPersist_aLoadedGameKeepsTheCardsTurnedOfTheLastOpening(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	store := newFileStore(t)
	clock := playtest.NewClock()
	first, _ := openSaved(t, store, clock, 42, rav)
	mustOpen(t, first, rav.Code)
	if err := first.Reveal(3); err != nil {
		t.Fatalf("Reveal: %v", err)
	}

	loaded, _ := openSaved(t, store, clock, 1, rav)

	if last, _ := loaded.LastOpening(); last.Revealed != 3 {
		t.Errorf("Revealed after the load = %d, want 3", last.Revealed)
	}
}

func TestPersist_aLoadedGameOpensTheBoosterAContinuousGameWould(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	clock := playtest.NewClock()
	continuous, _ := openSaved(t, newFileStore(t), clock, 42, rav)
	firstOfAll := mustOpen(t, continuous, rav.Code)
	wantNext := mustOpen(t, continuous, rav.Code)

	store := newFileStore(t)
	before, _ := openSaved(t, store, clock, 42, rav)
	mustOpen(t, before, rav.Code)
	// The player closes the game and starts it again. Even the seed they pass is another one: the
	// saved seed wins.
	after, stream := openSaved(t, store, clock, 7, rav)
	got := mustOpen(t, after, rav.Code)

	if !reflect.DeepEqual(cardIDs(got), cardIDs(wantNext)) {
		t.Errorf("booster after the reload = %v, want the continuous game's second booster %v", cardIDs(got), cardIDs(wantNext))
	}
	if reflect.DeepEqual(cardIDs(got), cardIDs(firstOfAll)) {
		t.Errorf("booster after the reload = %v, the same as a fresh game's first", cardIDs(got))
	}
	if stream.Seed() != 42 {
		t.Errorf("seed after the load = %d, want the saved 42", stream.Seed())
	}
}

func TestPersist_theEnergyRechargesForTheTimeTheGameWasClosed(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	store := newFileStore(t)
	clock := playtest.NewClock()
	g, _ := openSaved(t, store, clock, 42, rav)
	for range play.WelcomeBoosters + 2 { // the welcome boosters are free, the two after them empty the reserve
		mustOpen(t, g, rav.Code)
	}
	if got := g.Energy().Reserve; got != 0 {
		t.Fatalf("test setup: reserve = %d, want 0", got)
	}

	clock.Advance(6*playtest.Tick + playtest.Tick/2) // closed for six and a half ticks
	loaded, _ := openSaved(t, store, clock, 42, rav)

	got := loaded.Energy()
	if got.Reserve != 6 {
		t.Errorf("reserve after the load = %d, want 6", got.Reserve)
	}
	if want := playtest.Tick / 2; got.NextIn != want {
		t.Errorf("next point in %v, want %v: the half tick that had gone by must not be lost", got.NextIn, want)
	}
}

func TestPersist_aSaveFromTheFutureRechargesFromNow(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	store := newFileStore(t)
	clock := playtest.NewClock()
	// The game is played and saved while the clock is two hours ahead, then the clock is put back.
	clock.Advance(2 * time.Hour)
	g, _ := openSaved(t, store, clock, 42, rav)
	for range play.WelcomeBoosters + 2 {
		mustOpen(t, g, rav.Code)
	}
	clock.Advance(-2 * time.Hour)

	loaded, _ := openSaved(t, store, clock, 42, rav)
	clock.Advance(3 * playtest.Tick)

	if got := loaded.Energy().Reserve; got != 3 {
		t.Errorf("reserve three ticks after the load = %d, want 3: the energy must not wait for the clock to pass the saved time", got)
	}
}

func TestPersist_aWelcomeBoosterOpenedStaysOpened(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	store := newFileStore(t)
	clock := playtest.NewClock()
	g, _ := openSaved(t, store, clock, 42, rav)
	for range play.WelcomeBoosters {
		mustOpen(t, g, rav.Code)
	}

	loaded, _ := openSaved(t, store, clock, 42, rav)

	if got := packCounts(loaded)[rav.Code]; got != 0 {
		t.Errorf("welcome boosters after the load = %d, want 0: spending them all must not grant them again", got)
	}
}

func TestPersist_aSaveWithoutASectionLoadsItAsANewGameHasIt(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	path := filepath.Join(t.TempDir(), "save.json")
	if err := os.WriteFile(path, []byte(`{"version": 1}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	g, _ := openSaved(t, play.NewFileStore(path), playtest.NewClock(), 42, rav)

	if got := packCounts(g)[rav.Code]; got != play.WelcomeBoosters {
		t.Errorf("welcome boosters = %d, want %d", got, play.WelcomeBoosters)
	}
	if got := g.Energy(); got.Reserve != got.ReserveCap {
		t.Errorf("energy = %+v, want a full reserve", got)
	}
	if _, ok := g.LastOpening(); ok {
		t.Error("a save with no opening loaded one")
	}
}

// TestPersist_loadsTheVersion1Save pins the layout of the first save. A later game must still read
// this file, with the fields it adds taking a new game's values.
func TestPersist_loadsTheVersion1Save(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	data, err := os.ReadFile("testdata/save-v1.json")
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "save.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("copy the fixture: %v", err)
	}

	g, stream := openSaved(t, play.NewFileStore(path), playtest.NewClock(), 1, rav)

	if got := packCounts(g)[rav.Code]; got != 7 {
		t.Errorf("welcome boosters = %d, want 7", got)
	}
	if got := g.Binders()[0].Progress; got.Owned != 2 || got.Playsets != 0 {
		t.Errorf("progress = %+v, want 2 cards owned", got)
	}
	if got := g.Energy().Reserve; got != 5 {
		t.Errorf("reserve = %d, want 5", got)
	}
	if last, ok := g.LastOpening(); !ok || len(last.Cards) != 2 || !last.Cards[1].New || last.Cards[1].Finish != domain.FinishFoil {
		t.Errorf("last opening = %+v (found %v), want the two saved cards", last, ok)
	}
	if stream.Seed() != 42 {
		t.Errorf("seed = %d, want 42", stream.Seed())
	}
}

func TestPersist_aSaveThatDoesNotFitTheGameIsAnErrorAndIsLeftAlone(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tests := []struct {
		name    string
		content string
	}{
		{"negative quantity", `{"version": 1, "collection": [{"id": "rav-c-000", "finish": "foil", "quantity": -1}]}`},
		{"unknown finish", `{"version": 1, "collection": [{"id": "rav-c-000", "finish": "shiny", "quantity": 1}]}`},
		{"negative welcome boosters", `{"version": 1, "welcome": {"RAV": -1}}`},
		{"energy above the cap", `{"version": 1, "energy": {"reserve": 11, "synced_at": "2026-10-16T12:00:00Z"}}`},
		{"energy without a time", `{"version": 1, "energy": {"reserve": 5}}`},
		{"garbage random state", `{"version": 1, "opener": {"seed": 1, "state": "AAAA"}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "save.json")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			_, _, err := tryOpenSaved(play.NewFileStore(path), playtest.NewClock(), 42, nil, rav)

			if err == nil {
				t.Fatal("Persist returned nil error")
			}
			if got, _ := os.ReadFile(path); string(got) != tc.content {
				t.Errorf("the save now holds %q, want it left as it was", got)
			}
		})
	}
}

func TestPersist_aFailedSaveIsReportedAndDoesNotStopTheGame(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	store := &playtest.FlakyStore{}
	diskFull := errors.New("disk full")
	store.Fail(diskFull)
	var told []error
	g, _, err := tryOpenSaved(store, playtest.NewClock(), 42, func(err error) { told = append(told, err) }, rav)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}

	opening, err := g.Open(rav.Code)

	if err != nil || len(opening.Cards) == 0 {
		t.Fatalf("Open = %d cards, err %v, want the opening to go through", len(opening.Cards), err)
	}
	if !errors.Is(g.SaveError(), diskFull) {
		t.Errorf("SaveError() = %v, want %v", g.SaveError(), diskFull)
	}
	// One for the new game and one for the opening.
	if len(told) != 2 || !errors.Is(told[1], diskFull) {
		t.Errorf("failures told = %v, want the two failed saves", told)
	}

	store.Fail(nil)
	mustOpen(t, g, rav.Code)

	if g.SaveError() != nil {
		t.Errorf("SaveError() = %v after a save went through, want nil", g.SaveError())
	}
	if store.Saved() != 1 {
		t.Errorf("saves that went through = %d, want 1", store.Saved())
	}
}

func TestPersist_aRefusedOpeningIsNotSaved(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	store := &playtest.FlakyStore{}
	g, _, err := tryOpenSaved(store, playtest.NewClock(), 42, nil, rav)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	before := store.Saved()

	if _, err := g.Open("NOPE"); err == nil {
		t.Fatal("Open of an unknown set returned nil error")
	}

	if got := store.Saved(); got != before {
		t.Errorf("saves = %d, want %d: nothing changed", got, before)
	}
}
