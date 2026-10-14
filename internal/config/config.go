// Package config loads and validates simulation configuration from a YAML source.
// It translates the YAML file into a typed [Config] that the composition root
// (cmd/crackapack) passes to the simulation engine.
//
// Business rules live in internal/domain; this package is infrastructure
// for the simulation layer and imports no database, HTTP or OS packages.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	yaml "go.yaml.in/yaml/v3"

	"github.com/foutoucour/crackapack/internal/domain"
)

// nullPruneAndEncode prunes null-valued mapping entries from the YAML node
// tree and re-encodes to bytes. It is called by Load before decoding so that
// a key whose value is null is treated as absent and the pre-populated default
// applies. The returned bytes are used both for decoding and for rebuilding the
// line→key map used in error enrichment.
//
// If the document is empty or null, nil is returned and the caller should skip
// decoding so that defaults are preserved.
func nullPruneAndEncode(data []byte) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		// Syntax error: return nil so the caller uses the original bytes and
		// the KnownFields decoder produces the real error message.
		return nil, nil //nolint:nilerr
	}
	if isNullDoc(&root) {
		return nil, nil
	}
	pruneNulls(&root)
	pruned, err := yaml.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("encode pruned config: %w", err)
	}
	return pruned, nil
}

// Comparison is the operator used when evaluating a [CalibrationTarget].
// The three values cover all documented calibration criteria:
// lower-bound, upper-bound, and range checks.
type Comparison string

const (
	// ComparisonAtLeast passes when the metric value is >= Min.
	ComparisonAtLeast Comparison = "at_least"
	// ComparisonAtMost passes when the metric value is <= Max.
	ComparisonAtMost Comparison = "at_most"
	// ComparisonBetween passes when Min <= metric value <= Max.
	ComparisonBetween Comparison = "between"
)

// CalibrationTarget defines a pass/fail condition on a simulation metric.
// Evaluation against real metric values is performed by T-011; this type only
// models and validates the configuration. Whether Metric names a real metric
// is therefore not checked here.
type CalibrationTarget struct {
	// Metric is the name of the metric to evaluate. Must be non-empty.
	Metric string `yaml:"metric"`

	// Comparison is the operator to apply when checking the threshold.
	Comparison Comparison `yaml:"comparison"`

	// Min is the lower bound. Required for [ComparisonAtLeast] and [ComparisonBetween].
	Min *float64 `yaml:"min,omitempty"`

	// Max is the upper bound. Required for [ComparisonAtMost] and [ComparisonBetween].
	Max *float64 `yaml:"max,omitempty"`
}

// EnergyCost maps an upper booster-value/catalogue-median ratio threshold
// to an energy cost. A [EnergyConfig.CostTable] entry applies when the
// booster's ratio in percent is <= Threshold. The entry with the highest
// Threshold acts as the catch-all for any ratio above it.
// The table must be sorted in ascending order of Threshold.
type EnergyCost struct {
	// Threshold is the upper ratio bound in whole percent: 50 means ×0.5,
	// 100 means ×1, 200 means ×2, 400 means ×4. Using [domain.Percent] keeps
	// the ratio arithmetic integer-only, consistent with the no-float-for-money rule.
	Threshold domain.Percent `yaml:"threshold"`

	// Cost is the energy units spent to open one booster when the ratio is <= Threshold.
	Cost int `yaml:"cost"`
}

// CoinTopupVariants lists the accepted values for EnergyCoinTopupConfig.Variant.
const (
	// CoinTopupVariantMinEnergy applies a minimum real-energy requirement: the player must spend
	// at least MinEnergyUnits real energy; the rest may be substituted with coins at rate p.
	CoinTopupVariantMinEnergy = "min_energy"
	// CoinTopupVariantEscalating applies an escalating price: the first unit of energy bought
	// with coins within a day costs p; each successive unit costs p + k*EscalationStep (where k
	// is the number of units already bought today). Resets at day start.
	CoinTopupVariantEscalating = "escalating"
	// CoinTopupVariantBoth applies both the minimum-energy requirement and the escalating price.
	CoinTopupVariantBoth = "both"
)

