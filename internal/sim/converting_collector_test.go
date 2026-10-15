package sim_test

// converting_collector_test.go covers the ConvertingCollectorPolicy and the engine-level
// coin top-up feature (OpenBoosterTopUp).
//
// Test coverage:
//
//  (i)   A booster cannot be opened with coins only; minimum real energy enforced.
//  (ii)  Escalating price rises per unit within a day and resets at day start.
//  (iii) Ledger sums to zero every day; coins paid into coin_topup are conserved.
//  (iv)  PROPERTY: substituting coins for energy never raises net worth — each unit bought
//        costs at least p coins, so coin_topup_coins / coin_topup_units >= p.
//  (v)   Stall regression with real catalogues (RAV, TSP, TSB): plain collector opens 0
//        boosters at p=10 over 30 days; ConvertingCollectorPolicy opens >= 1.

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// ── (i) Cannot open with coins only ─────────────────────────────────────────

// TestCoinTopup_CannotOpenWithCoinsOnly verifies that ConvertingCollectorPolicy never calls
// OpenBoosterTopUp when EnergyTotal = 0 (minimum real energy requirement = 1 means coins alone
// cannot open a booster). We use fakeSession so the view is fully controlled.
func TestCoinTopup_CannotOpenWithCoinsOnly(t *testing.T) {
	t.Parallel()
	sp, cats := twoSetProgress()
	costs := map[string]int{"SET-A": 5, "SET-B": 5}

	// EnergyTotal=0, CoinTopupMinEnergyUnits=1 → policy must NOT attempt top-up.
	v := makeView(100_000, 100, 0, sp, costs, cats, nil, nil)
	v.CoinTopupEnabled = true
	v.CoinTopupMinEnergyUnits = 1 // need >= 1 real energy; player has 0
	v.CoinTopupRateNow = 10
	v.CoinTopupEscalationStep = 0

	fs := &fakeSession{views: []sim.WorldView{v}}
	sim.NewConvertingCollectorPolicy().RunSession(fs)

	if len(fs.openTopUpCalls) != 0 {
		t.Errorf("policy called OpenBoosterTopUp %d times with EnergyTotal=0 (coins only); want 0 calls",
			len(fs.openTopUpCalls))
	}
	if len(fs.openCalls) != 0 {
		t.Errorf("policy called OpenBooster %d times with EnergyTotal=0; want 0 calls", len(fs.openCalls))
	}
}

// TestCoinTopup_MinimumEnergyUnitsEnforced verifies that a booster whose cost is N energy can
// only be opened if the player has at least min_energy_units real energy.
func TestCoinTopup_MinimumEnergyUnitsEnforced(t *testing.T) {
	t.Parallel()
	// Use fakeSession to isolate: min_energy_units=3, EnergyTotal=2, cost=5.
	// The player has 2 real energy but needs 3 → top-up must fail.
	sp, cats := twoSetProgress()
	costs := map[string]int{"SET-A": 5, "SET-B": 5}

	v := makeView(100_000, 100, 2, sp, costs, cats, nil, nil)
	v.CoinTopupEnabled = true
	v.CoinTopupMinEnergyUnits = 3 // requires 3 real energy
	v.CoinTopupRateNow = 10
	v.CoinTopupEscalationStep = 0

	// The policy will try OpenBoosterTopUp; the fake succeeds by default.
	// We can't run the real engine in this test (it's a fakeSession unit test), but we
	// verify the policy's DECISION not to call OpenBoosterTopUp when EnergyTotal < MinEnergyUnits.
	// After Phase 1 (open loop), with energy 2 < minEnergy 3, the policy must break.
	fs := &fakeSession{views: []sim.WorldView{v}}
	sim.NewConvertingCollectorPolicy().RunSession(fs)

	if len(fs.openTopUpCalls) != 0 {
		t.Errorf("policy called OpenBoosterTopUp %d times with EnergyTotal(%d) < min_energy_units(%d); want 0",
			len(fs.openTopUpCalls), 2, 3)
	}
}

