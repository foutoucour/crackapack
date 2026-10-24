package play_test

import (
	"errors"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// The sets of these tests: S pays a coin fee on top of its energy, T does not. A booster of either
// costs playtest.Cost energy, and the reserve holds ten.
const (
	boosterFee     = domain.Coins(150)
	energyPrice    = domain.Coins(40)
	startingCoins  = domain.Coins(1000)
	minTopUpEnergy = 1
)

func costSets() []domain.SetCatalogue {
	return []domain.SetCatalogue{playtest.TinySet("S", 3, 3), playtest.TinySet("T", 3, 3)}
}

// costRules is playtest.Rules with the fee of S and, unless topUp is false, the coin top-up.
func costRules(topUp bool, sets ...domain.SetCatalogue) play.Rules {
	rules := playtest.Rules(sets...)
	rules.Fees = map[string]domain.Coins{"S": boosterFee}
	if topUp {
		rules.TopUp = &play.TopUpRules{Price: energyPrice, MinEnergy: minTopUpEnergy}
	}
	return rules
}

// costGame starts a game on costSets with a Counter that gives coins at the start, under rules.
func costGame(t *testing.T, store play.Store, clock *playtest.Clock, rules play.Rules, coins play.CounterRules) *play.Game {
	t.Helper()
	sets := costSets()
	stream := play.NewStream(1)
	g, err := play.NewGame(sets, nil, newOpenerOn(stream), rules, clock.Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if err := g.OpenCounter(coins, play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	if err := g.Persist(play.Saves{Store: store, Stream: stream}); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	return g
}

func costCounter() play.CounterRules {
	rules := emptyCounter()
	rules.WelcomeBonus = startingCoins
	return rules
}

// spendWelcome opens the welcome boosters of every set, which are free, so that the next opening
// costs energy and the fee.
func spendWelcome(t *testing.T, g *play.Game) {
	t.Helper()
	for _, pack := range g.Packs() {
		for range pack.Count {
			mustOpen(t, g, pack.SetCode)
		}
	}
	if got := g.Balance(); got != startingCoins {
		t.Fatalf("balance after the welcome boosters = %d, want %d: they are free", got, startingCoins)
	}
}

func reserveOf(g *play.Game) int { return g.Energy().Reserve }

func TestOpen_aWelcomeBoosterPaysNoFee(t *testing.T) {
	t.Parallel()
	g := costGame(t, &memStore{}, playtest.NewClock(), costRules(false, costSets()...), costCounter())

	mustOpen(t, g, "S")

	if got := g.Balance(); got != startingCoins {
		t.Errorf("balance %d, want %d: a welcome booster is free", got, startingCoins)
	}
}

func TestOpen_chargesTheFeeOfTheSetOnTopOfItsEnergyAndSavesIt(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	clock := playtest.NewClock()
	g := costGame(t, store, clock, costRules(false, costSets()...), costCounter())
	spendWelcome(t, g)

	mustOpen(t, g, "T")
	if got := g.Balance(); got != startingCoins {
		t.Errorf("balance after a set with no fee = %d, want %d", got, startingCoins)
	}
	mustOpen(t, g, "S")

	if got, want := g.Balance(), startingCoins-boosterFee; got != want {
		t.Errorf("balance after a booster with a fee = %d, want %d", got, want)
	}
	if got := reserveOf(g); got != 0 {
		t.Errorf("energy after two boosters = %d, want 0: the fee is on top of the energy", got)
	}
	loaded := costGame(t, store, clock, costRules(false, costSets()...), costCounter())
	if got, want := loaded.Balance(), startingCoins-boosterFee; got != want {
		t.Errorf("balance after a reload = %d, want %d", got, want)
	}
}

// A refusal for coins leaves the energy and the collection as they were, as a refusal for energy
// leaves the coins.
func TestOpen_aFeeThatTheCoinsCannotPayRefusesAndChangesNothing(t *testing.T) {
	t.Parallel()
	coins := costCounter()
	coins.WelcomeBonus = boosterFee - 1
	g := costGame(t, &memStore{}, playtest.NewClock(), costRules(false, costSets()...), coins)
	for _, pack := range g.Packs() {
		for range pack.Count {
			mustOpen(t, g, pack.SetCode)
		}
	}

	_, err := g.Open("S")

	var short *play.NotEnoughCoinsError
	if !errors.Is(err, play.ErrNotEnoughCoins) || !errors.As(err, &short) || short.Need != boosterFee || short.Have != boosterFee-1 {
		t.Fatalf("Open error = %v, want ErrNotEnoughCoins needing %d with %d", err, boosterFee, boosterFee-1)
	}
	if got := reserveOf(g); got != 10 {
		t.Errorf("energy after a refusal = %d, want 10", got)
	}
	if got := g.Balance(); got != boosterFee-1 {
		t.Errorf("balance after a refusal = %d, want %d", got, boosterFee-1)
	}
}

// The daily reward that is due pays for the fee: the check reads the coins as they stand now.
func TestOpen_theDailyRewardDueCountsTowardsTheFee(t *testing.T) {
	t.Parallel()
	coins := costCounter()
	coins.WelcomeBonus = 100
	coins.DailyReward = 100
	clock := playtest.NewClock()
	g := costGame(t, &memStore{}, clock, costRules(false, costSets()...), coins)
	for _, pack := range g.Packs() {
		for range pack.Count {
			mustOpen(t, g, pack.SetCode)
		}
	}
	if _, err := g.Open("S"); !errors.Is(err, play.ErrNotEnoughCoins) {
		t.Fatalf("Open before the reward error = %v, want ErrNotEnoughCoins", err)
	}

	clock.Advance(24 * time.Hour)

	mustOpen(t, g, "S")
	if got := g.Balance(); got != 200-boosterFee {
		t.Errorf("balance %d, want %d", got, 200-boosterFee)
	}
}

func TestOpenWithTopUp(t *testing.T) {
	t.Parallel()
	// A drained reserve recharges by one a tick, so the energy a test has is the ticks it waits.
	tests := []struct {
		name      string
		set       string
		recharge  int
		wantCoins domain.Coins
		wantErr   error
	}{
		{name: "enough energy pays no top-up", set: "T", recharge: 5, wantCoins: startingCoins},
		{name: "a shortfall is paid in coins", set: "T", recharge: 3, wantCoins: startingCoins - 2*energyPrice},
		{name: "the fee is paid with the top-up", set: "S", recharge: 3, wantCoins: startingCoins - 2*energyPrice - boosterFee},
		{name: "the least energy still has to be there", set: "T", recharge: 0, wantErr: play.ErrNotEnoughEnergy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			clock := playtest.NewClock()
			g := costGame(t, &memStore{}, clock, costRules(true, costSets()...), costCounter())
			spendWelcome(t, g)
			mustOpen(t, g, "T")
			mustOpen(t, g, "T")
			clock.Advance(time.Duration(tt.recharge) * playtest.Tick)

			_, err := g.OpenWithTopUp(tt.set)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("OpenWithTopUp error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got := g.Balance(); got != startingCoins {
					t.Errorf("balance after a refusal = %d, want %d", got, startingCoins)
				}
				return
			}
			if got := g.Balance(); got != tt.wantCoins {
				t.Errorf("balance = %d, want %d", got, tt.wantCoins)
			}
			if got := reserveOf(g); got != max(0, tt.recharge-playtest.Cost) {
				t.Errorf("energy after the top-up = %d, want what the booster left of %d", got, tt.recharge)
			}
		})
	}
}

func TestOpenWithTopUp_aTopUpTheCoinsCannotPayRefusesAndChangesNothing(t *testing.T) {
	t.Parallel()
	coins := costCounter()
	coins.WelcomeBonus = 2*energyPrice - 1
	clock := playtest.NewClock()
	g := costGame(t, &memStore{}, clock, costRules(true, costSets()...), coins)
	for _, pack := range g.Packs() {
		for range pack.Count {
			mustOpen(t, g, pack.SetCode)
		}
	}
	mustOpen(t, g, "T")
	mustOpen(t, g, "T")
	clock.Advance(3 * playtest.Tick)

	_, err := g.OpenWithTopUp("T")

	if !errors.Is(err, play.ErrNotEnoughCoins) {
		t.Fatalf("OpenWithTopUp error = %v, want ErrNotEnoughCoins", err)
	}
	if got := reserveOf(g); got != 3 {
		t.Errorf("energy after a refusal = %d, want 3", got)
	}
}

func TestOpenWithTopUp_aGameWithoutTopUpRefuses(t *testing.T) {
	t.Parallel()
	g := costGame(t, &memStore{}, playtest.NewClock(), costRules(false, costSets()...), costCounter())

	if _, err := g.OpenWithTopUp("T"); !errors.Is(err, play.ErrNoTopUp) {
		t.Errorf("OpenWithTopUp error = %v, want ErrNoTopUp", err)
	}
}

// Opening with Open never spends coins on energy: the top-up is something the player asks for.
func TestOpen_shortOfEnergyRefusesEvenWhenATopUpIsOffered(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := costGame(t, &memStore{}, clock, costRules(true, costSets()...), costCounter())
	spendWelcome(t, g)
	mustOpen(t, g, "T")
	mustOpen(t, g, "T")
	clock.Advance(3 * playtest.Tick)

	if _, err := g.Open("T"); !errors.Is(err, play.ErrNotEnoughEnergy) {
		t.Errorf("Open error = %v, want ErrNotEnoughEnergy", err)
	}
	if got := g.Balance(); got != startingCoins {
		t.Errorf("balance %d, want %d", got, startingCoins)
	}
}

func TestPacks_sayWhatABoosterCostsInCoinsAndWhatATopUpWould(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := costGame(t, &memStore{}, clock, costRules(true, costSets()...), costCounter())
	spendWelcome(t, g)
	packs := func() map[string]play.Pack {
		out := map[string]play.Pack{}
		for _, p := range g.Packs() {
			out[p.SetCode] = p
		}
		return out
	}

	got := packs()
	if got["S"].Fee != boosterFee || got["T"].Fee != 0 {
		t.Errorf("fees S = %d and T = %d, want %d and 0", got["S"].Fee, got["T"].Fee, boosterFee)
	}
	if got["S"].TopUp != nil {
		t.Errorf("top-up offered with a full reserve: %+v", got["S"].TopUp)
	}

	mustOpen(t, g, "T")
	mustOpen(t, g, "T")
	if got := packs(); got["S"].TopUp != nil {
		t.Errorf("top-up offered with no energy: %+v", got["S"].TopUp)
	}
	clock.Advance(3 * playtest.Tick)
	// S asks its fee on top of the top-up, and T does not.
	wants := map[string]play.TopUp{
		"T": {Units: 2, Coins: 2 * energyPrice, Total: 2 * energyPrice},
		"S": {Units: 2, Coins: 2 * energyPrice, Total: 2*energyPrice + boosterFee},
	}
	got = packs()
	for code, want := range wants {
		if got[code].TopUp == nil || *got[code].TopUp != want {
			t.Errorf("top-up of %s = %+v, want %+v", code, got[code].TopUp, want)
		}
	}
}

func TestNewGame_refusesATopUpWithNoPriceOrNoLeastEnergy(t *testing.T) {
	t.Parallel()
	for _, topUp := range []play.TopUpRules{{Price: 0, MinEnergy: 1}, {Price: 40, MinEnergy: 0}} {
		rules := costRules(false, costSets()...)
		rules.TopUp = &topUp
		if _, err := play.NewGame(costSets(), nil, newOpenerOn(play.NewStream(1)), rules, playtest.NewClock().Now); err == nil {
			t.Errorf("NewGame with top-up %+v went through", topUp)
		}
	}
}
