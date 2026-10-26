package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// questsPath is the page of the daily and weekly quests.
const questsPath = "/quests"

const (
	minutesPerHour = 60
	minutesPerDay  = 24 * minutesPerHour
)

// questsPageView is the page of the quests.
type questsPageView struct {
	Daily, Weekly []questView
	// DailyIn and WeeklyIn say how long is left before the next quests come.
	DailyIn, WeeklyIn string
}

// questView is one quest in words: what to do, what it gives, how far the player is and how it ended.
type questView struct {
	Task, Prize      string
	Progress, Target int64
	// State is "open", "done" or "lost", the class of its tag, and Label the tag's text.
	State, Label string
	// Link is the card that an open quest gives, nil when it gives none, and Got the cards that a
	// done quest added to the collection, each a link to its slot in its binder.
	Link *cardLink
	Got  []cardLink
}

// cardLink is a card's name that links to its slot in the binder of its set, with the icon of the set.
type cardLink struct {
	Name, Href string
	Icon       iconView
}

func newCardLink(card domain.Card, finish domain.Finish, set string, setOf setFinder) cardLink {
	cardSet, _ := setOf(card.ID)
	return cardLink{Name: cardName(card, finish), Href: focusHref(set, card.ID), Icon: newIconView(cardSet.IconCode, string(card.Rarity))}
}

// questDoneNotice tells the player of a quest just done, with links to the cards it gave.
type questDoneNotice struct {
	Text  string
	Cards []cardLink
}

func newQuestDoneNotice(q play.QuestView, setOf setFinder) questDoneNotice {
	return questDoneNotice{
		Text:  fmt.Sprintf("Quest done: %s. Reward: %s.", questTask(q), questPrize(q.Reward)),
		Cards: gotLinks(q.Reward.Got, setOf),
	}
}

func gotLinks(lines []play.CardLine, setOf setFinder) []cardLink {
	var links []cardLink
	for _, line := range lines {
		links = append(links, newCardLink(line.Card, line.Finish, line.SetCode, setOf))
	}
	return links
}

// takeFinishedQuests is the notices of the quests done since the last page, each told once.
func (h *handler) takeFinishedQuests() []questDoneNotice {
	var notices []questDoneNotice
	for _, q := range h.game.TakeFinishedQuests() {
		notices = append(notices, newQuestDoneNotice(q, h.game.SetOf))
	}
	return notices
}

func newQuestsPageView(b play.QuestBoard, setOf setFinder) questsPageView {
	return questsPageView{
		Daily: newQuestViews(b.Daily, setOf), Weekly: newQuestViews(b.Weekly, setOf),
		DailyIn: untilText(b.DailyResets.Sub(b.Now)), WeeklyIn: untilText(b.WeeklyResets.Sub(b.Now)),
	}
}

func newQuestViews(quests []play.QuestView, setOf setFinder) []questView {
	out := make([]questView, len(quests))
	for i, q := range quests {
		out[i] = questView{Task: questTask(q), Prize: questPrize(q.Reward), Progress: q.Progress, Target: q.Target}
		switch q.State {
		case play.QuestDone:
			out[i].State, out[i].Label = "done", "Done"
			out[i].Got = gotLinks(q.Reward.Got, setOf)
		case play.QuestLost:
			out[i].State, out[i].Label = "lost", "Lost"
		case play.QuestOpen:
			out[i].State, out[i].Label = "open", "In progress"
			if q.Reward.CardSet != "" {
				link := newCardLink(q.Reward.Card, q.Reward.Finish, q.Reward.CardSet, setOf)
				out[i].Link = &link
			}
		}
	}
	return out
}

// untilText says how long is left the way a player counts it: days, hours and minutes, each left
// out at zero, the minutes rounded up, and "less than a minute" before the first.
func untilText(d time.Duration) string {
	if d < time.Minute {
		return "less than a minute"
	}
	minutes := int((d + time.Minute - 1) / time.Minute)
	days, hours, mins := minutes/minutesPerDay, minutes%minutesPerDay/minutesPerHour, minutes%minutesPerHour
	var parts []string
	for _, unit := range []struct {
		n    int
		name string
	}{{days, "day"}, {hours, "hour"}, {mins, "min"}} {
		if unit.n > 0 {
			parts = append(parts, plural(unit.n, unit.name))
		}
	}
	return strings.Join(parts, " ")
}

// questTask is what a quest asks, as a sentence.
func questTask(q play.QuestView) string {
	switch q.Action {
	case domain.QuestOpenBoosters:
		return fmt.Sprintf("Open %s of %s", count(q.Target, "booster"), q.SetName)
	case domain.QuestSellCards:
		return fmt.Sprintf("Sell or trade %s", cardsOf(q.Target, q.Filter))
	case domain.QuestBuyCards:
		return fmt.Sprintf("Buy or trade for %s", cardsOf(q.Target, q.Filter))
	case domain.QuestWinTrade:
		return fmt.Sprintf("Win a deal on the market worth %s or more", coinsText(domain.Coins(q.Target)))
	case domain.QuestSpendCoins:
		return "Spend " + coinsText(domain.Coins(q.Target))
	case domain.QuestWinCoins:
		return "Win " + coinsText(domain.Coins(q.Target))
	case domain.QuestCompleteDailies:
		return fmt.Sprintf("Finish %s", count(q.Target, "daily quest"))
	}
	return ""
}

// questPrize is what a quest gives, as a sentence.
func questPrize(r play.RewardView) string {
	switch r.Kind {
	case domain.RewardWinCard:
		return "Win " + cardName(r.Card, r.Finish)
	case domain.RewardFoilCard:
		return fmt.Sprintf("Turn your %s into a foil", r.Card.Name)
	case domain.RewardPremiumBooster:
		return fmt.Sprintf("A premium booster of %s: it holds no card you have a playset of", r.SetName)
	case domain.RewardUnlockSet:
		return "Unlock " + r.SetName
	case domain.RewardDoubleDuplicates:
		return fmt.Sprintf("Double your spare copies of %s, the copies above a playset", r.SetName)
	case domain.RewardSwap:
		give := make([]string, len(r.Give))
		for i, line := range r.Give {
			give[i] = fmt.Sprintf("%d × %s", line.Qty, cardName(line.Card, line.Finish))
		}
		return fmt.Sprintf("Swap %s, your spare copies, for %s", strings.Join(give, " and "), cardName(r.Card, r.Finish))
	}
	return ""
}

// cardsOf is "6 red cards", "4 rare cards" or "6 cards", for a filter.
func cardsOf(n int64, f domain.CardFilter) string {
	adjective := string(f.Rarity)
	if f.Colour != "" {
		adjective = colourWord(f.Colour)
	}
	if adjective == "" {
		return count(n, "card")
	}
	return count(n, adjective+" card")
}

// colourWord is the name of a colour group.
func colourWord(c domain.ColourGroup) string {
	switch c {
	case domain.ColourWhite:
		return "white"
	case domain.ColourBlue:
		return "blue"
	case domain.ColourBlack:
		return "black"
	case domain.ColourRed:
		return "red"
	case domain.ColourGreen:
		return "green"
	case domain.ColourGold:
		return "gold"
	case domain.ColourColourless:
		return "colourless"
	case domain.ColourLand:
		return "land"
	}
	return string(c)
}

// count is "1 card" or "2 cards".
func count(n int64, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// questsNavView is the header's link to the quests. It has no field: the link is there or it is not.
type questsNavView struct{}