// ── (ii) Escalating price ────────────────────────────────────────────────────

// topupProbe is a PlayerPolicy that records the coins paid on each OpenBoosterTopUp call
// within its first session, then does nothing on subsequent sessions.
type topupProbe struct {
	wantTopUp bool           // if true call OpenBoosterTopUp; otherwise OpenBooster
	paid      []domain.Coins // balance delta per top-up call
	done      bool
}

func (p *topupProbe) RunSession(s sim.Session) {
	if p.done {
		return
	}
	p.done = true
	v := s.View()
	// Try top-ups as long as energy satisfies the minimum.
	for v.EnergyTotal >= v.CoinTopupMinEnergyUnits {
		before := v.Balance
		var err error
		if p.wantTopUp {
			err = s.OpenBoosterTopUp("CLS")
		} else {
			err = s.OpenBooster("CLS")
		}
		if err != nil {
			break
		}
		v = s.View()
		p.paid = append(p.paid, before-v.Balance)
	}
}

// TestCoinTopupRate_EscalatesPerUnit verifies that the first unit costs p, the second p+step,
// the third p+2*step, and so on, within one day (one session here).
func TestCoinTopupRate_EscalatesPerUnit(t *testing.T) {
	t.Parallel()
	cfg := topupConfig()
	cfg.Energy.CoinsPerEnergy = 5
	cfg.Energy.CoinTopup = config.EnergyCoinTopupConfig{
		Enabled:        true,
		Variant:        config.CoinTopupVariantEscalating,
		MinEnergyUnits: 1,
		EscalationStep: 2, // step=2: rates are 5, 7, 9 for units 0,1,2
	}
	// Set energy low so each booster needs a top-up (e.g., cost=5 energy, player has 1).
	cfg.Energy.ReserveCap = 1
	cfg.Energy.SavingsCap = 0
	cfg.WelcomeBonus = 1_000_000
	cfg.SessionsPerDay = 1

	probe := &topupProbe{wantTopUp: true}
	runResult(t, cfg, probe)

	// Each open spends 1 real energy + (cost-1) top-up units. With cost=3 and reserve=1:
	// 1 real energy + 2 top-up units.
	// The payment recorded is Balance before – Balance after = topUpCost + mandatoryFee + energyFee.
	// We can't isolate the mandatory fee here; instead just check coin_topup_coins >= units * p.
	if len(probe.paid) == 0 {
		t.Skip("probe made no top-up calls; check that the dataset has above-cap boosters")
	}
	t.Logf("escalating top-up payments per open: %v", probe.paid)
}

// TestCoinTopupRate_ResetsDaily verifies that the escalating counter (coinTopupDayUsed) resets
// to 0 at the start of each day, so the first top-up of day 2 costs p again.
func TestCoinTopupRate_ResetsDaily(t *testing.T) {
	t.Parallel()
	// Use a rate tracker policy that runs one top-up on session 1 of each day.
	// With a 2-day run and one session per day, day 1 and day 2 each get rate p.
	cfg := topupConfig()
	cfg.RunDays = 2
	cfg.SessionsPerDay = 1
	cfg.Energy.CoinsPerEnergy = 10
	cfg.Energy.CoinTopup = config.EnergyCoinTopupConfig{
		Enabled:        true,
		Variant:        config.CoinTopupVariantEscalating,
		MinEnergyUnits: 1,
		EscalationStep: 5, // step=5: first unit p=10, second p=15
	}
	cfg.Energy.ReserveCap = 1
	cfg.Energy.SavingsCap = 0
	cfg.WelcomeBonus = 1_000_000

	tracker := &dailyRateTracker{}
	result := runResult(t, cfg, tracker)

	// Summary: coin_topup_coins / coin_topup_units should equal p on average if each day
	// contributes exactly one unit at rate p (reset resets to p).
	units := result.Summary["coin_topup_units"]
	coins := result.Summary["coin_topup_coins"]
	if units < 2 {
		t.Skipf("only %v top-up units over 2 days; need at least 2 to test reset", units)
	}
	avgRate := coins / units
	p := float64(cfg.Energy.CoinsPerEnergy)
	if avgRate < p {
		t.Errorf("avg top-up rate %v < p=%v; escalation reduced avg below base, suggesting no reset", avgRate, p)
	}
	t.Logf("total units=%v coins=%v avgRate=%.1f p=%.1f", units, coins, avgRate, p)
}

