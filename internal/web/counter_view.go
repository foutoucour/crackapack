package web

import (
	"fmt"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// counterQuoter is what a binder page needs of the Counter. A nil one is a game without a Counter.
type counterQuoter interface {
	CounterQuote(id domain.CardID, finish domain.Finish) (play.Quote, error)
}

// cardRef is what a form of the Counter names: a card in a finish, and the binder to come back to,
// with the filter that was in force there.
type cardRef struct {
	Set    string
	ID     domain.CardID
	Finish string
	Filter binderFilter
}

// saleView is the sale of one finish of a held card.
type saleView struct {
	cardRef
	Foil   bool
	Copies int
	// Price is what the Counter pays for one copy, and Traded is false when it pays nothing.
	Price  domain.Coins
	Traded bool
}

// buyView is what the Counter offers for an empty slot: a copy to buy, or the reason it has none.
type buyView struct {
	cardRef
	Price domain.Coins
	// Buyable is true when the Counter has a copy and the player may buy it. Note then says why
	// not, and is empty when the slot has nothing to say, as in a game without a Counter.
	Buyable bool
	Note    string
}

// shelf is how the Counter serves the slots of one binder.
type shelf struct {
	set    string
	filter binderFilter
	quotes counterQuoter
	// locked is true for the binder of a set that cannot be opened yet: nothing in it can be bought.
	locked bool
	// market is the market, which adds its side to the slots. It is nil in a game without one.
	market *marketIndex
	// hunted is the cards the player hunts, to mark the empty slots of them.
	hunted map[domain.CardKey]bool
}

// apply fills the Counter's side of slot, from the card c it shows.
func (sh shelf) apply(slot *slotView, c play.BinderCard) error {
	if sh.quotes == nil {
		return nil
	}
	finishes := []struct {
		finish domain.Finish
		held   int
	}{{domain.FinishNonFoil, c.Regular}, {domain.FinishFoil, c.Foil}}
	for _, f := range finishes {
		if f.held == 0 {
			continue
		}
		q, err := sh.quotes.CounterQuote(c.Card.ID, f.finish)
		if err != nil {
			return fmt.Errorf("price %q: %w", c.Card.ID, err)
		}
		slot.Sales = append(slot.Sales, saleView{
			cardRef: sh.ref(c.Card.ID, f.finish),
			Foil:    f.finish == domain.FinishFoil, Copies: f.held, Price: q.SellPrice, Traded: q.SellPrice > 0,
		})
		if sh.market != nil {
			slot.Lists = append(slot.Lists, sh.market.sale(sh.ref(c.Card.ID, f.finish), f.finish == domain.FinishFoil, q.BuyPrice))
		}
	}
	// A held card can be bought again: the player may be building a playset.
	if sh.locked {
		return nil
	}
	finish, ok := buyableFinish(c.Card)
	if !ok {
		return nil
	}
	q, err := sh.quotes.CounterQuote(c.Card.ID, finish)
	if err != nil {
		return fmt.Errorf("price %q: %w", c.Card.ID, err)
	}
	slot.Buy = buyView{cardRef: sh.ref(c.Card.ID, finish), Price: q.BuyPrice, Buyable: q.InStock && q.BuyPrice > 0, Note: buyNote(q)}
	if sh.market != nil {
		slot.Hunt = sh.market.hunt(sh.ref(c.Card.ID, finish))
	}
	if c.Regular+c.Foil == 0 {
		slot.Hunted = &huntedMark{cardRef: sh.ref(c.Card.ID, finish), On: sh.hunted[domain.CardKey{ID: c.Card.ID, Finish: finish}]}
	}
	return nil
}

func (sh shelf) ref(id domain.CardID, finish domain.Finish) cardRef {
	return cardRef{Set: sh.set, ID: id, Finish: string(finish), Filter: sh.filter}
}

// buyableFinish is the finish an empty slot offers: the regular one, or the foil of a card that has
// no regular one.
func buyableFinish(card domain.Card) (domain.Finish, bool) {
	var foil bool
	for _, f := range card.Finishes {
		switch f {
		case domain.FinishNonFoil:
			return f, true
		case domain.FinishFoil:
			foil = true
		}
	}
	return domain.FinishFoil, foil
}

// buyNote says why an empty slot cannot be bought: never how many copies the Counter has.
func buyNote(q play.Quote) string {
	switch {
	case q.BuyPrice == 0:
		return "Not traded"
	case q.InStock:
		return ""
	case q.LastInStock.IsZero():
		return "Out of stock, never seen at the Counter"
	default:
		return "Out of stock, last in stock " + humanAgo(q.LastInStockAgo)
	}
}

// humanAgo says how long ago something was, rounded down to the largest whole unit: minutes, hours
// or days, and "less than a minute ago" before the first.
func humanAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "less than a minute ago"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute") + " ago"
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour") + " ago"
	default:
		return plural(int(d/(24*time.Hour)), "day") + " ago"
	}
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// coinSymbol stands for the currency the way $ does for dollars. It is a text character so that it
// works in the pages and in the messages built here alike, where an SVG could not go.
const coinSymbol = "◎"

// coinsText says an amount of coins, symbol first: "◎1", "◎60".
func coinsText(c domain.Coins) string {
	return fmt.Sprintf("%s%d", coinSymbol, c)
}

// duplicatesView is the bulk sale of a binder's duplicates.
type duplicatesView struct {
	// Offered is false in a game without a Counter, and when the Counter does not trade a duplicate.
	Offered bool
	Set     string
	Filter  binderFilter
	Copies  int
	Value   domain.Coins
}

// noticeView is what the last purchase or sale tells, on the binder it comes back to.
type noticeView struct {
	Text     string
	Unlocked []play.UnlockedSet
}

// confirmView is the page that asks before a sale empties a slot.
type confirmView struct {
	cardRef
	Name     string
	Foil     bool
	Qty      int
	Total    domain.Coins
	BackHref string
}
