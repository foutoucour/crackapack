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
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"github.com/foutoucour/crackapack/internal/domain"
)

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
// to an energy cost. A [Config.Energy].CostTable entry applies when the
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

	// CostTable maps booster-value/catalogue-median ratio thresholds to energy costs.
	// Must be sorted in ascending order of Threshold. The last entry is the catch-all.
	// Default table (from docs/core-features.md):
	//   Threshold 50  (×0.5) → Cost 3
	//   Threshold 100 (×1)   → Cost 5
	//   Threshold 200 (×2)   → Cost 8
	//   Threshold 400 (×4)   → Cost 12 (catch-all)
	CostTable []EnergyCost `yaml:"cost_table"`
}

// Config holds the tunable parameters for one simulation run.
// Fields omitted from the YAML file fall back to the documented defaults.
// Fields marked ASSUMPTION have no authoritative value yet and are to be
// calibrated through simulation.
type Config struct {
	// CoinRate is the number of coins one US dollar is worth.
	// ASSUMPTION: default 100 is a placeholder; the sweep calibrates this.
	CoinRate domain.CoinRate `yaml:"coin_rate"`

	// CounterSellPct is the percentage of a card's reference value the Counter
	// pays the player when buying a card (default 60).
	CounterSellPct domain.Percent `yaml:"counter_sell_pct"`

	// CounterBuyPct is the percentage of a card's reference value the Counter
	// charges the player when selling a card (default 120).
	// Invariant: CounterSellPct <= CounterBuyPct; a sell rate above the buy rate
	// lets the player profit from a round trip through the Counter.
	CounterBuyPct domain.Percent `yaml:"counter_buy_pct"`

	// WelcomeBonus is the coins awarded on first launch (default 200, per core-features.md).
	WelcomeBonus domain.Coins `yaml:"welcome_bonus"`

	// DailyReward is the coins awarded each in-game day (default 100, per core-features.md).
	DailyReward domain.Coins `yaml:"daily_reward"`

	// Energy holds the energy recharge parameters.
	Energy EnergyConfig `yaml:"energy"`

	// InitialStock is the number of each-rarity card copies stocked in the Counter
	// at simulation start. A partial YAML map merges with the defaults per key;
	// only the specified rarities are overridden.
	// ASSUMPTION defaults (pending calibration): common 20, uncommon 8, rare 2, special 2.
	InitialStock map[domain.Rarity]int `yaml:"initial_stock"`

	// FallbackPrice is the price in USDCents for cards absent from the price snapshot.
	// ASSUMPTION: default 1 cent; pending calibration.
	FallbackPrice domain.USDCents `yaml:"fallback_price"`

	// SessionsPerDay is the number of play sessions the simulated player runs each day.
	// ASSUMPTION: default 3; pending calibration.
	SessionsPerDay int `yaml:"sessions_per_day"`

	// RunDays is the number of in-game days to simulate (default 90, per core-features.md).
	RunDays int `yaml:"run_days"`

	// Seed is the random seed for the simulation. 0 is a valid seed value.
	Seed uint64 `yaml:"seed"`

	// Targets are calibration pass/fail conditions evaluated by T-011.
	Targets []CalibrationTarget `yaml:"targets"`
}

// defaults returns a Config pre-populated with the documented starting values.
// [Load] overlays the YAML file on top, leaving absent fields at their defaults.
func defaults() Config {
	return Config{
		CoinRate:       100, // ASSUMPTION: calibrated by sweep; see docs/specs-review.md
		CounterSellPct: 60,
		CounterBuyPct:  120,
		WelcomeBonus:   200,
		DailyReward:    100,
		Energy: EnergyConfig{
			RechargePerTick:      1,
			ReserveCap:           10,
			SavingsIntervalTicks: 3, // 3 ticks × 10 min = 30 min (core-features.md)
			SavingsCap:           200,
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
		},
		FallbackPrice:  1, // ASSUMPTION
		SessionsPerDay: 3, // ASSUMPTION
		RunDays:        90,
	}
}

// Load reads a YAML configuration from r, overlays it on the defaults, and
// validates the result. It returns an error naming the offending key for
// unknown keys, type mismatches, and invalid values.
//
// The caller is responsible for opening the file; Load accepts an [io.Reader]
// so tests can use [strings.NewReader] without touching the filesystem.
func Load(r io.Reader) (Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Config{}, fmt.Errorf("load config: read: %w", err)
	}

	lineKey := buildLineKeyMap(data)

	cfg := defaults()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			// Completely empty file: treat as empty mapping, keep all defaults.
		} else {
			return Config{}, enrichYAMLError(err, lineKey)
		}
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// buildLineKeyMap parses data into a yaml.Node tree and returns a map from each
// value node's line number to its dotted key path (e.g. "energy.reserve_cap").
// This enriches TypeError messages with the key name rather than just a line number.
func buildLineKeyMap(data []byte) map[int]string {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil
	}
	m := make(map[int]string)
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		walkNode(root.Content[0], "", m)
	}
	return m
}

