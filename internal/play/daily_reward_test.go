package play_test

import (
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const dailyReward = domain.Coins(100)

func rewardRules() play.CounterRules {
	rules := emptyCounter()
	rules.DailyReward = dailyReward
	return rules
}

func TestDailyReward_isPaidOnceADayHasGone(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, rewardRules(), counterSet())

	tests := []struct {
		name    string
		advance time.Duration
		want    domain.Coins
	}{
		{"nothing before a whole day", day - time.Minute, 200},
		{"one reward once the day is whole", time.Minute, 300},
		{"none again within the same day", 12 * time.Hour, 300},
		{"one for each day gone by", 2*day + time.Hour, 500},
	}
	for _, tt := range tests {
		clock.Advance(tt.advance)
		if got := g.Balance(); got != tt.want {
			t.Errorf("%s: balance %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestDailyReward_noRewardWhenTheRulesHaveNone(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, emptyCounter(), counterSet())

	clock.Advance(3 * day)

	if got := g.Balance(); got != 200 {
		t.Errorf("balance %d, want the welcome bonus alone", got)
	}
}

// A long time away is rewarded for the days it catches up on only, as the Counter's supply is.
func TestDailyReward_isPaidForTheDaysCaughtUpOnOnly(t *testing.T) {
	t.Parallel()
	rules := rewardRules()
	rules.CatchUpDays = 2
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, rules, counterSet())

	clock.Advance(10 * day)

	if got := g.Balance(); got != 200+2*dailyReward {
		t.Errorf("balance after ten days capped at two = %d, want %d", got, 200+2*dailyReward)
	}
}

func TestDailyReward_isNotPaidAgainAfterAReload(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	clock := playtest.NewClock()
	g := counterGame(t, store, clock, rewardRules(), counterSet())
	clock.Advance(day)
	if got := g.Balance(); got != 300 {
		t.Fatalf("balance after a day = %d, want 300", got)
	}
	mustOpen(t, g, "S")

	loaded := counterGame(t, store, clock, rewardRules(), counterSet())

	if got := loaded.Balance(); got != 300 {
		t.Errorf("balance after a reload = %d, want 300", got)
	}
	clock.Advance(day)
	if got := loaded.Balance(); got != 400 {
		t.Errorf("balance a day after the reload = %d, want 400", got)
	}
}
