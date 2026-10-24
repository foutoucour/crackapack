package play

import (
	"errors"
	"fmt"
	"slices"
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
	// LastInStockAgo is how long before now LastInStock was, by the game's clock: zero for a card in
	// stock and for one the Counter never had.
	LastInStockAgo time.Duration
}

// Duplicates is what SellDuplicates would sell in a binder: the copies above a playset, and what
// the Counter would pay for them.
type Duplicates struct {
	Copies int
	Value  domain.Coins
}

// Purchase is what a card bought at the Counter cost, and the sets it unlocked by completing a
// binder, as an opening does.
type Purchase struct {
	Paid     domain.Coins
	Unlocked []UnlockedSet
}

// HasCounter reports whether the game has coins and a Counter, see OpenCounter.
func (g *Game) HasCounter() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.economy != nil
}

// Balance is the player's coins, with the daily rewards of the days gone by, and zero in a game
// without a Counter.
func (g *Game) Balance() domain.Coins {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.economy == nil {
		return 0
	}
	// A catch-up that fails stops before the day it cannot play, so the coins it did pay show. The
	// next action catches up again and returns the error.
	_ = g.syncCounter()
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
		if q.LastInStock, _ = e.counter.LastInStock(key); !q.LastInStock.IsZero() {
			q.LastInStockAgo = e.now().Sub(q.LastInStock)
		}
	}
	return q, nil
}

