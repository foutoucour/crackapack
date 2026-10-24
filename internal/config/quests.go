package config

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// QuestsConfig is how the quests of the game are drawn, see docs/core-features.md#quests. Its values
// are starting guesses that the tuning of the quests settles.
type QuestsConfig struct {
	// DailyCount and WeeklyCount are the quests on the board at once (default 3 and 1).
	DailyCount  int `yaml:"daily_count"`
	WeeklyCount int `yaml:"weekly_count"`
	// Templates are the kinds of quest a board draws from. Giving any replaces the default ones.
	Templates []QuestTemplateConfig `yaml:"templates"`
}

// QuestTemplateConfig is one kind of quest, as a configuration writes it.
type QuestTemplateConfig struct {
	// Period is "daily" or "weekly".
	Period string `yaml:"period"`
	// Action is "open_boosters", "sell_cards", "buy_cards", "win_trade", "spend_coins", "win_coins"
	// or "complete_dailies".
	Action string `yaml:"action"`
	// Filter is "colour" or "rarity" for a sell or a buy quest, and empty otherwise.
	Filter string `yaml:"filter"`
	// Target is the boosters, cards or coins that the quest asks for.
	Target int64 `yaml:"target"`
	// Reward is "win_card", "foil_card", "premium_booster", "unlock_set", "double_duplicates" or "swap".
	Reward string `yaml:"reward"`
	// MaxValue caps, in coins, the value of the card a win, foil or swap reward names.
	MaxValue int64 `yaml:"max_value"`
	// Weight is the odds of the template against the others of its period (at least 1).
	Weight int `yaml:"weight"`
}

// defaultQuests is the board of a new game: three daily quests and one weekly. ASSUMPTION: every
// target and cap is a starting guess to tune, see docs/todo.md.
func defaultQuests() QuestsConfig {
	return QuestsConfig{
		DailyCount:  3,
		WeeklyCount: 1,
		Templates: []QuestTemplateConfig{
			{Period: "daily", Action: "open_boosters", Target: 2, Reward: "win_card", MaxValue: 300, Weight: 1},
			{Period: "daily", Action: "open_boosters", Target: 3, Reward: "premium_booster", Weight: 1},
			{Period: "daily", Action: "sell_cards", Filter: "colour", Target: 6, Reward: "foil_card", MaxValue: 300, Weight: 1},
			{Period: "daily", Action: "sell_cards", Filter: "rarity", Target: 4, Reward: "swap", MaxValue: 300, Weight: 1},
			{Period: "daily", Action: "buy_cards", Filter: "rarity", Target: 2, Reward: "win_card", MaxValue: 500, Weight: 1},
			{Period: "daily", Action: "win_coins", Target: 400, Reward: "swap", MaxValue: 300, Weight: 1},
			{Period: "daily", Action: "spend_coins", Target: 500, Reward: "premium_booster", Weight: 1},
			{Period: "daily", Action: "win_trade", Target: 150, Reward: "foil_card", MaxValue: 500, Weight: 1},
			{Period: "weekly", Action: "complete_dailies", Target: 5, Reward: "unlock_set", Weight: 2},
			{Period: "weekly", Action: "complete_dailies", Target: 5, Reward: "double_duplicates", Weight: 2},
			{Period: "weekly", Action: "open_boosters", Target: 8, Reward: "unlock_set", Weight: 1},
			{Period: "weekly", Action: "spend_coins", Target: 3000, Reward: "double_duplicates", Weight: 1},
		},
	}
}

// Rules is the quest rules of q, with the names of the periods, actions and rewards read.
func (q QuestsConfig) Rules() (domain.QuestRules, error) {
	rules := domain.QuestRules{DailyCount: q.DailyCount, WeeklyCount: q.WeeklyCount}
	for i, t := range q.Templates {
		period, err := domain.ParseQuestPeriod(t.Period)
		if err != nil {
			return domain.QuestRules{}, fmt.Errorf("quests.templates[%d].period: %w", i, err)
		}
		action, err := domain.ParseQuestAction(t.Action)
		if err != nil {
			return domain.QuestRules{}, fmt.Errorf("quests.templates[%d].action: %w", i, err)
		}
		reward, err := domain.ParseQuestRewardKind(t.Reward)
		if err != nil {
			return domain.QuestRules{}, fmt.Errorf("quests.templates[%d].reward: %w", i, err)
		}
		rules.Templates = append(rules.Templates, domain.QuestTemplate{
			Period: period, Action: action, Target: t.Target, Reward: reward,
			FilterBy: domain.QuestFilterKind(t.Filter), MaxValue: domain.Coins(t.MaxValue), Weight: t.Weight,
		})
	}
	return rules, nil
}

// validateQuests checks the quests block against the rules the domain holds; validateBounds checks
// the upper bounds.
func validateQuests(q QuestsConfig) error {
	rules, err := q.Rules()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := rules.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}