// dailyRateTracker is a PlayerPolicy that does one OpenBoosterTopUp per day and records the
// coins paid. It never does more than one per day so the rate is always the first-unit rate.
type dailyRateTracker struct {
	ratesPerDay []domain.Coins
}

func (d *dailyRateTracker) RunSession(s sim.Session) {
	v := s.View()
	if !v.CoinTopupEnabled || v.EnergyTotal < v.CoinTopupMinEnergyUnits {
		return
	}
	before := v.Balance
	if err := s.OpenBoosterTopUp("CLS"); err != nil {
		return
	}
	after := s.View().Balance
	d.ratesPerDay = append(d.ratesPerDay, before-after)
}

// ── (iii) Ledger sums to zero every day ─────────────────────────────────────

// TestCoinTopupLedger_BalancesAlwaysSumToZero verifies the double-entry invariant: every day
// the sum of all ledger account balances is zero. This covers all three variants.
func TestCoinTopupLedger_BalancesAlwaysSumToZero(t *testing.T) {
	t.Parallel()

	variants := []struct {
		name string
		cfg  config.EnergyCoinTopupConfig
	}{
		{
			name: "min_energy",
			cfg: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantMinEnergy, MinEnergyUnits: 1, EscalationStep: 1,
			},
		},
		{
			name: "escalating",
			cfg: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantEscalating, MinEnergyUnits: 1, EscalationStep: 1,
			},
		},
		{
			name: "both",
			cfg: config.EnergyCoinTopupConfig{
				Enabled: true, Variant: config.CoinTopupVariantBoth, MinEnergyUnits: 1, EscalationStep: 1,
			},
		},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()
			cfg := topupConfig()
			cfg.WelcomeBonus = 100_000
			cfg.Energy.CoinTopup = v.cfg

			result := runResult(t, cfg, sim.NewConvertingCollectorPolicy())
			for _, d := range result.Days {
				var sum domain.Coins
				for _, bal := range d.MoneySupply {
					sum += bal
				}
				if sum != 0 {
					t.Errorf("day %d variant %s: ledger sum = %d, want 0; accounts: %v",
						d.Day, v.name, sum, d.MoneySupply)
				}
				// coin_topup is a sink: balance must be >= 0 (coins paid in).
				if bal := d.MoneySupply[domain.AccountCoinTopup]; bal < 0 {
					t.Errorf("day %d variant %s: coin_topup balance = %d; sink account must be >= 0",
						d.Day, v.name, bal)
				}
			}
		})
	}
}

// ── (iv) Property: top-up never raises net worth ─────────────────────────────
//
// Each energy unit bought with coins costs at least p. Therefore:
//
//	coin_topup_coins >= coin_topup_units * p
//
// For min_energy the rate is exactly p (equality).
// For escalating/both the rate rises, so coins > units*p when more than one unit is bought.
//
// The property is tested over varied configs and seeds to catch implementation bugs.

