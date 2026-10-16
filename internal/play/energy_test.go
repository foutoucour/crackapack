package play_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// spendWelcomeBoosters opens every welcome booster of set, which are free.
func spendWelcomeBoosters(t *testing.T, g *play.Game, code string) {
	t.Helper()
	for range play.WelcomeBoosters {
		if _, err := g.Open(code); err != nil {
			t.Fatalf("Open a welcome booster: %v", err)
		}
	}
}

// startedGame returns a session on the Ravnica test set, with its welcome boosters already opened,
// so that the next opening costs energy, and the clock it runs on.
func startedGame(t *testing.T) (*play.Game, *playtest.Clock, string) {
	t.Helper()
	rav := domaintest.NewRavnicaCatalogue()
	g, clock := newGameWithClock(t, rav)
	spendWelcomeBoosters(t, g, rav.Code)
	return g, clock, rav.Code
}

func TestNewGame_refusesRulesThatCannotWork(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	clock := playtest.NewClock()

	tests := []struct {
		name  string
		rules func() play.Rules
	}{
		{"a set without a cost", func() play.Rules {
			r := playtest.Rules(rav)
			delete(r.Costs, rav.Code)
			return r
		}},
		{"a cost of zero", func() play.Rules {
			r := playtest.Rules(rav)
			r.Costs[rav.Code] = 0
			return r
		}},
		{"a cost that no reserve and savings could ever pay", func() play.Rules {
			r := playtest.Rules(rav)
			r.Costs[rav.Code] = 10 + 200 + 1
			return r
		}},
		{"recharge settings that are not valid", func() play.Rules {
			r := playtest.Rules(rav)
			r.Energy.ReserveCap = 0
			return r
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := play.NewGame([]domain.SetCatalogue{rav, tsp}, nil, newOpener(), tc.rules(), clock.Now); err == nil {
				t.Error("NewGame returned nil error")
			}
		})
	}
}

func TestEnergy_startsFull(t *testing.T) {
	t.Parallel()
	g := newGame(t, domaintest.NewRavnicaCatalogue())

	got := g.Energy()

	want := play.EnergyStatus{Reserve: 10, ReserveCap: 10, Savings: 0, SavingsCap: 200}
	// The savings counter is running, so the next point is three ticks away.
	want.NextIn = 3 * playtest.Tick
	if got != want {
		t.Errorf("Energy() = %+v, want %+v", got, want)
	}
}

func TestOpen_theWelcomeBoostersAreFreeAndSpentFirst(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	spendWelcomeBoosters(t, g, rav.Code)

	if got := g.Energy().Reserve; got != 10 {
		t.Errorf("reserve = %d after the welcome boosters, want 10: they cost no energy", got)
	}
	if got := packCounts(g)[rav.Code]; got != 0 {
		t.Errorf("%d welcome boosters left, want 0", got)
	}
}

func TestOpen_afterTheWelcomeBoostersABoosterCostsTheEnergyOfItsSet(t *testing.T) {
	t.Parallel()
	g, _, code := startedGame(t)

	if _, err := g.Open(code); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if got := g.Energy().Reserve; got != 10-playtest.Cost {
		t.Errorf("reserve = %d, want %d", got, 10-playtest.Cost)
	}
}

func TestOpen_withTooLittleEnergyRefusesAndChangesNothing(t *testing.T) {
	t.Parallel()
	g, _, code := startedGame(t)
	for range 2 {
		if _, err := g.Open(code); err != nil {
			t.Fatalf("Open: %v", err)
		}
	}
	last, _ := g.LastOpening()
	before, err := g.Binder(code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}

	_, err = g.Open(code)

	if !errors.Is(err, play.ErrNotEnoughEnergy) {
		t.Fatalf("Open error = %v, want ErrNotEnoughEnergy", err)
	}
	var short *play.NotEnoughEnergyError
	if !errors.As(err, &short) || short.Need != playtest.Cost || short.Have != 0 || short.Wait != playtest.Cost*playtest.Tick {
		t.Errorf("error details = %+v, want need %d, have 0, wait %v", short, playtest.Cost, playtest.Cost*playtest.Tick)
	}
	after, err := g.Binder(code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}
	if after.Progress.Owned != before.Progress.Owned {
		t.Errorf("a refused opening changed the collection: %d then %d cards", before.Progress.Owned, after.Progress.Owned)
	}
	if now, _ := g.LastOpening(); len(now.Cards) != len(last.Cards) || now.Cards[0].Card.ID != last.Cards[0].Card.ID {
		t.Error("a refused opening replaced the last opening")
	}
	if got := g.Energy().Reserve; got != 0 {
		t.Errorf("reserve = %d, want it left at 0", got)
	}
}

func TestEnergy_rechargesWithTheClockAndKeepsThePartOfATick(t *testing.T) {
	t.Parallel()
	g, clock, code := startedGame(t)
	for range 2 {
		if _, err := g.Open(code); err != nil {
			t.Fatalf("Open: %v", err)
		}
	}

	clock.Advance(3 * playtest.Tick)
	if got := g.Energy().Reserve; got != 3 {
		t.Fatalf("reserve = %d after three ticks, want 3", got)
	}
	clock.Advance(playtest.Tick / 2)
	if got := g.Energy().Reserve; got != 3 {
		t.Errorf("reserve = %d after half a tick more, want it still 3", got)
	}
	clock.Advance(playtest.Tick / 2)
	if got := g.Energy().Reserve; got != 4 {
		t.Errorf("reserve = %d after the other half, want 4: the half tick must not be lost", got)
	}
}

