package domain

import (
	"fmt"
	"maps"
	"slices"
	"time"
)

// MarketSale is a card that NPCs hold on their shelves, with who holds it and at what price.
type MarketSale struct {
	Card CardKey
	// Sellers are the NPCs that hold the card, cheapest first, and then by ID.
	Sellers []MarketSeller
}

// MarketSeller is an NPC that holds a card on its shelf.
type MarketSeller struct {
	NPC int
	// Price is what the NPC asks for one copy, the same price its answer to a request carries.
	Price Coins
	// Qty is the copies on its shelf.
	Qty int
}

// ForSale returns every card that an NPC that trades holds on its shelf, sorted by card.
func (m *Market) ForSale() []MarketSale {
	var sales []MarketSale
	for _, card := range slices.SortedFunc(maps.Keys(m.crowd.holders), cmpKey) {
		var sellers []MarketSeller
		for _, npc := range m.holders(card) {
			sellers = append(sellers, MarketSeller{NPC: npc.ID, Price: m.askingPrice(npc, card), Qty: npc.ShelfQuantity(card)})
		}
		if len(sellers) == 0 {
			continue
		}
		slices.SortStableFunc(sellers, func(a, b MarketSeller) int { return int(a.Price - b.Price) })
		sales = append(sales, MarketSale{Card: card, Sellers: sellers})
	}
	return sales
}

// askingPrice is what npc asks for one copy of card from its shelf: the reference value plus its
// margin, at least one coin. It is the price at which the acceptance rule of the Exchange takes a
// coin offer.
func (m *Market) askingPrice(npc *NPC, card CardKey) Coins {
	ask := Offer{NPC: npc.ID, Origin: OriginNPC, NPCGives: lineOf(card)}
	return max(1, requiredTopUp(0, m.valuer.Price(card.ID), npcMargin(npc, ask, m.crowd.tick), m.params.CoinRate))
}

// shelfHolder returns npc when it trades and holds card on its shelf. It returns ErrNPCLacksCard
// otherwise.
func (m *Market) shelfHolder(id int, card CardKey) (*NPC, error) {
	npc, ok := m.crowd.byID[id]
	if !ok || npc.Behaviour == nil || npc.ShelfQuantity(card) == 0 {
		return nil, fmt.Errorf("%w: NPC %d, %s", ErrNPCLacksCard, id, card.ID)
	}
	return npc, nil
}

// BuyNow buys one copy of card from the shelf of NPC npc at its asking price, at once. It returns
// the answer it took.
//
// It returns ErrNPCLacksCard when the NPC does not trade or has no copy, and
// ErrInsufficientBalance when the player cannot pay, and then changes nothing.
func (m *Market) BuyNow(npcID int, card CardKey, now time.Time) (MarketAnswer, error) {
	npc, err := m.shelfHolder(npcID, card)
	if err != nil {
		return MarketAnswer{}, fmt.Errorf("market: buy now: %w", err)
	}
	price := m.askingPrice(npc, card)
	if price > m.journal.Balance(AccountPlayer) {
		return MarketAnswer{}, fmt.Errorf("market: buy now: %w: %d coins", ErrInsufficientBalance, price)
	}
	a := MarketAnswer{ID: m.newID(), Kind: AnswerAsk, Card: card, NPC: npcID, Coins: price, At: now, Expires: now}
	if err := m.settle(a, a.terms(), now); err != nil {
		return MarketAnswer{}, fmt.Errorf("market: buy now: %w", err)
	}
	return a, nil
}

// Offer offers coins, below its asking price, to NPC npc for one copy of card on its shelf. The NPC
// replies once, after the delay of its archetype, by the acceptance rule of a counter: it is a
// counter on an asking price, with no ask-around before it. The request it makes ends with the
// reply.
//
// It returns ErrInvalidPrice for a price below one coin, ErrNPCLacksCard, ErrAlreadyAsked while the
// player waits on a request for the card, ErrOfferNotBelowAsk and ErrInsufficientBalance, and then
// changes nothing.
func (m *Market) Offer(npcID int, card CardKey, coins Coins, now time.Time) (MarketID, error) {
	if coins < 1 {
		return 0, fmt.Errorf("market: offer: %w: %d", ErrInvalidPrice, coins)
	}
	npc, err := m.shelfHolder(npcID, card)
	if err != nil {
		return 0, fmt.Errorf("market: offer: %w", err)
	}
	over, err := m.overRequest(card)
	if err != nil {
		return 0, fmt.Errorf("market: offer: %w", err)
	}
	price := m.askingPrice(npc, card)
	switch {
	case coins >= price:
		return 0, fmt.Errorf("market: offer: %w: %d coins for a price of %d", ErrOfferNotBelowAsk, coins, price)
	case coins > m.journal.Balance(AccountPlayer):
		return 0, fmt.Errorf("market: offer: %w: %d coins", ErrInsufficientBalance, coins)
	}
	if over != 0 {
		m.end(over)
	}
	id := m.newID()
	m.requests = append(m.requests, MarketRequest{ID: id, Card: card, At: now, Asked: 1, Direct: true})
	m.answers = append(m.answers, MarketAnswer{
		ID: m.newID(), To: id, Kind: AnswerAsk, Card: card, NPC: npcID, Coins: price, At: now, Expires: now.Add(m.params.Expiry),
		Counter: &MarketCounter{Coins: coins, ReplyAt: now.Add(npc.ResponseDelay)},
	})
	return id, nil
}

// NextAnswer returns when the next NPC answers the request or listing id, and false when none is to
// answer.
func (m *Market) NextAnswer(id MarketID) (time.Time, bool) {
	var next time.Time
	found := false
	for _, p := range m.pending {
		if p.To == id && (!found || p.Due.Before(next)) {
			next, found = p.Due, true
		}
	}
	return next, found
}