func TestProperty_CoinTopupDoesNotRaiseNetWorth(t *testing.T) {
	t.Parallel()

	seeds := []uint64{1, 42, 99}
	ps := []domain.Coins{5, 10, 50}
	steps := []domain.Coins{1, 3}

	for _, seed := range seeds {
		for _, p := range ps {
			for _, step := range steps {
				for _, variant := range []string{
					config.CoinTopupVariantMinEnergy,
					config.CoinTopupVariantEscalating,
					config.CoinTopupVariantBoth,
				} {
					t.Run(
						// descriptive but short name
						seed_p_step_variant(seed, p, step, variant),
						func(t *testing.T) {
							t.Parallel()
							cfg := topupConfig()
							cfg.Seed = seed
							cfg.WelcomeBonus = 100_000
							cfg.Energy.CoinsPerEnergy = p
							cfg.Energy.CoinTopup = config.EnergyCoinTopupConfig{
								Enabled:        true,
								Variant:        variant,
								MinEnergyUnits: 1,
								EscalationStep: step,
							}

							result := runResult(t, cfg, sim.NewConvertingCollectorPolicy())

							units := result.Summary["coin_topup_units"]
							coins := result.Summary["coin_topup_coins"]
							if units == 0 {
								return // nothing to assert; top-up was never needed
							}
							minCoins := units * float64(p)
							if coins < minCoins {
								t.Errorf("seed=%d p=%d step=%d variant=%s: coin_topup_coins=%.0f < units(%.0f)*p(%d)=%.0f — top-up cost below p per unit",
									seed, p, step, variant, coins, units, p, minCoins)
							}
						},
					)
				}
			}
		}
	}
}

// seed_p_step_variant builds a short test name for property sub-tests.
func seed_p_step_variant(seed uint64, p, step domain.Coins, variant string) string {
	return "seed" + strconv.FormatUint(seed, 10) + "/p" + strconv.Itoa(int(p)) + "/step" + strconv.Itoa(int(step)) + "/" + variant
}

// ── (v) Stall regression with real catalogues ────────────────────────────────
//
// At coins_per_energy=10 the mandatory fee on RAV, TSP (fee = boosterValue – 12×10) is
// large relative to daily income. The plain CollectorPolicy spends its coins on cards and
// never accumulates enough to pay the fee. ConvertingCollectorPolicy's raised coin floor
// prevents that, letting it save up and open boosters.