// EnergyCoinTopupConfig is the optional feature that lets the player substitute coins for missing
// energy when opening a booster, at rate p = energy.coins_per_energy per energy unit. Off by
// default; needs coins_per_energy > 0.
//
// A booster can never be opened with coins alone: at least MinEnergyUnits of real energy is
// always required. Three variants control the substitution price:
//   - "min_energy": constant price p; the player just must hold MinEnergyUnits real energy.
//   - "escalating": price starts at p and rises by EscalationStep for each successive unit
//     bought with coins within a day (the k-th unit costs p + k*EscalationStep). Resets daily.
//   - "both": the minimum-energy requirement and the escalating price both apply.
type EnergyCoinTopupConfig struct {
	// Enabled turns coin top-up on. Default false.
	Enabled bool `yaml:"enabled"`
	// Variant is "min_energy", "escalating", or "both". Default "min_energy".
	Variant string `yaml:"variant"`
	// MinEnergyUnits is the minimum real energy the player must have to open a booster via
	// top-up. The booster cannot be opened with coins only. Must be >= 1. Default 1.
	MinEnergyUnits int `yaml:"min_energy_units"`
	// EscalationStep is the additional coins per successive unit bought within a day, for
	// "escalating" or "both". The k-th unit (0-indexed within the day) costs p + k*EscalationStep.
	// Must be >= 1 when the escalating variant is active. Default 1.
	EscalationStep domain.Coins `yaml:"escalation_step"`
}

// EnergyConfig holds the energy recharge parameters described in docs/core-features.md.
type EnergyConfig struct {
	// RechargePerTick is the energy added to the reserve each 10-minute tick (default 1).
	RechargePerTick int `yaml:"recharge_per_tick"`

	// ReserveCap is the maximum energy held in the main reserve (default 10).
	ReserveCap int `yaml:"reserve_cap"`

	// SavingsIntervalTicks is the number of ticks between savings recharge events.
	// Default 3 (3 ticks × 10 min = 30 min, as documented in core-features.md).
	SavingsIntervalTicks int `yaml:"savings_interval_ticks"`

	// SavingsCap is the maximum energy held in the savings pool (default 200).
	SavingsCap int `yaml:"savings_cap"`

	// CoinsPerEnergy is the coin price of one energy point, p. A booster worth more than the
	// energy cap pays its value minus the cap times p in coins, on top of the cap's energy. The
	// default 0 means no coin fee.
	CoinsPerEnergy domain.Coins `yaml:"coins_per_energy"`

	// CoinTopup is the optional feature that lets the player substitute coins for missing energy
	// when opening a booster. Off by default; needs coins_per_energy > 0.
	CoinTopup EnergyCoinTopupConfig `yaml:"coin_topup"`

	// CostTable maps booster-value/catalogue-median ratio thresholds to energy costs.
	// Must be non-empty and sorted in ascending order of Threshold. The last entry
	// is the catch-all for any ratio above the highest Threshold.
	// Default table (from docs/core-features.md):
	//   Threshold 50  (×0.5) → Cost 3
	//   Threshold 100 (×1)   → Cost 5
	//   Threshold 200 (×2)   → Cost 8
	//   Threshold 400 (×4)   → Cost 12 (catch-all)
	CostTable []EnergyCost `yaml:"cost_table"`
}

// Modes of the Counter's own supply of boosters.
const (
	// CounterSupplyNone is the default: the Counter opens nothing.
	CounterSupplyNone = "none"
	// CounterSupplyFixed makes the Counter open a fixed number of boosters a day, with no energy.
	CounterSupplyFixed = "fixed"
	// CounterSupplyEnergy makes the Counter open boosters from an energy budget of its own, paying
	// the energy cost of each set.
	CounterSupplyEnergy = "energy"
)

// CounterEnergyConfig is the energy of the Counter in energy mode. It has the shape of the
// player's energy, only much larger.
type CounterEnergyConfig struct {
	// RechargePerTick is the energy added to the reserve each tick.
	RechargePerTick int `yaml:"recharge_per_tick"`
	// ReserveCap is the most energy the main reserve holds.
	ReserveCap int `yaml:"reserve_cap"`
	// SavingsIntervalTicks is the ticks between savings recharges once the reserve is full.
	SavingsIntervalTicks int `yaml:"savings_interval_ticks"`
	// SavingsCap is the most energy the savings pool holds.
	SavingsCap int `yaml:"savings_cap"`
}