func TestEnergy_aLongAbsenceFillsTheReserveThenTheSavings(t *testing.T) {
	t.Parallel()
	g, clock, code := startedGame(t)
	for range 2 {
		if _, err := g.Open(code); err != nil {
			t.Fatalf("Open: %v", err)
		}
	}

	// Ten ticks fill the reserve, and six more give two savings points, one every three ticks.
	clock.Advance(16 * playtest.Tick)

	got := g.Energy()
	if got.Reserve != 10 || got.Savings != 2 {
		t.Errorf("after 16 ticks energy = %d reserve, %d savings, want 10 and 2", got.Reserve, got.Savings)
	}
}

func TestEnergy_aClockThatGoesBackDoesNothing(t *testing.T) {
	t.Parallel()
	g, clock, code := startedGame(t)
	if _, err := g.Open(code); err != nil {
		t.Fatalf("Open: %v", err)
	}
	before := g.Energy()

	clock.Advance(-5 * playtest.Tick)

	// The recharge waits until the clock has caught up, so NextIn grows by the time it went back,
	// but no energy is gained or lost, and none can be made by moving the clock.
	if got := g.Energy(); got.Reserve != before.Reserve || got.Savings != before.Savings {
		t.Errorf("energy = %d reserve, %d savings after the clock went back, want %d and %d", got.Reserve, got.Savings, before.Reserve, before.Savings)
	}
	clock.Advance(5*playtest.Tick + playtest.Tick)
	if got := g.Energy().Reserve; got != before.Reserve+1 {
		t.Errorf("reserve = %d, want %d: time counts again from the point the clock was at", got, before.Reserve+1)
	}
}

func TestEnergy_spendsTheReserveFirstThenTheSavings(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	// A booster that costs 12 needs the 2 savings points on top of the full reserve of 10.
	rules := playtest.Rules(rav)
	rules.Costs[rav.Code] = 12
	clock := playtest.NewClock()
	g, err := play.NewGame([]domain.SetCatalogue{rav}, nil, newOpener(), rules, clock.Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	spendWelcomeBoosters(t, g, rav.Code)
	clock.Advance(6 * playtest.Tick)
	if got := g.Energy(); got.Reserve != 10 || got.Savings != 2 {
		t.Fatalf("before = %+v, want a full reserve and 2 savings", got)
	}

	if _, err := g.Open(rav.Code); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if got := g.Energy(); got.Reserve != 0 || got.Savings != 0 {
		t.Errorf("after = %d reserve, %d savings, want both spent", got.Reserve, got.Savings)
	}
}

func TestPacks_sayWhatABoosterCostsAndHowLongToWait(t *testing.T) {
	t.Parallel()
	g, clock, code := startedGame(t)
	for range 2 {
		if _, err := g.Open(code); err != nil {
			t.Fatalf("Open: %v", err)
		}
	}
	clock.Advance(2*playtest.Tick + 4*time.Minute)

	pack := g.Packs()[0]

	if pack.Cost != playtest.Cost || pack.Count != 0 {
		t.Errorf("pack = %+v, want a cost of %d and no welcome booster", pack, playtest.Cost)
	}
	// 2 points back, 3 more needed, and 4 minutes of the next tick have gone by.
	if want := 3*playtest.Tick - 4*time.Minute; pack.Wait != want {
		t.Errorf("wait = %v, want %v", pack.Wait, want)
	}
}

func TestPacks_noWaitWhileAWelcomeBoosterOrEnoughEnergyIsThere(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	g := newGame(t, rav)

	if got := g.Packs()[0].Wait; got != 0 {
		t.Errorf("wait = %v with welcome boosters left, want 0", got)
	}
	spendWelcomeBoosters(t, g, rav.Code)
	if got := g.Packs()[0].Wait; got != 0 {
		t.Errorf("wait = %v with a full reserve, want 0", got)
	}
}

func TestEnergy_saysWhenTheNextPointComes(t *testing.T) {
	t.Parallel()
	g, clock, code := startedGame(t)
	if _, err := g.Open(code); err != nil {
		t.Fatalf("Open: %v", err)
	}
	clock.Advance(3 * time.Minute)

	got := g.Energy().NextIn

	if want := playtest.Tick - 3*time.Minute; got != want {
		t.Errorf("NextIn = %v, want %v", got, want)
	}
}

func TestOpen_isSafeAndExactUnderConcurrentRequests(t *testing.T) {
	t.Parallel()
	g, _, code := startedGame(t)

	var wg sync.WaitGroup
	var opened, refused atomic.Int32
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := g.Open(code)
			switch {
			case err == nil:
				opened.Add(1)
			case errors.Is(err, play.ErrNotEnoughEnergy):
				refused.Add(1)
			default:
				t.Errorf("Open: %v", err)
			}
		}()
	}
	wg.Wait()

	// A reserve of 10 pays for exactly two boosters of 5.
	if opened.Load() != 2 || refused.Load() != 18 {
		t.Errorf("%d opened and %d refused, want 2 and 18", opened.Load(), refused.Load())
	}
}