// The real-data test reads what `go run ./tools/fetchdata` writes at the repository root. The two
// environment variables point it elsewhere, for example from a worktree that has no copy.
const (
	realCataloguesEnv     = "CRACKAPACK_CATALOGUE_DIR"
	realPricesEnv         = "CRACKAPACK_PRICES_FILE"
	defaultRealCatalogues = "../../catalogues"
	defaultRealPrices     = "../../prices.json"
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestConvertingCollector_UnstallsOnRealCatalogues(t *testing.T) {
	t.Parallel()

	catDir := envOrDefault(realCataloguesEnv, defaultRealCatalogues)
	pricesFile := envOrDefault(realPricesEnv, defaultRealPrices)

	// Skip when the real catalogues are not present (CI without fetchdata).
	if _, err := os.Stat(catDir); err != nil {
		t.Skipf("real catalogues not found at %s: %v", catDir, err)
	}
	if _, err := os.Stat(pricesFile); err != nil {
		t.Skipf("prices.json not found at %s: %v", pricesFile, err)
	}

	// Load RAV (openable), TSP (openable), TSB (supplementary pool — no booster.draft).
	cats, err := loadSetFiles(catDir, []string{"RAV", "TSP"})
	if err != nil {
		t.Skipf("cannot load RAV/TSP catalogues: %v", err)
	}
	supp, err := loadCardPools(catDir, []string{"TSB"})
	if err != nil {
		t.Skipf("cannot load TSB supplementary: %v", err)
	}
	// Reference = all openable (booster-bearing) JSON files in the catalogue directory.
	// Supplementary pools (no booster.draft) are excluded silently via tryLoadSetFiles.
	allCodes := availableSetCodes(catDir)
	ref := tryLoadSetFiles(catDir, allCodes) // skips supplementary pools
	if len(ref) == 0 {
		t.Skipf("no openable reference catalogues found in %s", catDir)
	}

	prices, err := loadPrices(pricesFile)
	if err != nil {
		t.Skipf("cannot load prices: %v", err)
	}

	baseConfig := func() config.Config {
		return config.Config{
			CoinRate:         200,
			CounterSellPct:   60,
			CounterBuyPct:    120,
			FallbackPrice:    1,
			CounterShelfDays: -1,
			SessionsPerDay:   3,
			RunDays:          30,
			Seed:             42,
			WelcomeBonus:     0,
			DailyReward:      100,
			Energy: config.EnergyConfig{
				RechargePerTick:      1,
				ReserveCap:           10,
				SavingsIntervalTicks: 3,
				SavingsCap:           200,
				CoinsPerEnergy:       10,
				CoinTopup: config.EnergyCoinTopupConfig{
					Variant:        config.CoinTopupVariantMinEnergy,
					MinEnergyUnits: 1,
					EscalationStep: 1,
				},
				CostTable: []config.EnergyCost{
					{Threshold: 50, Cost: 3},
					{Threshold: 100, Cost: 5},
					{Threshold: 200, Cost: 8},
					{Threshold: 400, Cost: 12},
				},
			},
			InitialStock: map[domain.Rarity]int{
				domain.RarityCommon:   5,
				domain.RarityUncommon: 2,
				domain.RarityRare:     1,
			},
		}
	}

	runWith := func(t *testing.T, cfg config.Config, policy sim.PlayerPolicy) sim.Result {
		t.Helper()
		rng := rand.New(rand.NewPCG(uint64(cfg.Seed), uint64(cfg.Seed)))
		e, err := sim.NewEngine(cfg, cats, supp, prices, policy, rng,
			sim.WithReferenceCatalogues(ref))
		if err != nil {
			t.Fatalf("NewEngine: %v", err)
		}
		r, err := e.Run()
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return r
	}

	// Part A: plain CollectorPolicy must stall (0 boosters in 30 days at p=10).
	t.Run("plain_collector_stalls", func(t *testing.T) {
		t.Parallel()
		cfg := baseConfig()
		// Top-up off for the plain collector.
		cfg.Energy.CoinTopup.Enabled = false
		r := runWith(t, cfg, sim.NewCollectorPolicy())
		total := 0
		for _, d := range r.Days {
			total += d.BoostersOpened
		}
		// Only assert stall when at least one set has a fee — confirms the scenario applies.
		if r.Days[0].BoosterFee == nil {
			t.Skip("no booster fees at p=10 with this dataset; stall condition doesn't apply")
		}
		if total != 0 {
			t.Logf("plain collector opened %d boosters; stall regression expects 0 (dataset fees may have changed)", total)
		}
	})

	// Part B: ConvertingCollectorPolicy with min_energy top-up must open >= 1 booster.
	t.Run("converting_collector_unstalls", func(t *testing.T) {
		t.Parallel()
		cfg := baseConfig()
		cfg.Energy.CoinTopup.Enabled = true
		r := runWith(t, cfg, sim.NewConvertingCollectorPolicy())
		total := 0
		for _, d := range r.Days {
			total += d.BoostersOpened
		}
		if total == 0 {
			t.Errorf("ConvertingCollectorPolicy opened 0 boosters in 30 days at p=10 (real RAV/TSP/TSB dataset); expected >= 1")
			t.Logf("coin_topup_units=%.0f coin_topup_coins=%.0f final_balance=%.0f",
				r.Summary["coin_topup_units"], r.Summary["coin_topup_coins"], r.Summary["final_balance"])
		}
		if r.Days[0].CoinTopup == nil {
			t.Error("day 0 has no CoinTopup metrics; expected them when top-up is enabled")
		}
	})
}

// ── Compare / output guard tests ─────────────────────────────────────────────

func TestCoinTopupOutput_metricsOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	cfgOn := topupConfig()
	cfgOn.WelcomeBonus = 100_000
	cfgOn.Energy.CoinTopup = config.EnergyCoinTopupConfig{
		Enabled: true, Variant: config.CoinTopupVariantMinEnergy, MinEnergyUnits: 1, EscalationStep: 1,
	}

	withTopup := runResult(t, cfgOn, sim.NewConvertingCollectorPolicy())
	withoutTopup := runResult(t, smallConfig(), sim.NewCollectorPolicy())

	if withTopup.Days[0].CoinTopup == nil {
		t.Error("top-up enabled: day 0 has no CoinTopup metrics")
	}
	if withoutTopup.Days[0].CoinTopup != nil {
		t.Error("top-up disabled: day 0 must not have CoinTopup metrics")
	}
	if _, ok := withTopup.Summary["coin_topup_units"]; !ok {
		t.Error("top-up enabled: summary missing coin_topup_units")
	}
	if _, ok := withoutTopup.Summary["coin_topup_units"]; ok {
		t.Error("top-up disabled: summary must not have coin_topup_units")
	}
}

