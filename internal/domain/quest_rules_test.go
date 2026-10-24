package domain_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func validTemplate() domain.QuestTemplate {
	return domain.QuestTemplate{
		Period: domain.QuestDaily, Action: domain.QuestOpenBoosters, Target: 2,
		Reward: domain.RewardWinCard, MaxValue: 300, Weight: 1,
	}
}

func TestQuestRulesValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(r *domain.QuestRules)
		wantErr string
	}{
		{name: "a valid rule set", mutate: func(*domain.QuestRules) {}},
		{name: "no daily and no weekly quest is allowed", mutate: func(r *domain.QuestRules) { r.DailyCount, r.WeeklyCount = 0, 0 }},
		{name: "a negative count", mutate: func(r *domain.QuestRules) { r.DailyCount = -1 }, wantErr: "daily_count"},
		{name: "a period that no template has", mutate: func(r *domain.QuestRules) { r.WeeklyCount = 1 }, wantErr: "weekly"},
		{name: "a target of zero", mutate: func(r *domain.QuestRules) { r.Templates[0].Target = 0 }, wantErr: "target"},
		{name: "a weight of zero", mutate: func(r *domain.QuestRules) { r.Templates[0].Weight = 0 }, wantErr: "weight"},
		{name: "an unknown action", mutate: func(r *domain.QuestRules) { r.Templates[0].Action = 0 }, wantErr: "action"},
		{name: "an unknown reward", mutate: func(r *domain.QuestRules) { r.Templates[0].Reward = 0 }, wantErr: "reward"},
		{name: "a filter on a quest that counts boosters", mutate: func(r *domain.QuestRules) { r.Templates[0].FilterBy = domain.QuestFilterColour }, wantErr: "filter"},
		{name: "an unknown filter", mutate: func(r *domain.QuestRules) {
			r.Templates[0].Action, r.Templates[0].FilterBy = domain.QuestSellCards, "size"
		}, wantErr: "filter"},
		{name: "a card reward with no value cap", mutate: func(r *domain.QuestRules) { r.Templates[0].MaxValue = 0 }, wantErr: "max_value"},
		{name: "daily quests that finish daily quests", mutate: func(r *domain.QuestRules) { r.Templates[0].Action = domain.QuestCompleteDailies }, wantErr: "complete_dailies"},
		{name: "a weekly set unlock is allowed", mutate: func(r *domain.QuestRules) {
			r.Templates[0].Period, r.Templates[0].Reward = domain.QuestWeekly, domain.RewardUnlockSet
			r.DailyCount, r.WeeklyCount = 0, 1
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rules := domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{validTemplate()}}
			tc.mutate(&rules)
			err := rules.Validate()
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("Validate() = %v, want nil", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Errorf("Validate() = %v, want an error naming %q", err, tc.wantErr)
			}
		})
	}
}
