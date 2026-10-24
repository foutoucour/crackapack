package play_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func TestQuests_theDailyRewardIsNotCoinsWon(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	counter := counterRules()
	counter.DailyReward = 100
	win := template(domain.QuestDaily, domain.QuestWinCoins, 50, domain.RewardPremiumBooster)
	g := questGameOn(t, &memStore{}, clock, counter, dailyOnly(win), questSeed, counterSet())

	clock.Advance(day)
	if _, err := g.CounterQuote(cheap().ID, cheap().Finish); err != nil { // catches the Counter up, and pays the reward
		t.Fatalf("CounterQuote: %v", err)
	}

	if q := firstQuest(t, g); q.Progress != 0 {
		t.Errorf("progress = %d after the daily reward, want 0: it is paid for logging in, not won by playing", q.Progress)
	}
}
