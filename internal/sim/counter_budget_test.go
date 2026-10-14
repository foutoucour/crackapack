package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// budgetConfig is smallConfig with a Counter that pays from a treasury instead of minting.
func budgetConfig(initial, dailyFloat domain.Coins) config.Config {
	cfg := smallConfig()
	cfg.CounterBudget = config.CounterBudgetConfig{Enabled: true, Initial: initial, DailyFloat: dailyFloat}
	return cfg
}

func TestRun_CounterBudgetRefusesSalesWhenTheTreasuryIsEmpty(t *testing.T) {
	t.Parallel()

	result := runResult(t, budgetConfig(100, 0), sim.NewCashOutPolicy())

	if result.Summary["counter_broke_refusals"] == 0 {
		t.Error("no sale was refused, want the Counter to run out of coins")
	}
	for _, d := range result.Days {
		if d.Counter == nil || d.Counter.Treasury < 0 {
			t.Errorf("day %d: Counter metrics = %+v, want a treasury that is never negative", d.Day, d.Counter)
		}
	}
}

func TestRun_CounterBudgetWithAnAmpleTreasuryRefusesNothing(t *testing.T) {
	t.Parallel()

	result := runResult(t, budgetConfig(1_000_000_000, 0), sim.NewCashOutPolicy())

	if got := result.Summary["counter_broke_refusals"]; got != 0 {
		t.Errorf("counter_broke_refusals = %v, want 0", got)
	}
	if result.Summary["counter_treasury"] <= 0 {
		t.Error("counter_treasury is missing or empty, want what is left of the ample treasury")
	}
}

func TestRun_CounterDailyFloatIsTheOnlyNewCoins(t *testing.T) {
	t.Parallel()
	cfg := budgetConfig(0, 500)

	result := runResult(t, cfg, sim.NewCashOutPolicy())

	// A cash-out player buys nothing, so every coin the Counter pays comes from the float.
	last := result.Days[len(result.Days)-1]
	earned := last.MoneySupply[domain.AccountPlayer] - cfg.WelcomeBonus - domain.Coins(cfg.RunDays)*cfg.DailyReward
	if earned <= 0 || earned > domain.Coins(cfg.RunDays)*500 {
		t.Errorf("the player earned %d from the Counter, want between 1 and the %d the float supplied", earned, cfg.RunDays*500)
	}
}

func TestRun_CounterCoinFeeIsPaidFromTheTreasury(t *testing.T) {
	t.Parallel()
	rich := energySupplyConfig(20)
	rich.CounterBudget = config.CounterBudgetConfig{Enabled: true, Initial: 1_000_000_000}
	rich.CounterSupply.CoinsPerEnergy = 1
	poor := rich
	poor.CounterBudget.Initial = 50

	withCoins := runResult(t, rich, sim.NewIdlePolicy())
	withoutCoins := runResult(t, poor, sim.NewIdlePolicy())

	if withCoins.Summary["counter_fee_coins"] <= 0 {
		t.Fatal("no coin fee was paid, so the test proves nothing")
	}
	if withoutCoins.Summary["counter_boosters_opened"] >= withCoins.Summary["counter_boosters_opened"] {
		t.Errorf("boosters opened: %v with a poor treasury, %v with a rich one, want fewer",
			withoutCoins.Summary["counter_boosters_opened"], withCoins.Summary["counter_boosters_opened"])
	}
}

func TestRun_CounterBudgetDoesNotMoveThePlayersDraws(t *testing.T) {
	t.Parallel()

	without := runResult(t, smallConfig(), openOnlyPolicy{})
	with := runResult(t, budgetConfig(100, 10), openOnlyPolicy{})

	if without.Days[len(without.Days)-1].BoostersOpened == 0 {
		t.Fatal("the player opened nothing, the test cannot show anything")
	}
	for i := range without.Days {
		if without.Days[i].CollectionUSD != with.Days[i].CollectionUSD {
			t.Errorf("day %d: player collection %d against %d: the budget moved the player's draws",
				without.Days[i].Day, without.Days[i].CollectionUSD, with.Days[i].CollectionUSD)
		}
	}
}