// walkNode recursively walks a MappingNode, recording line -> dotted-key-path
// for every key and its value node. Nested mappings are walked with a dotted prefix.
func walkNode(n *yaml.Node, prefix string, m map[int]string) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		keyNode := n.Content[i]
		valNode := n.Content[i+1]
		key := keyNode.Value
		if prefix != "" {
			key = prefix + "." + key
		}
		m[keyNode.Line] = key
		m[valNode.Line] = key
		walkNode(valNode, key, m)
	}
}

// enrichYAMLError wraps a yaml decode error, replacing line numbers with key
// names where available. Unknown-field errors already carry the field name;
// type errors (yaml.TypeError) are enriched using the line-to-key map.
func enrichYAMLError(err error, lineKey map[int]string) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return fmt.Errorf("load config: %w", err)
	}
	parts := make([]string, 0, len(typeErr.Errors))
	for _, msg := range typeErr.Errors {
		parts = append(parts, enrichTypeLine(msg, lineKey))
	}
	return fmt.Errorf("load config: %s", strings.Join(parts, "; "))
}

// enrichTypeLine replaces the "line N: <problem>" prefix with "key: <problem>"
// when the line number maps to a known YAML key, leaving the message unchanged
// otherwise.
func enrichTypeLine(msg string, lineKey map[int]string) string {
	const linePrefix = "line "
	if !strings.HasPrefix(msg, linePrefix) {
		return msg
	}
	rest := msg[len(linePrefix):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return msg
	}
	lineNum, err := strconv.Atoi(rest[:colon])
	if err != nil {
		return msg
	}
	detail := strings.TrimSpace(rest[colon+1:])
	if key, ok := lineKey[lineNum]; ok {
		return key + ": " + detail
	}
	return msg
}

// validate checks semantic constraints on cfg and returns an error naming the
// offending key on failure.
func validate(cfg Config) error {
	if cfg.CoinRate <= 0 {
		return fmt.Errorf("config: coin_rate must be > 0, got %d", cfg.CoinRate)
	}
	if cfg.CounterSellPct <= 0 {
		return fmt.Errorf("config: counter_sell_pct must be > 0, got %d", cfg.CounterSellPct)
	}
	if cfg.CounterBuyPct <= 0 {
		return fmt.Errorf("config: counter_buy_pct must be > 0, got %d", cfg.CounterBuyPct)
	}
	if cfg.CounterSellPct > cfg.CounterBuyPct {
		return fmt.Errorf("config: counter_sell_pct (%d) must be <= counter_buy_pct (%d); a sell rate above buy lets the player profit from a round trip",
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
	if cfg.RunDays <= 0 {
		return fmt.Errorf("config: run_days must be > 0, got %d", cfg.RunDays)
	}
	for r, v := range cfg.InitialStock {
		if v < 0 {
			return fmt.Errorf("config: initial_stock[%s] must be >= 0, got %d", r, v)
		}
	}
	if err := validateCostTable(cfg.Energy.CostTable); err != nil {
		return err
	}
	for i, t := range cfg.Targets {
		if err := validateTarget(i, t); err != nil {
			return err
		}
	}
	return nil
}

// validateCostTable checks that the energy cost table is sorted ascending by Threshold.
func validateCostTable(table []EnergyCost) error {
	for i := 1; i < len(table); i++ {
		if table[i].Threshold <= table[i-1].Threshold {
			return fmt.Errorf(
				"config: energy.cost_table: entry %d threshold (%d) must be > entry %d threshold (%d); table must be sorted ascending",
				i, table[i].Threshold, i-1, table[i-1].Threshold,
			)
		}
	}
	return nil
}

// validateTarget checks that a CalibrationTarget has a non-empty metric,
// a recognised comparison, and the bounds required by that comparison.
func validateTarget(idx int, t CalibrationTarget) error {
	if t.Metric == "" {
		return fmt.Errorf("config: targets[%d].metric must not be empty", idx)
	}
	switch t.Comparison {
	case ComparisonAtLeast:
		if t.Min == nil {
			return fmt.Errorf("config: targets[%d].min is required for comparison %q", idx, t.Comparison)
		}
	case ComparisonAtMost:
		if t.Max == nil {
			return fmt.Errorf("config: targets[%d].max is required for comparison %q", idx, t.Comparison)
		}
	case ComparisonBetween:
		if t.Min == nil {
			return fmt.Errorf("config: targets[%d].min is required for comparison %q", idx, t.Comparison)
		}
		if t.Max == nil {
			return fmt.Errorf("config: targets[%d].max is required for comparison %q", idx, t.Comparison)
		}
		if *t.Min > *t.Max {
			return fmt.Errorf("config: targets[%d]: min (%g) must be <= max (%g)", idx, *t.Min, *t.Max)
		}
	default:
		return fmt.Errorf("config: targets[%d].comparison: unknown value %q; must be one of at_least, at_most, between", idx, t.Comparison)
	}
	return nil
}
