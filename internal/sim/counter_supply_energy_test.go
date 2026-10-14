package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// energySupplyConfig is smallConfig with an empty Counter that opens boosters from its own energy.
func energySupplyConfig(rechargePerTick int) config.Config {
	cfg := smallConfig()
	cfg.InitialStock = map[domain.Rarity]int{}
	cfg.CounterSupply = config.CounterSupplyConfig{
		Mode: config.CounterSupplyEnergy,
		Energy: config.CounterEnergyConfig{
			RechargePerTick: rechargePerTick, ReserveCap: 100, SavingsIntervalTicks: 1, SavingsCap: 1000,
		},
	}
	return cfg
}

func TestRun_CounterEnergySupplyOpensWhatItsEnergyPays(t *testing.T) {
	t.Parallel()

	rich := runResult(t, energySupplyConfig(20), sim.NewIdlePolicy())
	poor := runResult(t, energySupplyConfig(2), sim.NewIdlePolicy())

	richBoosters, poorBoosters := rich.Summary["counter_boosters_opened"], poor.Summary["counter_boosters_opened"]
	if poorBoosters == 0 {
		t.Fatal("the Counter with little energy opened nothing, so the test proves nothing")
	}
	// Ten times the energy opens about ten times the boosters.
	if ratio := richBoosters / poorBoosters; ratio < 7 || ratio > 13 {
		t.Errorf("boosters opened: %v with 20 energy a tick, %v with 2, ratio %.1f, want about 10", richBoosters, poorBoosters, ratio)
	}
	for _, d := range rich.Days {
		if d.Counter == nil || d.Counter.BoostersOpened == 0 {
			t.Errorf("day %d: Counter metrics = %+v, want boosters opened", d.Day, d.Counter)
		}
	}
}

func TestRun_CounterEnergySupplyDoesNotMoveThePlayersDraws(t *testing.T) {
	t.Parallel()

	without := runResult(t, energySupplyConfig(1), openOnlyPolicy{})
	with := runResult(t, energySupplyConfig(30), openOnlyPolicy{})

	if without.Days[len(without.Days)-1].BoostersOpened == 0 {
		t.Fatal("the player opened nothing, the test cannot show anything")
	}
	for i := range without.Days {
		if without.Days[i].CollectionUSD != with.Days[i].CollectionUSD {
			t.Errorf("day %d: player collection %d against %d: the Counter's energy moved the player's draws",
				without.Days[i].Day, without.Days[i].CollectionUSD, with.Days[i].CollectionUSD)
		}
	}
}
