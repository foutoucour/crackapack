package domain

import "fmt"

// QuestFilterKind is what a sell or a buy quest draws its filter by.
type QuestFilterKind string

// The kinds of filter. The empty kind counts every card.
const (
	QuestFilterNone   QuestFilterKind = ""
	QuestFilterColour QuestFilterKind = "colour"
	QuestFilterRarity QuestFilterKind = "rarity"
)

// QuestTemplate is a kind of quest that a board can draw: an action with its target and the reward it
// gives. The board names the set, the cards and the filter when it draws one.
type QuestTemplate struct {
	Period QuestPeriod
	Action QuestAction
	// Target is what the quest asks for, see [Quest.Target].
	Target int64
	Reward QuestRewardKind
	// FilterBy is what a sell or a buy quest's filter is drawn by.
	FilterBy QuestFilterKind
	// MaxValue caps, in coins, the reference value of the card a win, foil or swap reward names.
	MaxValue Coins
	// Weight is the odds of the template against the others of its period, at least 1.
	Weight int
}

// QuestRules is how the quests of a game are drawn: how many of each period, from which templates.
type QuestRules struct {
	// DailyCount and WeeklyCount are the quests on the board at once. Zero draws none of the period.
	DailyCount, WeeklyCount int
	Templates               []QuestTemplate
}

// Validate checks the rules and returns an error naming the first template or key that is out of
// range.
func (r QuestRules) Validate() error {
	counts := map[QuestPeriod]int{QuestDaily: r.DailyCount, QuestWeekly: r.WeeklyCount}
	if r.DailyCount < 0 || r.WeeklyCount < 0 {
		return fmt.Errorf("quests: daily_count and weekly_count must be >= 0, got %d and %d", r.DailyCount, r.WeeklyCount)
	}
	drawable := make(map[QuestPeriod]bool)
	for i, t := range r.Templates {
		if err := t.validate(); err != nil {
			return fmt.Errorf("quests: template %d: %w", i, err)
		}
		drawable[t.Period] = true
	}
	for _, p := range []QuestPeriod{QuestDaily, QuestWeekly} {
		if counts[p] > 0 && !drawable[p] {
			return fmt.Errorf("quests: %s quests are asked for and no %s template draws one", p, p)
		}
	}
	return nil
}

func (t QuestTemplate) validate() error {
	switch {
	case questPeriodNames()[t.Period] == "":
		return fmt.Errorf("period %d is unknown", t.Period)
	case questActionNames()[t.Action] == "":
		return fmt.Errorf("action %d is unknown", t.Action)
	case questRewardNames()[t.Reward] == "":
		return fmt.Errorf("reward %d is unknown", t.Reward)
	case t.Target < 1:
		return fmt.Errorf("target must be >= 1, got %d", t.Target)
	case t.Weight < 1:
		return fmt.Errorf("weight must be >= 1, got %d", t.Weight)
	case t.Action == QuestCompleteDailies && t.Period != QuestWeekly:
		return fmt.Errorf("%s is a weekly quest's action", t.Action)
	case t.FilterBy != QuestFilterNone && t.Action != QuestSellCards && t.Action != QuestBuyCards:
		return fmt.Errorf("a filter belongs to a %s or a %s quest, not to %s", QuestSellCards, QuestBuyCards, t.Action)
	case t.FilterBy != QuestFilterNone && t.FilterBy != QuestFilterColour && t.FilterBy != QuestFilterRarity:
		return fmt.Errorf("filter %q is unknown: it is colour or rarity", t.FilterBy)
	case t.needsValueCap() && t.MaxValue < 1:
		return fmt.Errorf("max_value must be >= 1 for a %s reward, got %d", t.Reward, t.MaxValue)
	}
	return nil
}

// needsValueCap reports whether the reward picks a card by its value.
func (t QuestTemplate) needsValueCap() bool {
	return t.Reward == RewardWinCard || t.Reward == RewardFoilCard || t.Reward == RewardSwap
}
