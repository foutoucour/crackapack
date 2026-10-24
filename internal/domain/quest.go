package domain

import "fmt"

// QuestPeriod is how long a quest lasts: a daily one renews at midnight UTC, a weekly one on Monday.
type QuestPeriod int

// The periods of a quest.
const (
	QuestDaily QuestPeriod = iota + 1
	QuestWeekly
)

// QuestAction is what a quest asks the player to do.
type QuestAction int

// The actions a quest asks for. The first three count what the player did, the last three the coins
// that moved.
const (
	// QuestOpenBoosters asks for Target boosters of the quest's set.
	QuestOpenBoosters QuestAction = iota + 1
	// QuestSellCards asks for Target cards sold or traded away that the quest's filter takes.
	QuestSellCards
	// QuestBuyCards asks for Target cards bought or traded for that the quest's filter takes.
	QuestBuyCards
	// QuestWinTrade asks for one deal on the market that pays at least Target coins.
	QuestWinTrade
	// QuestSpendCoins asks for Target coins spent.
	QuestSpendCoins
	// QuestWinCoins asks for Target coins won.
	QuestWinCoins
)

// QuestRewardKind is what a finished quest gives the player.
type QuestRewardKind int

// The rewards of a quest.
const (
	// RewardWinCard gives one copy of the reward's card.
	RewardWinCard QuestRewardKind = iota + 1
	// RewardFoilCard turns a copy of the reward's card the player holds into a foil.
	RewardFoilCard
	// RewardPremiumBooster opens a booster of the reward's set that holds no card the player has a
	// playset of, see [LackingCatalogue].
	RewardPremiumBooster
	// RewardUnlockSet unlocks the reward's set.
	RewardUnlockSet
	// RewardDoubleDuplicates doubles the copies above a playset of the cards of the reward's set.
	RewardDoubleDuplicates
	// RewardSwap gives the card of the reward for the copies in Give, at the reference value.
	RewardSwap
)

// questActionNames, and the two tables after it, are how the kinds are written in a configuration.
// A function returns them so that no package variable holds state.
func questActionNames() map[QuestAction]string {
	return map[QuestAction]string{
		QuestOpenBoosters: "open_boosters", QuestSellCards: "sell_cards", QuestBuyCards: "buy_cards",
		QuestWinTrade: "win_trade", QuestSpendCoins: "spend_coins", QuestWinCoins: "win_coins",
	}
}

func questRewardNames() map[QuestRewardKind]string {
	return map[QuestRewardKind]string{
		RewardWinCard: "win_card", RewardFoilCard: "foil_card", RewardPremiumBooster: "premium_booster",
		RewardUnlockSet: "unlock_set", RewardDoubleDuplicates: "double_duplicates", RewardSwap: "swap",
	}
}

func questPeriodNames() map[QuestPeriod]string {
	return map[QuestPeriod]string{QuestDaily: "daily", QuestWeekly: "weekly"}
}

// String is the name of the action in a configuration, or "unknown".
func (a QuestAction) String() string { return nameOf(questActionNames(), a) }

// String is the name of the reward in a configuration, or "unknown".
func (k QuestRewardKind) String() string { return nameOf(questRewardNames(), k) }

// String is the name of the period in a configuration, or "unknown".
func (p QuestPeriod) String() string { return nameOf(questPeriodNames(), p) }

func nameOf[K comparable](names map[K]string, k K) string {
	if name, ok := names[k]; ok {
		return name
	}
	return "unknown"
}

// ParseQuestAction is the action that name stands for in a configuration.
func ParseQuestAction(name string) (QuestAction, error) {
	return parseName(questActionNames(), name, "quest action")
}

// ParseQuestRewardKind is the reward that name stands for in a configuration.
func ParseQuestRewardKind(name string) (QuestRewardKind, error) {
	return parseName(questRewardNames(), name, "quest reward")
}

// ParseQuestPeriod is the period that name stands for in a configuration.
func ParseQuestPeriod(name string) (QuestPeriod, error) {
	return parseName(questPeriodNames(), name, "quest period")
}

func parseName[K comparable](names map[K]string, name, what string) (K, error) {
	for k, n := range names {
		if n == name {
			return k, nil
		}
	}
	var zero K
	return zero, fmt.Errorf("%s %q is unknown", what, name)
}

// CardFilter picks the cards that a sell or a buy quest counts, by colour group or by rarity. The
// zero value takes every card.
type CardFilter struct {
	Colour ColourGroup
	Rarity Rarity
}

// Matches reports whether the filter takes c.
func (f CardFilter) Matches(c Card) bool {
	return (f.Colour == "" || ColourGroupOf(c) == f.Colour) && (f.Rarity == "" || c.Rarity == f.Rarity)
}

// QuestReward is what a quest gives, with the cards and the set it names.
type QuestReward struct {
	Kind QuestRewardKind
	// Card is the card won or turned into a foil, and for a swap the card the player receives.
	Card CardKey
	// Set is the set whose booster, unlock or duplicates the reward is.
	Set string
	// Give is the copies a swap takes from the player.
	Give []TradeLine
}

// QuestCards is a number of copies of one card that the player sold or bought.
type QuestCards struct {
	Card Card
	Qty  int
}

// QuestEvents is what the player did since the quests last counted.
type QuestEvents struct {
	// Opened is the boosters opened, by set code.
	Opened map[string]int
	// Sold are the cards that left the collection by a sale or a trade, Bought those that came to it
	// the same way. A booster opened and a reward are neither.
	Sold, Bought []QuestCards
	// CoinsSpent and CoinsWon are the coins that left and entered the player's wallet.
	CoinsSpent, CoinsWon Coins
	// Deals are the coins that each deal on the market paid the player.
	Deals []Coins
}

// Quest is one quest: an action to do and the reward it gives the moment it is done.
type Quest struct {
	Period QuestPeriod
	Action QuestAction
	// Set is the set of a QuestOpenBoosters quest.
	Set string
	// Filter is the cards a QuestSellCards or a QuestBuyCards quest counts.
	Filter CardFilter
	// Target is the boosters or cards asked for, or the coins.
	Target int64
	// Progress is how far the player is, never more than Target.
	Progress int64
	Reward   QuestReward
}

// Observe counts events into the quest's progress, which stops at the target.
func (q *Quest) Observe(ev QuestEvents) {
	switch q.Action {
	case QuestOpenBoosters:
		q.add(int64(ev.Opened[q.Set]))
	case QuestSellCards:
		q.add(q.countCards(ev.Sold))
	case QuestBuyCards:
		q.add(q.countCards(ev.Bought))
	case QuestWinTrade:
		for _, deal := range ev.Deals {
			q.Progress = min(max(q.Progress, int64(deal)), q.Target)
		}
	case QuestSpendCoins:
		q.add(int64(ev.CoinsSpent))
	case QuestWinCoins:
		q.add(int64(ev.CoinsWon))
	}
}

// Reached reports whether the player has done what the quest asks.
func (q Quest) Reached() bool { return q.Progress >= q.Target }

func (q *Quest) add(n int64) { q.Progress = min(q.Progress+n, q.Target) }

func (q Quest) countCards(lines []QuestCards) int64 {
	var n int64
	for _, l := range lines {
		if q.Filter.Matches(l.Card) {
			n += int64(l.Qty)
		}
	}
	return n
}