// CounterSupplyConfig is how the Counter stocks itself by opening boosters, as a special player.
type CounterSupplyConfig struct {
	// Mode is CounterSupplyNone (the default), CounterSupplyFixed or CounterSupplyEnergy.
	Mode string `yaml:"mode"`

	// BoostersPerDay is the number of boosters the Counter opens each day in fixed mode.
	BoostersPerDay int `yaml:"boosters_per_day"`

	// Energy is the Counter's energy in energy mode.
	Energy CounterEnergyConfig `yaml:"energy"`

	// CoinsPerEnergy is the coin price of one energy point, p. A booster worth more than the
	// energy cap costs its value minus the cap times p in coins, paid from the treasury. It needs
	// energy mode and the budget, and 0 means no coin fee.
	CoinsPerEnergy domain.Coins `yaml:"coins_per_energy"`
}

// CounterBudgetConfig gives the Counter a finite treasury of coins. Without it the Counter mints
// the coins it pays to the player.
type CounterBudgetConfig struct {
	// Enabled turns the budget on. The default false keeps a Counter that mints its coins.
	Enabled bool `yaml:"enabled"`

	// Initial is the coins in the treasury at the start of the run.
	Initial domain.Coins `yaml:"initial"`

	// DailyFloat is the coins added to the treasury each day, the one faucet of a closed Counter.
	DailyFloat domain.Coins `yaml:"daily_float"`
}

// ArchetypeConfig is the supply behaviour of one NPC archetype, see docs/core-features.md.
// An archetype given in YAML replaces the default of the same name as a whole: every field
// must be set.
type ArchetypeConfig struct {
	// Share is the archetype's weight in the population; only the ratio between shares matters.
	Share int `yaml:"share"`

	// MinBoostersPerDay and MaxBoostersPerDay bound the activity rhythm, inclusive. Each NPC
	// draws its own rhythm in that range once.
	MinBoostersPerDay int `yaml:"min_boosters_per_day"`
	MaxBoostersPerDay int `yaml:"max_boosters_per_day"`

	// KeepCopies is how many copies of a printing the NPC keeps before selling the rest to the
	// Counter. Must be in [1, 4].
	KeepCopies int `yaml:"keep_copies"`

	// FocusSet makes each NPC of the archetype open only one set, drawn at generation.
	FocusSet bool `yaml:"focus_set"`

	// Behaviour names the registered trade behaviour of the archetype, see
	// domain.BehaviourNames. It may be empty while trades are off, and is required for an
	// archetype with a share when they are on.
	Behaviour string `yaml:"behaviour"`

	// WishSize is how many cards an NPC of the archetype wishes for at once.
	WishSize int `yaml:"wish_size"`

	// MarginPct is the margin an NPC demands on top of the reference value it gives. Never
	// negative: an NPC never gives more value than it gets.
	MarginPct domain.Percent `yaml:"margin_pct"`

	// OfferChancePct is the chance, per tick, that an NPC with a match proposes a trade. In
	// [0, 100].
	OfferChancePct int `yaml:"offer_chance_pct"`
}

// NPCConfig holds the NPC crowd parameters.
type NPCConfig struct {
	// Population is the number of NPCs. The default 0 runs the simulation without NPCs.
	Population int `yaml:"population"`

	// SurplusHoldDays is how many days an NPC keeps the copies above its keep threshold on a
	// shelf, where they can be traded, before selling them to the Counter. It applies only when
	// trades are enabled, and trades need at least 1, see ADR-0011. Default 2.
	SurplusHoldDays int `yaml:"surplus_hold_days"`

	// Archetypes maps an archetype name to its behaviour. A partial YAML map merges with the
	// defaults per name. ASSUMPTION defaults (pending calibration): see defaults.
	Archetypes map[string]ArchetypeConfig `yaml:"archetypes"`
}

// TradesConfig holds the parameters of the trades between the player and the NPCs, see
// docs/core-features.md. Trades are off by default: a run without them is unchanged.
type TradesConfig struct {
	// Enabled turns trades on. It needs an NPC crowd and a surplus hold of at least one day.
	Enabled bool `yaml:"enabled"`

	// OfferExpiryDays is how many days an offer stays open, the day it is made included
	// (default 3).
	OfferExpiryDays int `yaml:"offer_expiry_days"`

	// MaxCardsPerSide bounds the cards each side of an offer gives (default 10).
	MaxCardsPerSide int `yaml:"max_cards_per_side"`

	// DuplicateKeep is how many copies of a printing the player keeps out of NPC proposals; the
	// copies above it are its duplicates. In [1, 4] (default 1).
	DuplicateKeep int `yaml:"duplicate_keep"`

	// HistorySize is how many answered offers the Exchange remembers. 0 keeps none, which is
	// enough for the metrics.
	HistorySize int `yaml:"history_size"`

	// MaxOpenOffersPerDay bounds the offers NPCs make the player each day (default 20).
	// ASSUMPTION: guessed, pending calibration.
	MaxOpenOffersPerDay int `yaml:"max_open_offers_per_day"`
}

