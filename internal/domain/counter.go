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
	// FoilPct is the percentage of a card's reference price that its foil is worth, for both the
	// price the Counter pays and the price it asks. Zero and 100 both price a foil as its non-foil.
	FoilPct Percent
	// InitialStock is the number of copies stocked per rarity at construction.
	// Absent rarities default to zero. Only non-foil, non-basic-land cards are
	// stocked initially.
	InitialStock map[Rarity]int
	// ShelfLife is how long a card sold to the Counter, by the player or as NPC surplus, stays
	// in stock. The zero value keeps it forever.
	ShelfLife ShelfLife
	// Treasury makes the Counter pay from, and be paid into, a guarded treasury account instead of
	// minting the coins it pays and destroying the coins it is paid.
	Treasury bool
}

// ShelfLife is how long a card sold to the Counter stays on its shelf. The zero value is
// forever, so a CounterParams that does not set it keeps cards as the Counter always did.
type ShelfLife struct {
	// Limited is true when the card vanishes after Days. Days is ignored otherwise.
	Limited bool
	// Days is the number of days on the shelf. Zero means the card vanishes at once, which
	// makes the Counter a sink. It must not be negative.
	Days int
}

// ForeverOnShelf returns the ShelfLife of a card that never vanishes.
func ForeverOnShelf() ShelfLife { return ShelfLife{} }

// VanishAfterDays returns the ShelfLife of a card that vanishes after days days on the shelf.
func VanishAfterDays(days int) ShelfLife { return ShelfLife{Limited: true, Days: days} }

var (
	// ErrUnknownCard is returned when a trade references a CardID absent from
	// all catalogues passed to NewCounter, or when the requested Finish is not
	// listed for that card.
	ErrUnknownCard = errors.New("counter: card not in catalogue")

	// ErrCounterBroke is returned by BuyFromPlayer when the Counter has a treasury and the treasury
	// cannot pay for the card. The card is not taken.
	ErrCounterBroke = errors.New("counter: treasury cannot pay for the card")

	// ErrOutOfStock is returned by SellToPlayer when the Counter holds no
	// copies of the requested card.
	ErrOutOfStock = errors.New("counter: card out of stock")
)

// Counter is the in-game shop. It buys cards from the player (BuyFromPlayer)
// and sells them to the player (SellToPlayer) at fixed prices derived from a
// PriceSnapshot. All transactions are posted to a shared Journal.
//
// Stock is keyed by CardKey (CardID + Finish). Reference prices are looked up
// by CardID only, and a foil is worth CounterParams.FoilPct percent of its
// non-foil counterpart's reference price. That factor stands in for a per-finish
// price source.
//
// The zero value is not useful; use NewCounter.
type Counter struct {
	params    CounterParams
	valuer    Valuer
	journal   *Journal
	stock     map[CardKey]int // initial stock, and sold cards when they never vanish
	resold    map[CardKey][]shelfBatch
	tick      int
	cardIndex map[CardID]Card
}

// shelfBatch is the copies of a card sold to the Counter at the same tick. A card is on the shelf
// only when its ShelfLife is limited and above zero days. Batches of a card are in arrival order.
type shelfBatch struct {
	tick int
	qty  int
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
//   - a limited ShelfLife must have Days >= 0
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

	if params.ShelfLife.Limited && params.ShelfLife.Days < 0 {
		return nil, fmt.Errorf("counter: ShelfLife.Days must be >= 0, got %d", params.ShelfLife.Days)
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
		valuer:    NewValuer(prices, params.FallbackPrice),
		journal:   journal,
		stock:     stock,
		resold:    make(map[CardKey][]shelfBatch),
		cardIndex: cardIndex,
	}, nil
}

// BuyFromPlayer processes the player selling one card to the Counter.
// The Counter pays the player SellPct% of the card's reference value.
//
// On success: posts Entry{From: AccountCounterSale, To: AccountPlayer} and
// puts one copy of key on the shelf for params.ShelfLife: forever by default, or until it
// vanishes. The Counter always accepts cards from the player; existing stock is not checked.
//
// Returns ErrUnknownCard if the card ID is not in any catalogue or if the
// requested finish is not listed for that card.
// Returns a wrapped ErrCounterBroke, with the card not taken, if the Counter has a treasury that
// cannot pay.
// Returns a wrapped ErrSubCoin if the trade is worth less than one coin.
// This covers both genuinely sub-coin prices and zero prices
// (e.g. FallbackPrice == 0 or a snapshot price of 0).
func (c *Counter) BuyFromPlayer(key CardKey) error {
	if _, err := c.lookupCard(key); err != nil {
		return err
	}
	coins, err := c.tradeCoins(c.referencePrice(key), c.params.SellPct)
	if err != nil {
		return err
	}
	from := AccountCounterSale
	if c.params.Treasury {
		from = AccountCounterTreasury
	}
	if err := c.journal.Post(Entry{
		From:   from,
		To:     AccountPlayer,
		Amount: coins,
		Memo:   fmt.Sprintf("buy from player: card %s finish %s", key.ID, key.Finish),
	}); err != nil {
		if c.params.Treasury && errors.Is(err, ErrInsufficientBalance) {
			return fmt.Errorf("counter: card %q finish %q: %w", key.ID, key.Finish, ErrCounterBroke)
		}
		return fmt.Errorf("counter: post payment for card %q finish %q: %w", key.ID, key.Finish, err)
	}
	c.shelve(key, 1)
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
	if c.Stock(key) <= 0 {
		return fmt.Errorf("counter: card %q finish %q: %w", key.ID, key.Finish, ErrOutOfStock)
	}
	coins, err := c.tradeCoins(c.referencePrice(key), c.params.BuyPct)
	if err != nil {
		return err
	}
	to := AccountCounterPurchase
	if c.params.Treasury {
		to = AccountCounterTreasury
	}
	if err := c.journal.Post(Entry{
		From:   AccountPlayer,
		To:     to,
		Amount: coins,
		Memo:   fmt.Sprintf("sell to player: card %s finish %s", key.ID, key.Finish),
	}); err != nil {
		return fmt.Errorf("counter: post payment for card %q finish %q: %w", key.ID, key.Finish, err)
	}
	c.unshelve(key)
	return nil
}

