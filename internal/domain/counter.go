package domain

import (
	"errors"
	"fmt"
)

// CounterParams holds the Counter's configuration. The composition root fills
// this from internal/config; the domain never imports config.
type CounterParams struct {
	// SellPct is the percentage of the reference value the Counter pays the
	// player when it buys a card from the player (default 60).
	SellPct Percent
	// BuyPct is the percentage of the reference value the player pays the
	// Counter when buying a card from the Counter (default 120).
	BuyPct Percent
	// CoinRate is the number of coins per US dollar.
	CoinRate CoinRate
	// FallbackPrice is the reference value in USDCents for cards absent from
	// the price snapshot.
	FallbackPrice USDCents
	// InitialStock is the number of copies stocked per rarity at construction.
	// Absent rarities default to zero. Only non-foil, non-basic-land cards are
	// stocked initially.
	InitialStock map[Rarity]int
}

var (
	// ErrUnknownCard is returned when a trade references a CardID absent from
	// all catalogues passed to NewCounter, or when the requested Finish is not
	// listed for that card.
	ErrUnknownCard = errors.New("counter: card not in catalogue")

	// ErrOutOfStock is returned by SellToPlayer when the Counter holds no
	// copies of the requested card.
	ErrOutOfStock = errors.New("counter: card out of stock")
)

// Counter is the in-game shop. It buys cards from the player (BuyFromPlayer)
// and sells them to the player (SellToPlayer) at fixed prices derived from a
// PriceSnapshot. All transactions are posted to a shared Journal.
//
// Stock is keyed by CardKey (CardID + Finish). Reference prices are looked up
// by CardID only; a foil and its non-foil counterpart therefore share the same
// reference price. This is a documented simplification pending a per-finish
// price source.
//
// The zero value is not useful; use NewCounter.
type Counter struct {
	params    CounterParams
	prices    PriceSnapshot
	journal   *Journal
	stock     map[CardKey]int
	cardIndex map[CardID]Card
}

// NewCounter builds a Counter and populates initial stock from cats.
//
// Initial stock covers every non-foil, non-basic-land card in cats, with
// params.InitialStock[rarity] copies each (0 if the rarity is absent from the
// map). A CardID that appears in more than one catalogue is a data error;
// NewCounter returns an error naming the duplicate ID.
//
// NewCounter validates params and returns an error naming the offending field:
//   - journal must not be nil
//   - CoinRate must be > 0
//   - SellPct must be > 0
//   - SellPct must be <= BuyPct (otherwise a round trip through the Counter profits)
//   - FallbackPrice must be >= 0
//   - every value in InitialStock must be >= 0
func NewCounter(cats []SetCatalogue, prices PriceSnapshot, journal *Journal, params CounterParams) (*Counter, error) {
	if journal == nil {
		return nil, fmt.Errorf("counter: journal must not be nil")
	}
	if params.CoinRate <= 0 {
		return nil, fmt.Errorf("counter: CoinRate must be > 0, got %d", params.CoinRate)
	}
	if params.SellPct <= 0 {
		return nil, fmt.Errorf("counter: SellPct must be > 0, got %d", params.SellPct)
	}
	if params.SellPct > params.BuyPct {
		return nil, fmt.Errorf("counter: SellPct (%d) must be <= BuyPct (%d); a higher sell percentage would let the player profit from a round trip",
			params.SellPct, params.BuyPct)
	}
	if params.FallbackPrice < 0 {
		return nil, fmt.Errorf("counter: FallbackPrice must be >= 0, got %d", params.FallbackPrice)
	}
	for r, v := range params.InitialStock {
		if v < 0 {
			return nil, fmt.Errorf("counter: InitialStock[%s] must be >= 0, got %d", r, v)
		}
	}

	cardIndex := make(map[CardID]Card)
	for _, cat := range cats {
		for id, card := range cat.Cards {
			if _, exists := cardIndex[id]; exists {
				return nil, fmt.Errorf("counter: card %q appears in more than one catalogue", id)
			}
			cardIndex[id] = card
		}
	}

	stock := make(map[CardKey]int)
	for _, cat := range cats {
		for id, card := range cat.Cards {
			if card.IsBasicLand {
				continue
			}
			if !hasFinish(card.Finishes, FinishNonFoil) {
				continue
			}
			qty := params.InitialStock[card.Rarity]
			if qty > 0 {
				stock[CardKey{ID: id, Finish: FinishNonFoil}] = qty
			}
		}
	}

	return &Counter{
		params:    params,
		prices:    prices,
		journal:   journal,
		stock:     stock,
		cardIndex: cardIndex,
	}, nil
}

// BuyFromPlayer processes the player selling one card to the Counter.
// The Counter pays the player SellPct% of the card's reference value.
//
// On success: posts Entry{From: AccountCounterSale, To: AccountPlayer} and
// adds one copy of key to stock. The Counter always accepts cards from the
// player; existing stock is not checked.
//
// Returns ErrUnknownCard if the card ID is not in any catalogue or if the
// requested finish is not listed for that card.
// Returns a wrapped ErrSubCoin if the trade is worth less than one coin.
// This covers both genuinely sub-coin prices and zero prices
// (e.g. FallbackPrice == 0 or a snapshot price of 0).
func (c *Counter) BuyFromPlayer(key CardKey) error {
	if _, err := c.lookupCard(key); err != nil {
		return err
	}
	coins, err := c.tradeCoins(c.referencePrice(key.ID), c.params.SellPct)
	if err != nil {
		return err
	}
	if err := c.journal.Post(Entry{
		From:   AccountCounterSale,
		To:     AccountPlayer,
		Amount: coins,
		Memo:   fmt.Sprintf("buy from player: card %s finish %s", key.ID, key.Finish),
	}); err != nil {
		return fmt.Errorf("counter: post payment for card %q finish %q: %w", key.ID, key.Finish, err)
	}
	c.stock[key]++
	return nil
}