// Config holds the tunable parameters for one simulation run.
// Fields omitted from the YAML file fall back to the documented defaults.
// Fields marked ASSUMPTION have no authoritative value yet and are to be
// calibrated through simulation.
type Config struct {
	// CoinRate is the number of coins one US dollar is worth.
	// The default is 100, see docs/adr/0006-coin-rate-is-100-per-dollar.md.
	CoinRate domain.CoinRate `yaml:"coin_rate"`

	// CounterSellPct is the percentage of the reference value the player is paid
	// when the player sells a card to the Counter (default 60).
	CounterSellPct domain.Percent `yaml:"counter_sell_pct"`

	// CounterBuyPct is the percentage of the reference value the player pays
	// when the player buys a card from the Counter (default 120).
	// Invariant: CounterSellPct <= CounterBuyPct; a sell percentage above the buy
	// percentage lets the player profit from a round trip through the Counter.
	CounterBuyPct domain.Percent `yaml:"counter_buy_pct"`

	// WelcomeBonus is the coins awarded on first launch (default 200, per core-features.md).
	WelcomeBonus domain.Coins `yaml:"welcome_bonus"`

	// DailyReward is the coins awarded each in-game day (default 100, per core-features.md).
	DailyReward domain.Coins `yaml:"daily_reward"`

	// CounterShelfDays is how many days a card sold to the Counter, by the player or by the
	// NPCs, stays in its stock before it vanishes. 0 makes the Counter a sink, and -1, the
	// default, keeps cards forever.
	CounterShelfDays int `yaml:"counter_shelf_days"`

	// CounterSupply makes the Counter open boosters to stock itself. The default opens none.
	CounterSupply CounterSupplyConfig `yaml:"counter_supply"`

	// CounterBudget gives the Counter a finite treasury. The default is off.
	CounterBudget CounterBudgetConfig `yaml:"counter_budget"`

	// Energy holds the energy recharge parameters.
	Energy EnergyConfig `yaml:"energy"`

	// InitialStock is the number of each-rarity card copies stocked in the Counter
	// at simulation start. A partial YAML map merges with the defaults per key;
	// only the specified rarities are overridden.
	// ASSUMPTION defaults (pending calibration): common 20, uncommon 8, rare 2, special 2, mythic 2.
	InitialStock map[domain.Rarity]int `yaml:"initial_stock"`

	// FallbackPrice is the price in USDCents for cards absent from the price snapshot.
	// ASSUMPTION: default 1 cent; pending calibration.
	FallbackPrice domain.USDCents `yaml:"fallback_price"`

	// NPCs holds the NPC crowd parameters.
	NPCs NPCConfig `yaml:"npcs"`

	// Trades holds the trade parameters.
	Trades TradesConfig `yaml:"trades"`

	// SessionsPerDay is the number of play sessions the simulated player runs each day.
	// ASSUMPTION: default 3; pending calibration.
	SessionsPerDay int `yaml:"sessions_per_day"`

	// RunDays is the number of in-game days to simulate (default 90, per core-features.md).
	RunDays int `yaml:"run_days"`

	// Seed is the random seed for the simulation. 0 is a valid seed value.
	Seed uint64 `yaml:"seed"`

	// Targets are calibration pass/fail conditions evaluated by T-011.
	Targets []CalibrationTarget `yaml:"targets"`

	// SweepRates is the grid of coin-per-USD rates used by the sweep subcommand.
	// Each entry must be > 0 and the list must be strictly ascending.
	// Default: [10, 25, 50, 100, 250, 500].
	SweepRates []domain.CoinRate `yaml:"sweep_rates"`
}

// isKnownRarity reports whether r is a rarity initial_stock may name.
func isKnownRarity(r domain.Rarity) bool {
	switch r {
	case domain.RarityCommon, domain.RarityUncommon, domain.RarityRare, domain.RaritySpecial, domain.RarityMythic:
		return true
	}
	return false
}