// AcceptSurplus shelves n copies of key without touching the ledger. It also stocks the cards a
// CounterSupply opens for the Counter, which cost nothing and pay nothing either: NPCs hold no coins
// and have no account, so what they sell to the Counter costs nothing and pays nothing.
//
// Returns ErrUnknownCard if the card ID is not in any catalogue or if the finish is not listed
// for that card, and InvalidQuantityError when n is not positive. Stock is unchanged on error.
// The surplus goes on the shelf for params.ShelfLife, like a card sold by the player.
func (c *Counter) AcceptSurplus(key CardKey, n int) error {
	if n <= 0 {
		return &InvalidQuantityError{N: n}
	}
	if _, err := c.lookupCard(key); err != nil {
		return err
	}
	c.shelve(key, n)
	return nil
}

// Tick advances the Counter's clock by one tick and removes the cards whose shelf life has ended.
// Initial stock never vanishes: only cards sold to the Counter do.
func (c *Counter) Tick() {
	c.tick++
	if len(c.resold) == 0 {
		return
	}
	lifeTicks := c.params.ShelfLife.Days * TicksPerDay
	for key, batches := range c.resold {
		expired := 0
		for expired < len(batches) && c.tick-batches[expired].tick >= lifeTicks {
			expired++
		}
		switch {
		case expired == len(batches):
			delete(c.resold, key)
		case expired > 0:
			c.resold[key] = batches[expired:]
		}
	}
}

// Stock returns the current number of copies of key held by the Counter.
func (c *Counter) Stock(key CardKey) int {
	return c.stock[key] + shelved(c.resold[key])
}

// shelve puts n sold copies of key on the shelf for the ShelfLife of the Counter.
func (c *Counter) shelve(key CardKey, n int) {
	life := c.params.ShelfLife
	switch {
	case !life.Limited:
		c.stock[key] += n
	case life.Days == 0:
		// A sink: the copies are gone.
	default:
		batches := c.resold[key]
		if last := len(batches) - 1; last >= 0 && batches[last].tick == c.tick {
			batches[last].qty += n
			return
		}
		c.resold[key] = append(batches, shelfBatch{tick: c.tick, qty: n})
	}
}

// unshelve takes one copy of key off the shelf for the player. The oldest sold copy goes first,
// so the copies about to vanish are not wasted. Initial stock goes last. The caller has checked
// that the Counter holds a copy.
func (c *Counter) unshelve(key CardKey) {
	batches := c.resold[key]
	if len(batches) == 0 {
		c.stock[key]--
		return
	}
	if batches[0].qty > 1 {
		batches[0].qty--
		return
	}
	if len(batches) == 1 {
		delete(c.resold, key)
		return
	}
	c.resold[key] = batches[1:]
}

func shelved(batches []shelfBatch) int {
	var total int
	for _, b := range batches {
		total += b.qty
	}
	return total
}

// StockByRarity returns the total copies in stock grouped by rarity.
// Only cards known to the Counter's catalogue contribute to the result.
func (c *Counter) StockByRarity() map[Rarity]int {
	result := make(map[Rarity]int)
	add := func(key CardKey, qty int) {
		if qty <= 0 {
			return
		}
		if card, ok := c.cardIndex[key.ID]; ok {
			result[card.Rarity] += qty
		}
	}
	for key, qty := range c.stock {
		add(key, qty)
	}
	for key, batches := range c.resold {
		add(key, shelved(batches))
	}
	return result
}

// referencePrice returns the reference price for a card. Cards absent from the snapshot fall back
// to FallbackPrice. A foil is worth FoilPct percent of the price its CardID carries.
func (c *Counter) referencePrice(key CardKey) USDCents {
	price := c.valuer.Price(key.ID)
	if key.Finish == FinishFoil {
		return FoilPrice(price, c.params.FoilPct)
	}
	return price
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
