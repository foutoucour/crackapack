package play

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// secondsPerDay is the length of a day of quests, which renews at midnight UTC.
const secondsPerDay = 24 * 60 * 60

// daysPerWeek is the days of a weekly quest, which renews on Monday at midnight UTC.
const daysPerWeek = 7

// mondayOffset shifts the day count from the Unix epoch, a Thursday, to a week that starts on Monday.
const mondayOffset = 3

// ErrNoQuests is returned by Quests in a game without quests, see OpenQuests.
var ErrNoQuests = errors.New("play: the game has no quests")

// QuestState is where a quest stands.
type QuestState int

// The states of a quest.
const (
	// QuestOpen is a quest the player is still doing.
	QuestOpen QuestState = iota + 1
	// QuestDone is a quest that was done and whose reward was given.
	QuestDone
	// QuestLost is a quest that was done when its reward could no longer be given: the copies a swap
	// takes were gone, or the card to turn into a foil was.
	QuestLost
)

// QuestBoard is the quests the player sees: the daily ones until the next midnight UTC and the weekly
// ones until the next Monday.
type QuestBoard struct {
	Daily, Weekly             []QuestView
	DailyResets, WeeklyResets time.Time
}

// QuestView is one quest as the player reads it: the action, how far the player is, and the reward.
type QuestView struct {
	Period domain.QuestPeriod
	Action domain.QuestAction
	// SetCode and SetName are the set of a quest that asks for boosters.
	SetCode, SetName string
	// Filter is what a quest that asks for cards counts.
	Filter           domain.CardFilter
	Target, Progress int64
	State            QuestState
	Reward           RewardView
}

// RewardView is what a quest gives, with the names resolved.
type RewardView struct {
	Kind domain.QuestRewardKind
	// Card and Finish are the card won or turned into a foil, and the card a swap gives the player.
	Card   domain.Card
	Finish domain.Finish
	// SetCode and SetName are the set whose booster, unlock or duplicates the reward is.
	SetCode, SetName string
	// Give is the copies a swap takes.
	Give []CardLine
	// Got is the cards a done quest added to the collection, a premium booster's included.
	Got []CardLine
}

// quest is a quest as the game keeps it.
type quest struct {
	domain.Quest
	state QuestState
	// got is the cards that the reward added, once the quest is done.
	got []domain.CardKey
}

// questBoard is the quests of a game and what it takes to count them: the collection and the ledger as
// they stood at the last count, and the boosters opened since. Every method needs the lock of its
// Game.
type questBoard struct {
	rules  domain.QuestRules
	stream *Stream
	rng    *rand.Rand
	// day and week are the indices of the day and the week the quests were drawn for.
	day, week int64
	daily     []*quest
	weekly    []*quest
	// baseline is the collection at the last count, so that what changed since is what the player
	// sold or bought.
	baseline map[domain.CardKey]int
	// counted is the entries of the ledger already counted.
	counted int
	// opened is the boosters opened since the last count, by set code.
	opened map[string]int
}

// OpenQuests gives the game quests drawn under rules, a board of its own on the random stream given,
// which the save keeps. It is called once, after OpenCounter and Stage and before Persist. A game
// without OpenQuests has no quests. It returns ErrNoCounter, wrapped, in a game without a Counter,
// whose prices value the cards of the rewards, and an error for rules out of range.
func (g *Game) OpenQuests(rules domain.QuestRules, stream *Stream) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.economy == nil {
		return fmt.Errorf("play: quests need coins: %w", ErrNoCounter)
	}
	if err := rules.Validate(); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	g.quests = &questBoard{rules: rules, stream: stream, rng: rand.New(stream), opened: make(map[string]int)}
	g.quests.rebase(g)
	g.renewQuests()
	return nil
}

// Quests is the board, after the quests are renewed for the time it is and counted for what the
// player did. It returns ErrNoQuests in a game without quests.
func (g *Game) Quests() (QuestBoard, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.quests == nil {
		return QuestBoard{}, ErrNoQuests
	}
	if g.settleQuests() {
		g.save()
	}
	b := g.quests
	return QuestBoard{
		Daily: g.questViews(b.daily), Weekly: g.questViews(b.weekly),
		DailyResets:  time.Unix((b.day+1)*secondsPerDay, 0).UTC(),
		WeeklyResets: time.Unix(((b.week+1)*daysPerWeek-mondayOffset)*secondsPerDay, 0).UTC(),
	}, nil
}

// settleQuests renews the quests for the time it is, and counts what the player did since the last
// count: it finishes the quests that are done and gives their rewards. It reports whether anything
// changed, which the caller saves. g.mu must be held. Every save settles first, so that the quests
// follow every method that changes the game.
func (g *Game) settleQuests() bool {
	b := g.quests
	if b == nil {
		return false
	}
	changed := g.renewQuests()
	events := g.questEvents()
	doneDaily, changedDaily := g.countQuests(b.daily, events)
	events.DailiesDone = doneDaily
	_, changedWeekly := g.countQuests(b.weekly, events)
	if changedDaily || changedWeekly {
		g.unlockEarned()
		b.rebase(g)
		changed = true
	}
	return changed
}