// defaults returns a Config pre-populated with the documented starting values.
// [Load] overlays the YAML file on top, leaving absent fields at their defaults.
func defaults() Config {
	return Config{
		CoinRate:         100, // see docs/adr/0006-coin-rate-is-100-per-dollar.md
		CounterSellPct:   60,
		CounterBuyPct:    120,
		CounterShelfDays: -1,
		// ASSUMPTION: the budget values are starting guesses.
		CounterBudget: CounterBudgetConfig{Initial: 100000},
		// ASSUMPTION: the Counter's energy is a starting guess, twenty times the player's recharge.
		CounterSupply: CounterSupplyConfig{
			Mode:   CounterSupplyNone,
			Energy: CounterEnergyConfig{RechargePerTick: 20, ReserveCap: 100, SavingsIntervalTicks: 1, SavingsCap: 1000},
		},
		WelcomeBonus: 200,
		DailyReward:  100,
		Energy: EnergyConfig{
			RechargePerTick:      1,
			ReserveCap:           10,
			SavingsIntervalTicks: 3, // 3 ticks × 10 min = 30 min (core-features.md)
			SavingsCap:           200,
			CoinTopup: EnergyCoinTopupConfig{
				Variant:        CoinTopupVariantMinEnergy,
				MinEnergyUnits: 1,
				EscalationStep: 1,
			},
			CostTable: []EnergyCost{
				{Threshold: 50, Cost: 3},   // booster value ≤ 0.5× catalogue median
				{Threshold: 100, Cost: 5},  // ≤ 1×
				{Threshold: 200, Cost: 8},  // ≤ 2×
				{Threshold: 400, Cost: 12}, // catch-all (4× and above)
			},
		},
		// ASSUMPTION: initial stock values are starting assumptions pending calibration.
		InitialStock: map[domain.Rarity]int{
			domain.RarityCommon:   20,
			domain.RarityUncommon: 8,
			domain.RarityRare:     2,
			domain.RaritySpecial:  2,
			domain.RarityMythic:   2,
		},
		FallbackPrice: 1, // ASSUMPTION
		// ASSUMPTION: archetype shares, rhythms, keep thresholds and the trade settings are starting
		// guesses pending calibration. A casual is the main source of supply, a whale opens far
		// more than the rest. The casual and speculator margins come from docs/core-features.md.
		NPCs: NPCConfig{
			SurplusHoldDays: 2,
			Archetypes: map[string]ArchetypeConfig{
				"casual":             {Share: 50, MinBoostersPerDay: 4, MaxBoostersPerDay: 8, KeepCopies: 1, Behaviour: "casual", WishSize: 5, MarginPct: 0, OfferChancePct: 2},
				"playset_hunter":     {Share: 15, MinBoostersPerDay: 3, MaxBoostersPerDay: 6, KeepCopies: 4, Behaviour: "playset_hunter", WishSize: 5, MarginPct: 5, OfferChancePct: 2},
				"targeted_collector": {Share: 20, MinBoostersPerDay: 2, MaxBoostersPerDay: 5, KeepCopies: 1, FocusSet: true, Behaviour: "targeted_collector", WishSize: 5, MarginPct: 5, OfferChancePct: 2},
				"speculator":         {Share: 10, MinBoostersPerDay: 2, MaxBoostersPerDay: 4, KeepCopies: 2, Behaviour: "speculator", WishSize: 5, MarginPct: 15, OfferChancePct: 1},
				"whale":              {Share: 5, MinBoostersPerDay: 30, MaxBoostersPerDay: 50, KeepCopies: 1, Behaviour: "whale", WishSize: 3, MarginPct: 10, OfferChancePct: 1},
			},
		},
		Trades:         TradesConfig{OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, MaxOpenOffersPerDay: 20},
		SessionsPerDay: 3, // ASSUMPTION
		RunDays:        90,
		SweepRates:     []domain.CoinRate{10, 25, 50, 100, 250, 500},
	}
}