// CardValue returns the reference value in coins of the card id in finish, what the Counter's
// prices are a percentage of. It returns ErrUnknownCard, wrapped, for a card the game does not
// hold, and ErrNoCounter in a game without a Counter.
func (g *Game) CardValue(id domain.CardID, finish domain.Finish) (domain.Coins, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, key, err := g.counterFor(id, finish)
	if err != nil {
		return 0, fmt.Errorf("value %q: %w", id, err)
	}
	value, err := e.counter.Reference(key)
	if err != nil {
		return 0, fmt.Errorf("value %q: %w", id, err)
	}
	return value, nil
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
// than qty copies or qty is not positive, ErrCopyLocked when a milestone locks one of them,
// ErrReserved when the market holds some of them, ErrCounterBroke, ErrNotTraded or ErrNoCounter,
// wrapped, and then changes nothing.
// A sale never locks a set again: an unlock is permanent.
func (g *Game) Sell(id domain.CardID, finish domain.Finish, qty int) (domain.Coins, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, key, err := g.counterFor(id, finish)
	if err != nil {
		return 0, fmt.Errorf("sell %q: %w", id, err)
	}
	have := g.collection.Quantity(key)
	if qty < 1 || qty > have {
		return 0, fmt.Errorf("sell %d of %q, the player holds %d: %w", qty, id, have, ErrNothingToSell)
	}
	locked := g.collection.Locked(key)
	if qty > have-locked {
		return 0, fmt.Errorf("sell %d of %q, a milestone locks %d of %d: %w", qty, id, locked, have, ErrCopyLocked)
	}
	if held := g.reserved(key); qty > have-locked-held {
		return 0, fmt.Errorf("sell %d of %q, the market holds %d of %d: %w", qty, id, held, have, ErrReserved)
	}
	paid, err := g.sellLot([]domain.TradeLine{{Key: key, Qty: qty}})
	if err != nil {
		return 0, fmt.Errorf("sell %q: %w", id, err)
	}
	return paid, nil
}

// SellDuplicates sells the copies above a playset of every card of the binder of setCode that
// include accepts, or of every card when include is nil, keeping the foils longest and the copies
// the market holds, and returns the coins paid. It returns ErrUnknownSet, ErrNothingToSell,
// ErrCounterBroke, ErrNotTraded or ErrNoCounter, wrapped, and then changes nothing.
func (g *Game) SellDuplicates(setCode string, include func(domain.CardID) bool) (domain.Coins, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	lines, err := g.duplicateLines(setCode, include)
	if err != nil {
		return 0, fmt.Errorf("sell the duplicates of %q: %w", setCode, err)
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

// Duplicates previews SellDuplicates for setCode and include: the copies it would sell and what it
// would pay, zero of both when the binder holds nothing above a playset. It returns ErrUnknownSet,
// ErrNotTraded or ErrNoCounter, wrapped, and changes nothing.
func (g *Game) Duplicates(setCode string, include func(domain.CardID) bool) (Duplicates, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	lines, err := g.duplicateLines(setCode, include)
	if err != nil {
		return Duplicates{}, fmt.Errorf("duplicates of %q: %w", setCode, err)
	}
	var d Duplicates
	for _, l := range lines {
		prices, err := g.economy.counter.Quote(l.Key)
		if err != nil {
			return Duplicates{}, fmt.Errorf("duplicates of %q: %w", setCode, err)
		}
		if prices.Pays == 0 {
			return Duplicates{}, fmt.Errorf("duplicates of %q: %w", setCode, ErrNotTraded)
		}
		d.Copies += l.Qty
		d.Value += prices.Pays * domain.Coins(l.Qty)
	}
	return d, nil
}

// SaleEmptiesSlot reports whether selling qty copies of the card id in finish would leave the
// player holding no copy of it, in either finish. It is false for a sale that Sell would refuse.
func (g *Game) SaleEmptiesSlot(id domain.CardID, finish domain.Finish, qty int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.cards[id]; !ok {
		return false
	}
	have := g.collection.Quantity(domain.CardKey{ID: id, Finish: finish})
	return qty >= 1 && qty == have && g.collection.Copies(id) == have
}

// Card returns the card id of any catalogue of the game.
func (g *Game) Card(id domain.CardID) (domain.Card, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	card, ok := g.cards[id]
	return card, ok
}

// duplicateLines is the sale of the copies above a playset of every card of the binder of
// setCode, the pools' cards included, that include accepts, or all of them when it is nil. It
// returns ErrUnknownSet and ErrNoCounter. g.mu must be held.
func (g *Game) duplicateLines(setCode string, include func(domain.CardID) bool) ([]domain.TradeLine, error) {
	binder, ok := g.binders[setCode]
	if !ok {
		return nil, ErrUnknownSet
	}
	return g.spareLines(func(id domain.CardID) bool {
		_, inBinder := binder.Cards[id]
		return inBinder && (include == nil || include(id))
	})
}

// Spare returns the copies above a playset of every card the player holds, but those the market
// holds, in card order: what the player can put in a counter. It returns ErrNoMarket in a game
// without a market.
func (g *Game) Spare() ([]CardLine, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, err := g.syncMarket(); err != nil {
		return nil, fmt.Errorf("spare cards: %w", err)
	}
	lines, err := g.spareLines(func(domain.CardID) bool { return true })
	if err != nil {
		return nil, fmt.Errorf("spare cards: %w", err)
	}
	return g.cardLines(lines), nil
}

// spareLines is the copies above a playset of the cards that include accepts, keeping the foils
// longest and leaving the copies the market holds. It returns ErrNoCounter. g.mu must be held.
func (g *Game) spareLines(include func(domain.CardID) bool) ([]domain.TradeLine, error) {
	if err := g.syncCounter(); err != nil {
		return nil, err
	}
	surplus, err := g.collection.DuplicatesAbove(domain.PlaysetSize)
	if err != nil {
		return nil, err
	}
	var lines []domain.TradeLine
	for _, s := range surplus {
		if !include(s.Key.ID) {
			continue
		}
		// The copies the market holds, and the ones a milestone locks, are never sold.
		if qty := min(s.Quantity, g.collection.Quantity(s.Key)-g.unspendable(s.Key)); qty > 0 {
			lines = append(lines, domain.TradeLine{Key: s.Key, Qty: qty})
		}
	}
	return lines, nil
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
	if !ok || !slices.Contains(card.Finishes, finish) {
		return nil, key, ErrUnknownCard
	}
	if err := g.syncCounter(); err != nil {
		return nil, key, err
	}
	return g.economy, key, nil
}

// syncCounter catches the Counter up with the days gone by, with the sets that the market's trades
// meanwhile unlocked. g.mu must be held.
func (g *Game) syncCounter() error {
	if g.economy == nil {
		return ErrNoCounter
	}
	if err := g.economy.catchUp(); err != nil {
		return err
	}
	g.unlockCaughtUp()
	return nil
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
