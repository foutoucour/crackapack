package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// costSets are two sets, a booster of the first worth $10 and of the second $20, which a coin rate
// of 100 and a sale at 60% turn into 600 and 1,200 coins at the Counter.
func costSets() ([]domain.SetCatalogue, domain.PriceSnapshot) {
	cheap, dear := playtest.TinySet("A", 1, 1), playtest.TinySet("B", 1, 1)
	return []domain.SetCatalogue{cheap, dear}, domain.PriceSnapshot{"A-00": 1000, "B-00": 2000}
}

func costConfig(t *testing.T, yaml string) config.Config {
	t.Helper()
	cfg, err := config.Load(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestPlayRules_aBoosterWorthMoreThanTheCapPaysTheDifferenceAsAFee(t *testing.T) {
	t.Parallel()
	sets, prices := costSets()
	costs := map[string]int{"A": 5, "B": 5}
	// The cap is 12 energy at 40 coins: 480 coins, so A pays 600-480 and B 1,200-480.
	cfg := costConfig(t, "energy:\n  coins_per_energy: 40\n")

	rules, err := playRules(cfg, sets, costs, prices)

	if err != nil {
		t.Fatalf("playRules: %v", err)
	}
	if rules.Fees["A"] != 120 || rules.Fees["B"] != 720 {
		t.Errorf("fees = %v, want A 120 and B 720", rules.Fees)
	}
	if rules.TopUp != nil {
		t.Errorf("top-up = %+v, want none while coin_topup is off", rules.TopUp)
	}
}

func TestPlayRules_noPriceForEnergyMeansNoFeeAndNoTopUp(t *testing.T) {
	t.Parallel()
	sets, prices := costSets()

	rules, err := playRules(costConfig(t, ""), sets, map[string]int{"A": 5, "B": 5}, prices)

	if err != nil {
		t.Fatalf("playRules: %v", err)
	}
	if len(rules.Fees) != 0 || rules.TopUp != nil {
		t.Errorf("rules = %+v, want no fee and no top-up", rules)
	}
}

func TestPlayRules_theTopUpHasTheConfiguredPriceAndLeastEnergy(t *testing.T) {
	t.Parallel()
	sets, prices := costSets()
	cfg := costConfig(t, "energy:\n  coins_per_energy: 70\n  coin_topup:\n    enabled: true\n    min_energy_units: 2\n")

	rules, err := playRules(cfg, sets, map[string]int{"A": 5, "B": 5}, prices)

	if err != nil {
		t.Fatalf("playRules: %v", err)
	}
	if want := (play.TopUpRules{Price: 70, MinEnergy: 2}); rules.TopUp == nil || *rules.TopUp != want {
		t.Errorf("top-up = %+v, want %+v", rules.TopUp, want)
	}
}

// The game charges one price for the energy it tops up. A price that rises with the day's use needs
// the day's count, which the game does not keep.
func TestPlayRules_refusesAnEscalatingTopUp(t *testing.T) {
	t.Parallel()
	sets, prices := costSets()
	for _, variant := range []string{config.CoinTopupVariantEscalating, config.CoinTopupVariantBoth} {
		cfg := costConfig(t, "energy:\n  coins_per_energy: 70\n  coin_topup:\n    enabled: true\n    variant: "+variant+"\n")

		_, err := playRules(cfg, sets, map[string]int{"A": 5, "B": 5}, prices)

		if err == nil || !strings.Contains(err.Error(), variant) {
			t.Errorf("playRules with the %s variant error = %v, want a refusal that names it", variant, err)
		}
	}
}

// The demo shows the fee on one of its two sets and a top-up on both, and pays the daily reward, so
// that a new demo exercises all three.
func TestPlayConfig_theDemoGameHasAFeeATopUpAndADailyReward(t *testing.T) {
	t.Parallel()
	a := demoListArgs(t)
	clock := playtest.NewClock()
	a.now, a.stdout = clock.Now, io.Discard
	game, _, err := startGame(a, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}

	fees := map[string]domain.Coins{}
	for _, p := range game.Packs() {
		fees[p.SetCode] = p.Fee
	}
	if fees["DEM"] != 0 || fees["DSP"] <= 0 {
		t.Errorf("fees = %v, want none on DEM and one on DSP", fees)
	}
	if _, err := game.OpenWithTopUp("DEM"); err != nil {
		t.Errorf("OpenWithTopUp: %v, want the demo to offer a top-up", err)
	}
	if a.cfg.DailyReward <= 0 {
		t.Fatal("the demo has no daily reward")
	}
	before := game.Balance()
	clock.Advance(24 * time.Hour)
	if got, want := game.Balance(), before+a.cfg.DailyReward; got != want {
		t.Errorf("balance a day later = %d, want %d: the daily reward on top of %d", got, want, before)
	}
}

// The real game prices energy at 100 coins a point and offers the top-up, see tune/play.yaml.
func TestPlayConfig_theRealGamePricesEnergyAndOffersATopUp(t *testing.T) {
	t.Parallel()
	cfg, err := playConfig(false, demo.Dataset{})
	if err != nil {
		t.Fatalf("playConfig: %v", err)
	}

	if cfg.Energy.CoinsPerEnergy != 100 || !cfg.Energy.CoinTopup.Enabled || cfg.Energy.CoinTopup.Variant != config.CoinTopupVariantMinEnergy {
		t.Errorf("energy = %+v, want a price of 100 with the min_energy top-up on", cfg.Energy)
	}
	if cfg.DailyReward <= 0 {
		t.Errorf("daily_reward = %d, want a reward the game pays", cfg.DailyReward)
	}
}