// countQuests gives events to the open quests, finishes the ones they complete, and returns how many
// were done and whether any quest changed.
func (g *Game) countQuests(quests []*quest, events domain.QuestEvents) (done int, changed bool) {
	for _, q := range quests {
		if q.state != QuestOpen {
			continue
		}
		before := q.Progress
		q.Observe(events)
		changed = changed || q.Progress != before
		if !q.Reached() {
			continue
		}
		g.finishQuest(q)
		changed = true
		if q.state == QuestDone {
			done++
		}
	}
	return done, changed
}

// renewQuests draws the daily quests when a new day has come, and the weekly ones when a new week
// has, and reports whether it drew any.
func (g *Game) renewQuests() bool {
	b := g.quests
	now := g.now().UTC().Unix()
	day := now / secondsPerDay
	week := (day + mondayOffset) / daysPerWeek
	changed := false
	if b.day != day {
		b.day, b.daily, changed = day, g.drawQuests(domain.QuestDaily, b.rules.DailyCount), true
	}
	if b.week != week {
		b.week, b.weekly, changed = week, g.drawQuests(domain.QuestWeekly, b.rules.WeeklyCount), true
	}
	return changed
}

// questEvents is what the player did since the last count: the copies that came to the collection
// and left it, the coins that moved in the ledger and the boosters opened. The copies are the
// collection's change, whichever way it came, so that a card sold at the Counter and one traded on
// the market both count, and a booster or a reward, which rebase the baseline, do not.
func (g *Game) questEvents() domain.QuestEvents {
	b := g.quests
	events := domain.QuestEvents{Opened: b.opened}
	b.opened = make(map[string]int)
	now := g.collection.Snapshot()
	for key, n := range now {
		if gained := n - b.baseline[key]; gained > 0 {
			events.Bought = append(events.Bought, domain.QuestCards{Card: g.cards[key.ID], Qty: gained})
		}
	}
	for key, n := range b.baseline {
		if lost := n - now[key]; lost > 0 {
			events.Sold = append(events.Sold, domain.QuestCards{Card: g.cards[key.ID], Qty: lost})
		}
	}
	journal := g.economy.journal
	if b.counted > journal.Len() {
		b.counted = 0
	}
	for _, e := range journal.EntriesSince(b.counted) {
		switch {
		case e.To == domain.AccountPlayer && isGift(e.From):
			// Coins paid for being there are not coins won by playing.
		case e.To == domain.AccountPlayer:
			events.CoinsWon += e.Amount
			if e.From == domain.AccountMarketSale {
				events.Deals = append(events.Deals, e.Amount)
			}
		case e.From == domain.AccountPlayer:
			events.CoinsSpent += e.Amount
		}
	}
	b.counted, b.baseline = journal.Len(), now
	return events
}

// isGift reports whether a ledger account pays the player for being there and not for playing: the
// welcome bonus and the daily reward.
func isGift(from domain.Account) bool {
	return from == domain.AccountWelcomeBonus || from == domain.AccountDailyReward
}

// rebase makes the collection and the ledger as they stand the ones that the next count compares
// with: what changed so far counts for nothing.
func (b *questBoard) rebase(g *Game) {
	b.baseline = g.collection.Snapshot()
	b.counted = g.economy.journal.Len()
}

// questOpened counts a booster opened of setCode, whose cards are in the collection already, and
// leaves them out of what the player bought. g.mu must be held.
func (g *Game) questOpened(setCode string) {
	if b := g.quests; b != nil {
		b.opened[setCode]++
		b.baseline = g.collection.Snapshot()
	}
}

// questViews resolves the names of quests for the player.
func (g *Game) questViews(quests []*quest) []QuestView {
	out := make([]QuestView, len(quests))
	for i, q := range quests {
		v := QuestView{
			Period: q.Period, Action: q.Action, Filter: q.Filter, Target: q.Target, Progress: q.Progress, State: q.state,
			Reward: RewardView{
				Kind: q.Reward.Kind, Card: g.cards[q.Reward.Card.ID], Finish: q.Reward.Card.Finish,
				Give: g.cardLines(q.Reward.Give), Got: g.cardLines(linesOf(q.got)),
			},
		}
		v.SetCode, v.SetName = q.Set, g.setName(q.Set)
		v.Reward.SetCode, v.Reward.SetName = q.Reward.Set, g.setName(q.Reward.Set)
		out[i] = v
	}
	return out
}

// setName is the name of the set code, or "" for a code that is none.
func (g *Game) setName(code string) string {
	cat, _ := g.set(code)
	return cat.Name
}

// linesOf is one copy for each key.
func linesOf(keys []domain.CardKey) []domain.TradeLine {
	lines := make([]domain.TradeLine, len(keys))
	for i, key := range keys {
		lines[i] = domain.TradeLine{Key: key, Qty: 1}
	}
	return lines
}