// Load reads a YAML configuration from r, overlays it on the defaults, and
// validates the result. It returns an error naming the offending key for
// unknown keys, type mismatches, and invalid values.
//
// A mapping key whose YAML value is null is treated as absent: the
// pre-populated default applies. This covers every key including nested ones
// (energy and its sub-keys, cost_table, sweep_rates, targets, initial_stock
// and each rarity). A null item inside a sequence is rejected with a path-named
// error, see ADR-0017.
//
// The caller is responsible for opening the file; Load accepts an [io.Reader]
// so tests can use [strings.NewReader] without touching the filesystem.
func Load(r io.Reader) (Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Config{}, fmt.Errorf("load config: read: %w", err)
	}

	if err := checkStrictInput(data); err != nil {
		return Config{}, err
	}

	// Prune null-valued mapping entries so they are treated as absent.
	// nullPruneAndEncode returns nil when the document is empty/null or has a
	// syntax error; in both cases we fall through to the original bytes.
	if pruned, pruneErr := nullPruneAndEncode(data); pruneErr != nil {
		return Config{}, fmt.Errorf("load config: %w", pruneErr)
	} else if pruned != nil {
		data = pruned
	}

	lineKey := buildLineKeyMap(data)

	// Completely empty file is valid: the pre-populated defaults are kept as-is.
	cfg := defaults()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, enrichYAMLError(err, lineKey)
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validate checks semantic constraints on cfg and returns an error naming the
// offending key on failure.
func validate(cfg Config) error {
	if err := validateBounds(cfg); err != nil {
		return err
	}
	if cfg.CoinRate <= 0 {
		return fmt.Errorf("config: coin_rate must be > 0, got %d", cfg.CoinRate)
	}
	if cfg.CounterSellPct <= 0 {
		return fmt.Errorf("config: counter_sell_pct must be > 0, got %d", cfg.CounterSellPct)
	}
	if cfg.CounterShelfDays < -1 {
		return fmt.Errorf("config: counter_shelf_days must be >= -1 (-1 keeps cards forever), got %d", cfg.CounterShelfDays)
	}
	if cfg.CounterBuyPct <= 0 {
		return fmt.Errorf("config: counter_buy_pct must be > 0, got %d", cfg.CounterBuyPct)
	}
	if cfg.CounterSellPct > cfg.CounterBuyPct {
		return fmt.Errorf("config: counter_sell_pct (%d) must be <= counter_buy_pct (%d); a sell percentage above buy lets the player profit from a round trip",
			cfg.CounterSellPct, cfg.CounterBuyPct)
	}
	if cfg.WelcomeBonus < 0 {
		return fmt.Errorf("config: welcome_bonus must be >= 0, got %d", cfg.WelcomeBonus)
	}
	if cfg.DailyReward < 0 {
		return fmt.Errorf("config: daily_reward must be >= 0, got %d", cfg.DailyReward)
	}
	if cfg.FallbackPrice < 0 {
		return fmt.Errorf("config: fallback_price must be >= 0, got %d", cfg.FallbackPrice)
	}
	if cfg.SessionsPerDay <= 0 {
		return fmt.Errorf("config: sessions_per_day must be > 0, got %d", cfg.SessionsPerDay)
	}
	if cfg.SessionsPerDay > domain.TicksPerDay {
		return fmt.Errorf(
			"config: sessions_per_day (%d) must be <= %d (domain.TicksPerDay); above that value multiple sessions land on the same tick",
			cfg.SessionsPerDay, domain.TicksPerDay,
		)
	}
	if cfg.RunDays <= 0 {
		return fmt.Errorf("config: run_days must be > 0, got %d", cfg.RunDays)
	}
	for r, v := range cfg.InitialStock {
		if !isKnownRarity(r) {
			return fmt.Errorf("config: initial_stock has an unknown rarity %q, want one of common, uncommon, rare, special, mythic", r)
		}
		if v < 0 {
			return fmt.Errorf("config: initial_stock[%s] must be >= 0, got %d", r, v)
		}
	}
	if err := validateEnergy(cfg.Energy); err != nil {
		return err
	}
	if err := validateCounterBudget(cfg.CounterBudget); err != nil {
		return err
	}
	if err := validateCounterSupply(cfg); err != nil {
		return err
	}
	if err := validateNPCs(cfg.NPCs); err != nil {
		return err
	}
	if err := validateTrades(cfg); err != nil {
		return err
	}
	for i, t := range cfg.Targets {
		if err := validateTarget(i, t, cfg.RunDays); err != nil {
			return err
		}
	}
	if err := ValidateSweepRates(cfg.SweepRates); err != nil {
		return err
	}
	return nil
}