// SellToPlayer processes the player buying one card from the Counter.
// The player pays BuyPct% of the card's reference value.
//
// Order of operations: validate (card known, finish listed, stock > 0, price
// worth at least one coin), post to the journal, then update stock. A ledger
// failure (e.g. ErrInsufficientBalance) therefore leaves stock untouched.
//
// Returns ErrUnknownCard if the card ID is not in any catalogue or if the
// requested finish is not listed for that card.
// Returns ErrOutOfStock if no copies are available.
// Returns a wrapped ErrSubCoin if the trade is worth less than one coin.
// Returns a wrapped ErrInsufficientBalance if the player cannot pay.
func (c *Counter) SellToPlayer(key CardKey) error {
	if _, err := c.lookupCard(key); err != nil {
		return err
	}
	if c.stock[key] <= 0 {
		return fmt.Errorf("counter: card %q finish %q: %w", key.ID, key.Finish, ErrOutOfStock)
	}
	coins, err := c.tradeCoins(c.referencePrice(key.ID), c.params.BuyPct)
	if err != nil {
		return err
	}
	if err := c.journal.Post(Entry{
		From:   AccountPlayer,
		To:     AccountCounterPurchase,
		Amount: coins,
		Memo:   fmt.Sprintf("sell to player: card %s finish %s", key.ID, key.Finish),
	}); err != nil {
		return fmt.Errorf("counter: post payment for card %q finish %q: %w", key.ID, key.Finish, err)
	}
	c.stock[key]--
	return nil
}

// AcceptSurplus adds n copies of key to stock without touching the ledger: NPCs hold no coins
// and have no account, so what they sell to the Counter costs nothing and pays nothing.
//
// Returns ErrUnknownCard if the card ID is not in any catalogue or if the finish is not listed
// for that card, and InvalidQuantityError when n is not positive. Stock is unchanged on error.
func (c *Counter) AcceptSurplus(key CardKey, n int) error {
	if n <= 0 {
		return &InvalidQuantityError{N: n}
	}
	if _, err := c.lookupCard(key); err != nil {
		return err
	}
	c.stock[key] += n
	return nil
}

// Stock returns the current number of copies of key held by the Counter.
func (c *Counter) Stock(key CardKey) int {
	return c.stock[key]
}

// StockByRarity returns the total copies in stock grouped by rarity.
// Only cards known to the Counter's catalogue contribute to the result.
func (c *Counter) StockByRarity() map[Rarity]int {
	result := make(map[Rarity]int)
	for key, qty := range c.stock {
		if qty <= 0 {
			continue
		}
		card, ok := c.cardIndex[key.ID]
		if !ok {
			continue
		}
		result[card.Rarity] += qty
	}
	return result
}

// referencePrice returns the reference price for a card by ID. Cards absent
// from the snapshot fall back to FallbackPrice. A foil and its non-foil share
// the same CardID and therefore the same price; this is a documented
// simplification pending a per-finish price source.
func (c *Counter) referencePrice(id CardID) USDCents {
	if p, ok := c.prices[id]; ok {
		return p
	}
	return c.params.FallbackPrice
}

// tradeCoins converts a reference price to coins at the given percentage.
// A trade worth less than one coin is refused: this covers both sub-coin
// prices (ErrSubCoin from PriceToCoins) and zero prices (price or pct zero,
// which PriceToCoins returns as (0, nil)). Both are reported as a wrapped
// ErrSubCoin so callers receive one consistent refusal signal.
func (c *Counter) tradeCoins(price USDCents, pct Percent) (Coins, error) {
	coins, err := PriceToCoins(price, c.params.CoinRate, pct)
	if err != nil {
		if errors.Is(err, ErrSubCoin) {
			return 0, fmt.Errorf("counter: trade refused, worth less than one coin: %w", ErrSubCoin)
		}
		return 0, fmt.Errorf("counter: price conversion: %w", err)
	}
	if coins == 0 {
		// price == 0 yields (0, nil) from PriceToCoins; refuse rather than letting
		// the ledger's ErrNonPositiveAmount surface as an opaque error.
		return 0, fmt.Errorf("counter: trade refused, worth less than one coin: %w", ErrSubCoin)
	}
	return coins, nil
}

// lookupCard returns the Card for key, verifying that the finish is listed in
// the card's Finishes slice. Returns a wrapped ErrUnknownCard if the CardID is
// absent or if the finish is not listed.
func (c *Counter) lookupCard(key CardKey) (Card, error) {
	card, ok := c.cardIndex[key.ID]
	if !ok {
		return Card{}, fmt.Errorf("counter: card %q finish %q: %w", key.ID, key.Finish, ErrUnknownCard)
	}
	if !hasFinish(card.Finishes, key.Finish) {
		return Card{}, fmt.Errorf("counter: card %q finish %q: %w", key.ID, key.Finish, ErrUnknownCard)
	}
	return card, nil
}

// hasFinish reports whether f appears in finishes.
func hasFinish(finishes []Finish, f Finish) bool {
	for _, fin := range finishes {
		if fin == f {
			return true
		}
	}
	return false
}
