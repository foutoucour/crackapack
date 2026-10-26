package play

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ForSale returns every card that an NPC that trades holds on its shelf, in the sets the player
// can buy from, with what each NPC asks for it, once the market is caught up. It returns
// ErrNoMarket in a game without one.
func (g *Game) ForSale() ([]ForSale, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, err := g.syncMarket()
	if err != nil {
		return nil, err
	}
	var out []ForSale
	for _, sale := range m.ForSale() {
		card := g.cards[sale.Card.ID]
		if !g.buyable(card.ID) {
			continue
		}
		out = append(out, ForSale{
			Card: card, Finish: sale.Card.Finish, SetCode: g.unlockedBinderOf(card.ID),
			Hunted:  g.huntIndex(sale.Card) >= 0,
			Missing: g.collection.Copies(card.ID) == 0,
			Sellers: g.sellers(sale.Sellers),
		})
	}
	return out, nil
}

func (g *Game) sellers(sellers []domain.MarketSeller) []Seller {
	out := make([]Seller, len(sellers))
	for i, s := range sellers {
		out[i] = Seller{NPC: s.NPC, Price: s.Price, Qty: s.Qty}
		if npc, ok := g.economy.crowd.NPC(s.NPC); ok {
			out[i].Name, out[i].Archetype = npc.Name, npc.Archetype
		}
	}
	return out
}

// BuyNow buys the card id in finish from NPC npc at its asking price, at once, and returns what it
// paid and the sets the card unlocked. It returns ErrUnknownCard, ErrLocked when no unlocked set's binder holds the
// card, ErrGone when the NPC does not hold a copy, ErrNotEnoughCoins or ErrNoMarket, wrapped, and
// then changes nothing.
func (g *Game) BuyNow(npc int, id domain.CardID, finish domain.Finish) (Purchase, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, key, err := g.marketFor(id, finish)
	if err != nil {
		return Purchase{}, fmt.Errorf("buy %q from NPC %d: %w", id, npc, err)
	}
	if !g.buyable(id) {
		return Purchase{}, fmt.Errorf("buy %q from NPC %d: %w", id, npc, ErrLocked)
	}
	a, err := m.BuyNow(npc, key, g.now())
	if err != nil {
		return Purchase{}, fmt.Errorf("buy %q from NPC %d: %w", id, npc, marketError(err))
	}
	p := Purchase{Paid: a.Coins, Unlocked: g.unlockEarned()}
	g.save()
	return p, nil
}

// OfferTo offers coins, below its asking price, to NPC npc for the card id in finish. The NPC
// replies once, after the delay of its personality, see Market. It returns ErrUnknownCard,
// ErrLocked, ErrInvalidPrice, ErrOfferNotBelowAsk, ErrGone, ErrAlreadyAsked, ErrNotEnoughCoins or
// ErrNoMarket, wrapped, and then changes nothing.
func (g *Game) OfferTo(npc int, id domain.CardID, finish domain.Finish, coins domain.Coins) (domain.MarketID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, key, err := g.marketFor(id, finish)
	if err != nil {
		return 0, fmt.Errorf("offer for %q to NPC %d: %w", id, npc, err)
	}
	if !g.buyable(id) {
		return 0, fmt.Errorf("offer for %q to NPC %d: %w", id, npc, ErrLocked)
	}
	rid, err := m.Offer(npc, key, coins, g.now())
	if err != nil {
		return 0, fmt.Errorf("offer for %q to NPC %d: %w", id, npc, marketError(err))
	}
	if err := g.deliver(); err != nil {
		return rid, fmt.Errorf("offer for %q to NPC %d: %w", id, npc, err)
	}
	return rid, nil
}
