package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func redCommon() domain.Card {
	return domain.Card{ID: "R-1", Rarity: domain.RarityCommon, Colors: []string{"R"}}
}

func blueRare() domain.Card {
	return domain.Card{ID: "U-1", Rarity: domain.RarityRare, Colors: []string{"U"}}
}

func TestQuestObserve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		quest        domain.Quest
		events       domain.QuestEvents
		wantProgress int64
		wantReached  bool
	}{
		{
			name:         "boosters of the named set count and those of another set do not",
			quest:        domain.Quest{Action: domain.QuestOpenBoosters, Set: "MIR", Target: 3},
			events:       domain.QuestEvents{Opened: map[string]int{"MIR": 2, "TSP": 5}},
			wantProgress: 2,
		},
		{
			name:         "boosters reach the target",
			quest:        domain.Quest{Action: domain.QuestOpenBoosters, Set: "MIR", Target: 2, Progress: 1},
			events:       domain.QuestEvents{Opened: map[string]int{"MIR": 1}},
			wantProgress: 2,
			wantReached:  true,
		},
		{
			name:         "progress stops at the target",
			quest:        domain.Quest{Action: domain.QuestOpenBoosters, Set: "MIR", Target: 2},
			events:       domain.QuestEvents{Opened: map[string]int{"MIR": 9}},
			wantProgress: 2,
			wantReached:  true,
		},
		{
			name:  "sold cards count by colour",
			quest: domain.Quest{Action: domain.QuestSellCards, Filter: domain.CardFilter{Colour: domain.ColourRed}, Target: 4},
			events: domain.QuestEvents{Sold: []domain.QuestCards{
				{Card: redCommon(), Qty: 3}, {Card: blueRare(), Qty: 5},
			}},
			wantProgress: 3,
		},
		{
			name:  "sold cards count by rarity",
			quest: domain.Quest{Action: domain.QuestSellCards, Filter: domain.CardFilter{Rarity: domain.RarityRare}, Target: 2},
			events: domain.QuestEvents{Sold: []domain.QuestCards{
				{Card: redCommon(), Qty: 3}, {Card: blueRare(), Qty: 2},
			}},
			wantProgress: 2,
			wantReached:  true,
		},
		{
			name:         "a sell quest with no filter counts every card",
			quest:        domain.Quest{Action: domain.QuestSellCards, Target: 9},
			events:       domain.QuestEvents{Sold: []domain.QuestCards{{Card: redCommon(), Qty: 3}, {Card: blueRare(), Qty: 2}}},
			wantProgress: 5,
		},
		{
			name:         "a sell quest ignores the cards bought",
			quest:        domain.Quest{Action: domain.QuestSellCards, Target: 2},
			events:       domain.QuestEvents{Bought: []domain.QuestCards{{Card: redCommon(), Qty: 3}}},
			wantProgress: 0,
		},
		{
			name:         "bought cards count by rarity",
			quest:        domain.Quest{Action: domain.QuestBuyCards, Filter: domain.CardFilter{Rarity: domain.RarityCommon}, Target: 2},
			events:       domain.QuestEvents{Bought: []domain.QuestCards{{Card: redCommon(), Qty: 2}, {Card: blueRare(), Qty: 1}}},
			wantProgress: 2,
			wantReached:  true,
		},
		{
			name:         "a trade quest keeps the best deal, not the sum",
			quest:        domain.Quest{Action: domain.QuestWinTrade, Target: 200},
			events:       domain.QuestEvents{Deals: []domain.Coins{120, 150}},
			wantProgress: 150,
		},
		{
			name:         "a deal of the target or more completes a trade quest",
			quest:        domain.Quest{Action: domain.QuestWinTrade, Target: 200, Progress: 150},
			events:       domain.QuestEvents{Deals: []domain.Coins{300}},
			wantProgress: 200,
			wantReached:  true,
		},
		{
			name:         "coins spent add up",
			quest:        domain.Quest{Action: domain.QuestSpendCoins, Target: 500, Progress: 100},
			events:       domain.QuestEvents{CoinsSpent: 250, CoinsWon: 900},
			wantProgress: 350,
		},
		{
			name:         "daily quests finished count toward a weekly one",
			quest:        domain.Quest{Action: domain.QuestCompleteDailies, Target: 5, Progress: 3},
			events:       domain.QuestEvents{DailiesDone: 2, CoinsWon: 900},
			wantProgress: 5,
			wantReached:  true,
		},
		{
			name:         "coins won add up",
			quest:        domain.Quest{Action: domain.QuestWinCoins, Target: 500, Progress: 100},
			events:       domain.QuestEvents{CoinsSpent: 900, CoinsWon: 400},
			wantProgress: 500,
			wantReached:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := tc.quest
			q.Observe(tc.events)
			if q.Progress != tc.wantProgress {
				t.Errorf("Progress = %d, want %d", q.Progress, tc.wantProgress)
			}
			if got := q.Reached(); got != tc.wantReached {
				t.Errorf("Reached() = %t, want %t", got, tc.wantReached)
			}
		})
	}
}

