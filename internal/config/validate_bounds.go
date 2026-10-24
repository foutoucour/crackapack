package config

import (
	"fmt"
	"maps"
	"slices"
)

// Upper bounds of the numeric keys. Without them a value near the int64 limit overflows the
// ledger or a price computation, and an absurd energy or supply makes a run last minutes. They sit
// far above any sensible setting, see ADR-0014.
const (
	// maxCoinRate is the documented realistic bound of the price-to-coins conversion.
	maxCoinRate = 1_000
	// maxBoosterValue bounds the booster value that makes a set expensive, in USDCents: $1,000,000,
	// a thousand times the dearest booster there is.
	maxBoosterValue = 100_000_000
	// maxPercent bounds the Counter's sell and buy percentages, as the price conversion documents.
	maxPercent = 10_000
	// maxCoins bounds every amount of coins a key sets. A year of it still sits far below int64.
	maxCoins = 1_000_000_000_000
	// maxEnergyRecharge, maxEnergyReserve, maxEnergySavings and maxEnergyCost bound the energy keys of
	// the player and of the Counter alike.
	maxEnergyRecharge = 1_000
	maxEnergyReserve  = 10_000
	maxEnergySavings  = 100_000
	maxEnergyCost     = 10_000
	// maxBoostersPerDay bounds the Counter's fixed supply.
	maxBoostersPerDay = 100_000
	// maxMarginPct bounds the NPC trait margin cap.
	maxMarginPct = 10_000
	// maxTraitParam bounds each parameter value of a trait.
	maxTraitParam = 10_000
	// maxUnlockSets bounds the unlock keys that count sets or stages: ten times the sets there are.
	maxUnlockSets = 1_000
	// maxCatchUpDays bounds the days a game catches up on: a year of them still starts in seconds.
	maxCatchUpDays = 365
	// maxQuestCount bounds the quests on a board: far more than a page shows.
	maxQuestCount = 100
	// maxQuestWeight bounds the odds of a quest template.
	maxQuestWeight = 1_000
)

// validateBounds checks every key that has an upper bound and returns an error naming the first one
// above it. It does not check lower bounds: each key's own validation does.
func validateBounds(cfg Config) error {
	checks := []struct {
		key   string
		value int64
		max   int64
	}{
		{"coin_rate", int64(cfg.CoinRate), maxCoinRate},
		{"expensive_booster_value", int64(cfg.ExpensiveBoosterValue), maxBoosterValue},
		{"counter_sell_pct", int64(cfg.CounterSellPct), maxPercent},
		{"counter_buy_pct", int64(cfg.CounterBuyPct), maxPercent},
		{"counter_foil_pct", int64(cfg.CounterFoilPct), maxPercent},
		{"welcome_bonus", int64(cfg.WelcomeBonus), maxCoins},
		{"daily_reward", int64(cfg.DailyReward), maxCoins},
		{"counter_budget.initial", int64(cfg.CounterBudget.Initial), maxCoins},
		{"counter_budget.daily_float", int64(cfg.CounterBudget.DailyFloat), maxCoins},
		{"counter_supply.coins_per_energy", int64(cfg.CounterSupply.CoinsPerEnergy), maxCoins},
		{"counter_supply.boosters_per_day", int64(cfg.CounterSupply.BoostersPerDay), maxBoostersPerDay},
		{"energy.coins_per_energy", int64(cfg.Energy.CoinsPerEnergy), maxCoins},
		{"energy.recharge_per_tick", int64(cfg.Energy.RechargePerTick), maxEnergyRecharge},
		{"energy.reserve_cap", int64(cfg.Energy.ReserveCap), maxEnergyReserve},
		{"energy.savings_cap", int64(cfg.Energy.SavingsCap), maxEnergySavings},
		{"counter_supply.energy.recharge_per_tick", int64(cfg.CounterSupply.Energy.RechargePerTick), maxEnergyRecharge},
		{"counter_supply.energy.reserve_cap", int64(cfg.CounterSupply.Energy.ReserveCap), maxEnergyReserve},
		{"counter_supply.energy.savings_cap", int64(cfg.CounterSupply.Energy.SavingsCap), maxEnergySavings},
		{"npcs.max_margin_pct", int64(cfg.NPCs.MaxMarginPct), maxMarginPct},
		{"unlock.sets_per_unlock", int64(cfg.Unlock.SetsPerUnlock), maxUnlockSets},
		{"unlock.stages", int64(cfg.Unlock.Stages), maxUnlockSets},
		{"unlock.start_sets", int64(cfg.Unlock.StartSets), maxUnlockSets},
		{"catch_up_days", int64(cfg.CatchUpDays), maxCatchUpDays},
		{"quests.daily_count", int64(cfg.Quests.DailyCount), maxQuestCount},
		{"quests.weekly_count", int64(cfg.Quests.WeeklyCount), maxQuestCount},
	}
	for i, entry := range cfg.Energy.CostTable {
		checks = append(checks, struct {
			key   string
			value int64
			max   int64
		}{fmt.Sprintf("energy.cost_table[%d].cost", i), int64(entry.Cost), maxEnergyCost})
	}
	for i, t := range cfg.Quests.Templates {
		checks = append(checks, []struct {
			key   string
			value int64
			max   int64
		}{
			{fmt.Sprintf("quests.templates[%d].target", i), t.Target, maxCoins},
			{fmt.Sprintf("quests.templates[%d].max_value", i), t.MaxValue, maxCoins},
			{fmt.Sprintf("quests.templates[%d].weight", i), int64(t.Weight), maxQuestWeight},
		}...)
	}
	// Trait parameter upper bounds: archetypes in name order, params in sorted order.
	for _, aname := range slices.Sorted(maps.Keys(cfg.NPCs.Archetypes)) {
		a := cfg.NPCs.Archetypes[aname]
		for _, tname := range slices.Sorted(maps.Keys(a.TraitParams)) {
			params := a.TraitParams[tname]
			for _, pname := range slices.Sorted(maps.Keys(params)) {
				checks = append(checks, struct {
					key   string
					value int64
					max   int64
				}{
					fmt.Sprintf("npcs.archetypes.%s.trait_params.%s.%s", aname, tname, pname),
					int64(params[pname]),
					maxTraitParam,
				})
			}
		}
	}
	for _, c := range checks {
		if c.value > c.max {
			return fmt.Errorf("config: %s must be <= %d, got %d", c.key, c.max, c.value)
		}
	}
	return nil
}