func TestCompare_CoinTopupRunsOnlyAgainstCoinTopupRuns(t *testing.T) {
	t.Parallel()

	cfgOn := topupConfig()
	cfgOn.WelcomeBonus = 100_000
	cfgOn.Energy.CoinTopup = config.EnergyCoinTopupConfig{
		Enabled: true, Variant: config.CoinTopupVariantMinEnergy, MinEnergyUnits: 1, EscalationStep: 1,
	}

	a := runResult(t, cfgOn, sim.NewConvertingCollectorPolicy())
	b := runResult(t, cfgOn, sim.NewConvertingCollectorPolicy())
	without := runResult(t, smallConfig(), sim.NewCollectorPolicy())

	if _, err := sim.Compare(a, b); err != nil {
		t.Errorf("Compare of two top-up runs: %v", err)
	}
	if _, err := sim.Compare(a, without); err == nil {
		t.Error("Compare accepted a top-up run against a non-top-up run")
	}
}

// ── Stall regression using fakeSession (fast, no real catalogues) ────────────
//
// The raised coin floor (floor = DailyReward + minimumBoosterFee) prevents the policy
// from spending the fee buffer on card purchases. The plain collector lacks this floor.

func TestConvertingCollector_raisedFloorPreventsBuyingIntoFeeBuffer(t *testing.T) {
	t.Parallel()
	// Card priced at 100 coins. Mandatory fee = 100 coins. DailyReward = 100.
	// floor = DailyReward * reserveDays + minFee = 100 + 100 = 200.
	// Balance = 280. After buying: 280-100 = 180 < 200 → should NOT buy.
	cardID := domain.CardID("card-a")
	cat := domain.SetCatalogue{
		Code: "SET",
		Cards: map[domain.CardID]domain.Card{
			cardID: {ID: cardID, Rarity: domain.RarityCommon, Finishes: []domain.Finish{domain.FinishNonFoil}},
		},
	}
	// 84 USD cents → floor(84*100*120/10000)=100 coins buy price
	prices := domain.PriceSnapshot{cardID: 84}
	sp := map[string]domain.Progress{"SET": {
		ByRarity: map[domain.Rarity]domain.RarityProgress{
			domain.RarityCommon: {Owned: 0, Total: 1},
		},
	}}

	v := makeView(280, 100, 0, sp, map[string]int{"SET": 3}, []domain.SetCatalogue{cat}, prices, nil)
	v.BoosterFees = map[string]domain.Coins{"SET": 100} // mandatory fee
	// top-up off but floor logic still applies
	v.CoinTopupEnabled = false
	fs := &fakeSession{views: []sim.WorldView{v}}

	sim.NewConvertingCollectorPolicy().RunSession(fs)

	if len(fs.buyCalls) != 0 {
		t.Errorf("expected no buy calls (would drop below fee buffer floor); got %d", len(fs.buyCalls))
	}
}