func TestCardFilterMatches(t *testing.T) {
	t.Parallel()
	gold := domain.Card{ID: "G-1", Rarity: domain.RarityUncommon, Colors: []string{"R", "G"}}
	tests := []struct {
		name   string
		filter domain.CardFilter
		card   domain.Card
		want   bool
	}{
		{"the zero filter takes any card", domain.CardFilter{}, blueRare(), true},
		{"a colour takes its cards", domain.CardFilter{Colour: domain.ColourRed}, redCommon(), true},
		{"a colour leaves the others out", domain.CardFilter{Colour: domain.ColourRed}, blueRare(), false},
		{"gold is its own colour group", domain.CardFilter{Colour: domain.ColourGold}, gold, true},
		{"a rarity takes its cards", domain.CardFilter{Rarity: domain.RarityRare}, blueRare(), true},
		{"a rarity leaves the others out", domain.CardFilter{Rarity: domain.RarityRare}, redCommon(), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.filter.Matches(tc.card); got != tc.want {
				t.Errorf("%+v.Matches(%s) = %t, want %t", tc.filter, tc.card.ID, got, tc.want)
			}
		})
	}
}

func TestQuestNamesRoundTrip(t *testing.T) {
	t.Parallel()
	for _, a := range []domain.QuestAction{
		domain.QuestOpenBoosters, domain.QuestSellCards, domain.QuestBuyCards,
		domain.QuestWinTrade, domain.QuestSpendCoins, domain.QuestWinCoins, domain.QuestCompleteDailies,
	} {
		got, err := domain.ParseQuestAction(a.String())
		if err != nil || got != a {
			t.Errorf("ParseQuestAction(%q) = %v, %v, want %v", a.String(), got, err, a)
		}
	}
	for _, r := range []domain.QuestRewardKind{
		domain.RewardWinCard, domain.RewardFoilCard, domain.RewardPremiumBooster,
		domain.RewardUnlockSet, domain.RewardDoubleDuplicates, domain.RewardSwap,
	} {
		got, err := domain.ParseQuestRewardKind(r.String())
		if err != nil || got != r {
			t.Errorf("ParseQuestRewardKind(%q) = %v, %v, want %v", r.String(), got, err, r)
		}
	}
	for _, p := range []domain.QuestPeriod{domain.QuestDaily, domain.QuestWeekly} {
		got, err := domain.ParseQuestPeriod(p.String())
		if err != nil || got != p {
			t.Errorf("ParseQuestPeriod(%q) = %v, %v, want %v", p.String(), got, err, p)
		}
	}
	if _, err := domain.ParseQuestAction("dance"); err == nil {
		t.Error("ParseQuestAction(dance) = nil error, want one")
	}
}
