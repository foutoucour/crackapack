package play

import (
	"errors"
	"fmt"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

var (
	// ErrUnknownCard is returned for a card, or a finish of it, that no catalogue of the game holds.
	ErrUnknownCard = errors.New("play: unknown card")
	// ErrNotEnoughCoins is returned by Buy when the player cannot pay the price.
	ErrNotEnoughCoins = errors.New("play: not enough coins")
	// ErrOutOfStock is returned by Buy when the Counter has no copy of the card.
	ErrOutOfStock = errors.New("play: the Counter has no copy of the card")
	// ErrNothingToSell is returned by Sell when the player does not hold the copies, and by
	// SellDuplicates when no card of the set is held beyond a playset.
	ErrNothingToSell = errors.New("play: nothing to sell")
	// ErrCounterBroke is returned by a sale that the Counter's treasury cannot pay.
	ErrCounterBroke = errors.New("play: the Counter cannot pay for the cards")
	// ErrNotTraded is returned by a purchase or a sale of a card worth less than one coin, which
	// only a card priced at zero is: its quote has a price of zero.
	ErrNotTraded = errors.New("play: the Counter does not trade a card worth nothing")
	// ErrNoCounter is returned by every Counter action of a game without one, see Game.OpenCounter.
	ErrNoCounter = errors.New("play: the game has no Counter")
)

// Quote is what the Counter pays for one copy of a card, what one costs, and whether it has a copy:
// never how many.
type Quote struct {
	// BuyPrice is what the player pays for a copy, and SellPrice what the player is paid for one. A
	// price of zero is a trade the Counter refuses, which only a card priced at zero gives.
	BuyPrice  domain.Coins
	SellPrice domain.Coins
	InStock   bool
	// LastInStock is, for a card out of stock, the last instant the Counter had a copy, and the zero
	// time when it never had one.
	LastInStock time.Time
}

// Purchase is what a card bought at the Counter cost, and the sets it unlocked by completing a
// binder, as an opening does.
type Purchase struct {
	Paid     domain.Coins
	Unlocked []UnlockedSet
}

// Balance is the player's coins, and zero in a game without a Counter.
func (g *Game) Balance() domain.Coins {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.economy == nil {
		return 0
	}
	return g.economy.journal.Balance(domain.AccountPlayer)
}

// CounterQuote returns the prices of a copy of the card id in finish at the Counter, and whether
// it has one. It returns ErrUnknownCard, wrapped, for a card the game does not hold, and
// ErrNoCounter in a game without a Counter.
func (g *Game) CounterQuote(id domain.CardID, finish domain.Finish) (Quote, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, key, err := g.counterFor(id, finish)
	if err != nil {
		return Quote{}, fmt.Errorf("quote %q: %w", id, err)
	}
	prices, err := e.counter.Quote(key)
	if err != nil {
		return Quote{}, fmt.Errorf("quote %q: %w", id, err)
	}
	q := Quote{BuyPrice: prices.Asks, SellPrice: prices.Pays, InStock: e.counter.Stock(key) > 0}
	if !q.InStock {
		q.LastInStock, _ = e.counter.LastInStock(key)
	}
	return q, nil
}

// Buy buys one copy of the card id in finish from the Counter and adds it to the collection. It
// returns ErrUnknownCard, ErrLocked when no unlocked set's binder holds the card, ErrOutOfStock,
// ErrNotEnoughCoins, ErrNotTraded or ErrNoCounter, wrapped, when it cannot, and then changes nothing.
func (g *Game) Buy(id domain.CardID, finish domain.Finish) (Purchase, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, key, err := g.counterFor(id, finish)
	if err != nil {
		return Purchase{}, fmt.Errorf("buy %q: %w", id, err)
	}
	if !g.buyable(id) {
		return Purchase{}, fmt.Errorf("buy %q: %w", id, ErrLocked)
	}
	before := e.journal.Balance(domain.AccountPlayer)
	if err := e.counter.SellToPlayer(key); err != nil {
		switch {
		case errors.Is(err, domain.ErrOutOfStock):
			err = ErrOutOfStock
		case errors.Is(err, domain.ErrInsufficientBalance):
			err = ErrNotEnoughCoins
		case errors.Is(err, domain.ErrSubCoin):
			err = ErrNotTraded
		}
		return Purchase{}, fmt.Errorf("buy %q: %w", id, err)
	}
	// Add cannot fail for a positive quantity.
	_ = g.collection.Add(key, 1)
	p := Purchase{Paid: before - e.journal.Balance(domain.AccountPlayer), Unlocked: g.unlockEarned()}
	g.save()
	return p, nil
}

// Sell sells qty copies of the card id in finish to the Counter, the only copy included, and
// returns the coins paid. It returns ErrUnknownCard, ErrNothingToSell when the player holds fewer
// than qty copies or qty is not positive, ErrCounterBroke, ErrNotTraded or ErrNoCounter, wrapped,
// and then changes nothing.
// A sale never locks a set again: an unlock is permanent.
func (g *Game) Sell(id domain.CardID, finish domain.Finish, qty int) (domain.Coins, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, key, err := g.counterFor(id, finish)
	if err != nil {
		return 0, fmt.Errorf("sell %q: %w", id, err)
	}
	if have := g.collection.Quantity(key); qty < 1 || qty > have {
		return 0, fmt.Errorf("sell %d of %q, the player holds %d: %w", qty, id, have, ErrNothingToSell)
	}
	paid, err := g.sellLot([]domain.TradeLine{{Key: key, Qty: qty}})
	if err != nil {
		return 0, fmt.Errorf("sell %q: %w", id, err)
	}
	return paid, nil
}

// SellDuplicates sells the copies above a playset of every card of the binder of setCode, keeping
// the foils longest, and returns the coins paid. It returns ErrUnknownSet, ErrNothingToSell,
// ErrCounterBroke, ErrNotTraded or ErrNoCounter, wrapped, and then changes nothing.
func (g *Game) SellDuplicates(setCode string) (domain.Coins, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	binder, ok := g.binders[setCode]
	if !ok {
		return 0, fmt.Errorf("sell the duplicates of %q: %w", setCode, ErrUnknownSet)
	}
	if err := g.syncCounter(); err != nil {
		return 0, err
	}
	surplus, err := g.collection.DuplicatesAbove(domain.PlaysetSize)
	if err != nil {
		return 0, fmt.Errorf("sell the duplicates of %q: %w", setCode, err)
	}
	var lines []domain.TradeLine
	for _, s := range surplus {
		if _, inBinder := binder.Cards[s.Key.ID]; inBinder {
			lines = append(lines, domain.TradeLine{Key: s.Key, Qty: s.Quantity})
		}
	}
	if len(lines) == 0 {
		return 0, fmt.Errorf("sell the duplicates of %q: %w", setCode, ErrNothingToSell)
	}
	paid, err := g.sellLot(lines)
	if err != nil {
		return 0, fmt.Errorf("sell the duplicates of %q: %w", setCode, err)
	}
	return paid, nil
}

// sellLot sells lines, which the player holds, to the Counter in one payment, and saves. g.mu must
// be held, and the Counter caught up.
func (g *Game) sellLot(lines []domain.TradeLine) (domain.Coins, error) {
	paid, err := g.economy.counter.BuyLotFromPlayer(lines)
	switch {
	case errors.Is(err, domain.ErrCounterBroke):
		return 0, ErrCounterBroke
	case errors.Is(err, domain.ErrSubCoin):
		return 0, ErrNotTraded
	case err != nil:
		return 0, err
	}
	for _, l := range lines {
		// Remove cannot fail: the caller checked that the player holds the copies.
		_ = g.collection.Remove(l.Key, l.Qty)
	}
	g.save()
	return paid, nil
}

// counterFor returns the game's Counter, caught up, and the key of the card id in finish. It
// returns ErrUnknownCard for a card or a finish no catalogue of the game holds. g.mu must be held.
func (g *Game) counterFor(id domain.CardID, finish domain.Finish) (*economy, domain.CardKey, error) {
	key := domain.CardKey{ID: id, Finish: finish}
	card, ok := g.cards[id]
	if !ok || !hasFinish(card, finish) {
		return nil, key, ErrUnknownCard
	}
	if err := g.syncCounter(); err != nil {
		return nil, key, err
	}
	return g.economy, key, nil
}

// syncCounter catches the Counter up with the days gone by. g.mu must be held.
func (g *Game) syncCounter() error {
	if g.economy == nil {
		return ErrNoCounter
	}
	return g.economy.catchUp()
}

// buyable reports whether a binder of an unlocked set holds the card. g.mu must be held.
func (g *Game) buyable(id domain.CardID) bool {
	for _, code := range g.binderSets[id] {
		if !g.locked(code) {
			return true
		}
	}
	return false
}

func hasFinish(card domain.Card, finish domain.Finish) bool {
	for _, f := range card.Finishes {
		if f == finish {
			return true
		}
	}
	return false
}