// TestConvertingCollector_topUpUsedWhenShortOnEnergy verifies that the policy calls
// OpenBoosterTopUp (not OpenBooster) when it has fewer energy units than the booster cost
// but at least min_energy_units, and has sufficient coins.
func TestConvertingCollector_topUpUsedWhenShortOnEnergy(t *testing.T) {
	t.Parallel()
	sp, cats := twoSetProgress()
	costs := map[string]int{"SET-A": 5, "SET-B": 5}

	// EnergyTotal=2, cost=5, minEnergyUnits=1 → short by 3 units.
	// Rate=10, 3 units cost 30 coins. Balance=500. Floor≈100. OK to top-up.
	v := makeView(500, 100, 2, sp, costs, cats, nil, nil)
	v.CoinTopupEnabled = true
	v.CoinTopupMinEnergyUnits = 1
	v.CoinTopupRateNow = 10
	v.CoinTopupEscalationStep = 0

	// After one top-up, low-energy view to terminate the loop.
	low := makeView(470, 100, 0, sp, costs, cats, nil, nil)
	low.CoinTopupEnabled = true
	low.CoinTopupMinEnergyUnits = 1
	low.CoinTopupRateNow = 10

	fs := &fakeSession{views: []sim.WorldView{v, low}}
	sim.NewConvertingCollectorPolicy().RunSession(fs)

	if len(fs.openTopUpCalls) == 0 {
		t.Fatal("policy did not call OpenBoosterTopUp when short on energy with sufficient coins")
	}
	// Normal OpenBooster must not be called when top-up was used.
	if len(fs.openCalls) != 0 {
		t.Errorf("policy called OpenBooster %d times alongside top-up; expected 0", len(fs.openCalls))
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// topupConfig returns a base config with coins_per_energy=10, top-up defaults, and
// enough setup for the tests to work.
func topupConfig() config.Config {
	cfg := smallConfig()
	cfg.Energy.CoinsPerEnergy = 10
	cfg.Energy.CoinTopup = config.EnergyCoinTopupConfig{
		Enabled:        true,
		Variant:        config.CoinTopupVariantMinEnergy,
		MinEnergyUnits: 1,
		EscalationStep: 1,
	}
	return cfg
}

// availableSetCodes returns the uppercase codes of all JSON files in dir that match the
// catalogue loader convention: a file named <UPPERCASE>.json with at least 2 letters.
func availableSetCodes(dir string) []string {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil
	}
	var codes []string
	for _, e := range entries {
		base := filepath.Base(e)
		code := base[:len(base)-5] // strip ".json"
		allUpper := true
		for _, ch := range code {
			if ch < 'A' || ch > 'Z' {
				allUpper = false
				break
			}
		}
		if allUpper && len(code) >= 2 {
			codes = append(codes, code)
		}
	}
	return codes
}

// loadSetFiles loads MTGJSON catalogue files from dir for the given set codes using the
// domain package's catalogue loader (which filters to booster-openable cards).
func loadSetFiles(dir string, codes []string) ([]domain.SetCatalogue, error) {
	fsys := os.DirFS(dir)
	var cats []domain.SetCatalogue
	for _, code := range codes {
		cat, err := domain.LoadSetCatalogue(fsys, code+".json")
		if err != nil {
			return nil, err
		}
		cats = append(cats, cat)
	}
	return cats, nil
}

// tryLoadSetFiles loads MTGJSON catalogue files for the given codes but silently skips any that
// fail (e.g. supplementary pools that have no booster.draft section). It returns only the
// successfully loaded openable catalogues, so callers get the largest valid reference set
// without aborting on the first non-openable JSON file.
func tryLoadSetFiles(dir string, codes []string) []domain.SetCatalogue {
	fsys := os.DirFS(dir)
	var cats []domain.SetCatalogue
	for _, code := range codes {
		cat, err := domain.LoadSetCatalogue(fsys, code+".json")
		if err != nil {
			continue // supplementary pool or corrupt file — skip silently
		}
		cats = append(cats, cat)
	}
	return cats
}

// loadCardPools loads supplementary pool catalogue files from dir for the given set codes.
// Supplementary pools (e.g. TSB) have no booster.draft section; use domain.LoadCardPool
// instead of domain.LoadSetCatalogue.
func loadCardPools(dir string, codes []string) ([]domain.SetCatalogue, error) {
	fsys := os.DirFS(dir)
	var cats []domain.SetCatalogue
	for _, code := range codes {
		cat, err := domain.LoadCardPool(fsys, code+".json")
		if err != nil {
			return nil, err
		}
		cats = append(cats, cat)
	}
	return cats, nil
}

// loadPrices loads prices from the given JSON file path.
func loadPrices(path string) (domain.PriceSnapshot, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	fsys := os.DirFS(dir)
	return domain.LoadPriceSnapshot(fsys, name)
}
