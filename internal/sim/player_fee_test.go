package sim_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// feeConfig is smallConfig with a price of p coins per energy point. At 1 coin the cap's 12 energy
// is worth 12 coins, so the synthetic sets, worth far more, pay a fee.
func feeConfig(p domain.Coins) config.Config {
	cfg := smallConfig()
	cfg.Energy.CoinsPerEnergy = p
	return cfg
}

func TestRun_PlayerFeeLeavesTheEconomyIntoTheFeeAccount(t *testing.T) {
	t.Parallel()
	cfg := feeConfig(1)
	cfg.WelcomeBonus = 1_000_000 // the synthetic boosters carry fees of several thousand coins

	result := runResult(t, cfg, sim.NewCollectorPolicy())

	fees := result.Summary["booster_fee_coins"]
	if fees <= 0 {
		t.Fatalf("booster_fee_coins = %v, want fees paid, otherwise the test proves nothing", fees)
	}
	last := result.Days[len(result.Days)-1]
	if got := last.MoneySupply[domain.AccountBoosterFee]; float64(got) != fees {
		t.Errorf("booster_fee balance = %d, want the %v the summary reports", got, fees)
	}
	for _, d := range result.Days {
		if d.BoosterFee == nil {
			t.Fatalf("day %d has no booster fee metrics, want them on every day of a run with a fee price", d.Day)
		}
	}
}

func TestRun_WithoutAPriceThereIsNoFeeMetric(t *testing.T) {
	t.Parallel()

	result := runResult(t, smallConfig(), sim.NewCollectorPolicy())

	if _, ok := result.Summary["booster_fee_coins"]; ok {
		t.Error("booster_fee_coins is present, want it absent without a price so other runs keep their output")
	}
	if result.Days[0].BoosterFee != nil {
		t.Error("day 0 has booster fee metrics, want none without a price")
	}
}

// openFeeSetPolicy tries once to open a booster of a set that has a fee, recording what the
// attempt did to the player's energy and to the run.
type openFeeSetPolicy struct {
	err                       *error
	energyBefore, energyAfter *int
	done                      *bool
}

func (p openFeeSetPolicy) RunSession(s sim.Session) {
	if *p.done {
		return
	}
	v := s.View()
	for code, fee := range v.BoosterFees {
		if fee <= 0 {
			continue
		}
		*p.energyBefore = v.EnergyTotal
		*p.err = s.OpenBooster(code)
		*p.energyAfter = s.View().EnergyTotal
		*p.done = true
		return
	}
}

// A player who cannot pay the fee gets an error and keeps its energy: neither half of the price
// is taken alone.
func TestOpenBooster_AnUnpayableFeeRefusesAndKeepsTheEnergy(t *testing.T) {
	t.Parallel()
	cfg := feeConfig(1)
	cfg.WelcomeBonus = 0
	cfg.DailyReward = 0
	var (
		err                       error
		energyBefore, energyAfter int
		done                      bool
	)

	result := runResult(t, cfg, openFeeSetPolicy{err: &err, energyBefore: &energyBefore, energyAfter: &energyAfter, done: &done})

	if !done {
		t.Fatal("no set had a fee, so the test proves nothing")
	}
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("OpenBooster error = %v, want one wrapping ErrInsufficientBalance", err)
	}
	if energyAfter != energyBefore {
		t.Errorf("energy = %d after the refusal, want the %d it had before", energyAfter, energyBefore)
	}
	if got := result.Summary["booster_fee_coins"]; got != 0 {
		t.Errorf("booster_fee_coins = %v, want 0: nothing was paid", got)
	}
}

// The collector skips a set whose fee it cannot pay and opens the next one, instead of waiting
// forever on a set it cannot afford.
func TestCollector_SkipsASetWhoseFeeItCannotPay(t *testing.T) {
	t.Parallel()
	sp, cats := twoSetProgress() // SET-B has the lower progress and would be the target
	costs := map[string]int{"SET-A": 5, "SET-B": 5}
	v := makeView(500, 100, 5, sp, costs, cats, nil, nil)
	v.BoosterFees = map[string]domain.Coins{"SET-B": 501}
	fs := &fakeSession{views: []sim.WorldView{v}, openErrs: []error{nil, errors.New("stop")}}

	sim.NewCollectorPolicy().RunSession(fs)

	if len(fs.openCalls) == 0 || fs.openCalls[0] != "SET-A" {
		t.Errorf("opens = %v, want SET-A first: SET-B costs 501 coins and the player has 500", fs.openCalls)
	}
}

func TestOutputs_BoosterFeeAddsItsColumnOnlyWithAPrice(t *testing.T) {
	t.Parallel()
	with := runResult(t, feeConfig(1), sim.NewCollectorPolicy())
	without := runResult(t, smallConfig(), sim.NewCollectorPolicy())

	var withJSON, withCSV, withoutJSON, withoutCSV bytes.Buffer
	for _, w := range []struct {
		buf    *bytes.Buffer
		result sim.Result
		write  func(io.Writer, sim.Result) error
	}{{&withJSON, with, sim.WriteJSON}, {&withCSV, with, sim.WriteCSV}, {&withoutJSON, without, sim.WriteJSON}, {&withoutCSV, without, sim.WriteCSV}} {
		if err := w.write(w.buf, w.result); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if !strings.Contains(withJSON.String(), `"BoosterFee"`) {
		t.Error("result.json lacks the BoosterFee block with a price")
	}
	header, _, _ := strings.Cut(withCSV.String(), "\n")
	if !strings.Contains(header, "booster_fee_coins") {
		t.Errorf("days.csv header lacks booster_fee_coins: %s", header)
	}
	if strings.Contains(withoutJSON.String(), "BoosterFee") || strings.Contains(withoutCSV.String(), "booster_fee") {
		t.Error("output mentions booster fees without a price: runs without one must not change")
	}
}

func TestCompare_BoosterFeeRunsOnlyAgainstBoosterFeeRuns(t *testing.T) {
	t.Parallel()
	a := runResult(t, feeConfig(1), sim.NewCollectorPolicy())
	b := runResult(t, feeConfig(2), sim.NewCollectorPolicy())
	without := runResult(t, smallConfig(), sim.NewCollectorPolicy())

	if _, err := sim.Compare(a, b); err != nil {
		t.Errorf("Compare of two runs with a price: %v", err)
	}
	if _, err := sim.Compare(a, without); err == nil {
		t.Error("Compare accepted a run with a booster fee against a run without")
	}
}

// Coins are never created or destroyed by a posting: with the fee account counted, every day's
// balances still sum to zero.
func TestRun_PlayerFeeKeepsTheLedgerBalanced(t *testing.T) {
	t.Parallel()
	cfg := feeConfig(1)
	cfg.WelcomeBonus = 1_000_000

	result := runResult(t, cfg, sim.NewCollectorPolicy())

	if result.Summary["booster_fee_coins"] <= 0 {
		t.Fatal("no fee was paid, so the test proves nothing")
	}
	for _, d := range result.Days {
		var sum domain.Coins
		for _, balance := range d.MoneySupply {
			sum += balance
		}
		if sum != 0 {
			t.Errorf("day %d: balances sum to %d, want 0", d.Day, sum)
		}
	}
}
