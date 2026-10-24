package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

func TestLoad_questDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rules, err := cfg.Quests.Rules()
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	if rules.DailyCount != 3 || rules.WeeklyCount != 1 {
		t.Errorf("counts = %d daily and %d weekly, want 3 and 1", rules.DailyCount, rules.WeeklyCount)
	}
	periods := map[domain.QuestPeriod]int{}
	rewards := map[domain.QuestRewardKind]bool{}
	for _, tpl := range rules.Templates {
		periods[tpl.Period]++
		rewards[tpl.Reward] = true
	}
	if periods[domain.QuestDaily] == 0 || periods[domain.QuestWeekly] == 0 {
		t.Errorf("templates per period = %v, want both periods", periods)
	}
	for _, r := range []domain.QuestRewardKind{
		domain.RewardWinCard, domain.RewardFoilCard, domain.RewardPremiumBooster,
		domain.RewardUnlockSet, domain.RewardDoubleDuplicates, domain.RewardSwap,
	} {
		if !rewards[r] {
			t.Errorf("no default template gives %s", r)
		}
	}
}

func TestLoad_questOverrides(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		check   func(t *testing.T, r domain.QuestRules)
	}{
		{
			name: "a count changes and the templates stay",
			yaml: "quests: {daily_count: 2}",
			check: func(t *testing.T, r domain.QuestRules) {
				t.Helper()
				if r.DailyCount != 2 || r.WeeklyCount != 1 || len(r.Templates) < 2 {
					t.Errorf("rules = %+v, want 2 daily, 1 weekly and the default templates", r)
				}
			},
		},
		{
			name: "templates replace the defaults",
			yaml: `quests:
  daily_count: 1
  weekly_count: 0
  templates:
    - {period: daily, action: sell_cards, filter: colour, target: 5, reward: foil_card, max_value: 400, weight: 2}`,
			check: func(t *testing.T, r domain.QuestRules) {
				t.Helper()
				want := domain.QuestTemplate{
					Period: domain.QuestDaily, Action: domain.QuestSellCards, FilterBy: domain.QuestFilterColour,
					Target: 5, Reward: domain.RewardFoilCard, MaxValue: 400, Weight: 2,
				}
				if len(r.Templates) != 1 || r.Templates[0] != want {
					t.Errorf("templates = %+v, want [%+v]", r.Templates, want)
				}
			},
		},
		{name: "a weekly quest that no template draws", yaml: "quests: {templates: [{period: daily, action: win_coins, target: 1, reward: premium_booster, weight: 1}]}", wantErr: "weekly"},
		{name: "an action that is not one", yaml: "quests: {templates: [{period: daily, action: dance, target: 1, reward: premium_booster, weight: 1}]}", wantErr: "dance"},
		{name: "a key of no template", yaml: "quests: {templates: [{period: daily, speed: 3}]}", wantErr: "speed"},
		{name: "a count above its bound", yaml: "quests: {daily_count: 1000000}", wantErr: "quests.daily_count"},
		{name: "a target above its bound", yaml: "quests: {daily_count: 0, weekly_count: 0, templates: [{period: daily, action: win_coins, target: 9000000000000000, reward: premium_booster, weight: 1}]}", wantErr: "target"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(strings.NewReader(tc.yaml))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load = %v, want an error naming %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			rules, err := cfg.Quests.Rules()
			if err != nil {
				t.Fatalf("Rules: %v", err)
			}
			tc.check(t, rules)
		})
	}
}
