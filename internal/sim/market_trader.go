package sim

import (
	"cmp"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// referencePct values a listed card at its reference price, with no Counter margin.
const referencePct domain.Percent = 100

// MarketTraderPolicy is a PlayerPolicy that plays like the collector and trades on the market too.
//
// Per session, in order:
//  1. Accept answers: the cheapest asking price it can pay above the coin floor for each request, an
//     offer of a card it lacks on a listing, else the highest coin offer.
//  2. Play as the collector: open boosters, sell duplicates above a playset to the Counter and buy
//     missing cards from it.
//  3. Ask around for every missing rare and mythic.
//  4. List every card it holds twice at its reference value.
//
// It never counters an answer, so the market's delays are the only thing slowing it down.
type MarketTraderPolicy struct {
	collector CollectorPolicy
}

// NewMarketTraderPolicy returns a MarketTraderPolicy ready to use.
func NewMarketTraderPolicy() MarketTraderPolicy {
	return MarketTraderPolicy{collector: NewCollectorPolicy()}
}

// UsesMarket implements MarketPlayer.
func (MarketTraderPolicy) UsesMarket() bool { return true }

// RunSession implements PlayerPolicy.
func (p MarketTraderPolicy) RunSession(s Session) {
	acceptAnswers(s)
	p.collector.RunSession(s)
	askForMissingRares(s)
	listDuplicates(s)
}

// acceptAnswers takes the answers worth taking, at most one per request or listing.
func acceptAnswers(s Session) {
	v := s.View()
	owned := ownedIDs(v)
	spendable := v.Balance - v.DailyReward*domain.Coins(collectorReserveDays)
	served := map[domain.MarketID]bool{}
	var asks, cardOffers, coinOffers []domain.MarketAnswer
	for _, a := range s.MarketAnswers() {
		switch {
		case a.Counter != nil:
		case a.Kind == domain.AnswerAsk:
			asks = append(asks, a)
		case offersMissingCard(a, owned):
			cardOffers = append(cardOffers, a)
		case len(a.Cards) == 0 && a.Coins > 0:
			coinOffers = append(coinOffers, a)
		}
	}
	slices.SortFunc(asks, func(a, b domain.MarketAnswer) int {
		return cmp.Or(cmp.Compare(a.Coins, b.Coins), cmp.Compare(a.ID, b.ID))
	})
	slices.SortFunc(coinOffers, func(a, b domain.MarketAnswer) int {
		return cmp.Or(cmp.Compare(b.Coins, a.Coins), cmp.Compare(a.ID, b.ID))
	})

	for _, a := range asks {
		if served[a.To] || owned[a.Card.ID] || a.Coins > spendable {
			continue
		}
		if s.MarketAccept(a.ID) == nil {
			served[a.To] = true
			owned[a.Card.ID] = true
			spendable -= a.Coins
		}
	}
	for _, a := range append(cardOffers, coinOffers...) {
		if served[a.To] {
			continue
		}
		if s.MarketAccept(a.ID) == nil {
			served[a.To] = true
			for _, l := range a.Cards {
				owned[l.Key.ID] = true
			}
		}
	}
}

// offersMissingCard reports whether an offer gives at least one card the player lacks.
func offersMissingCard(a domain.MarketAnswer, owned map[domain.CardID]bool) bool {
	return slices.ContainsFunc(a.Cards, func(l domain.TradeLine) bool { return !owned[l.Key.ID] })
}

// askForMissingRares asks around for each rare and mythic the player does not own, which the market
// refuses for a card no NPC holds or one already asked for.
func askForMissingRares(s Session) {
	for _, key := range missingRares(s.View()) {
		_, _ = s.MarketAsk(key)
	}
}

// missingRares returns the non-foil rares and mythics of the catalogues, basic lands aside, that the
// player owns in no finish, by card ID.
func missingRares(v WorldView) []domain.CardKey {
	owned := ownedIDs(v)
	seen := map[domain.CardID]bool{}
	var keys []domain.CardKey
	for _, cat := range v.Catalogues {
		for id, card := range cat.Cards {
			if (card.Rarity != domain.RarityRare && card.Rarity != domain.RarityMythic) || card.IsBasicLand ||
				!hasNonFoil(card.Finishes) || owned[id] || seen[id] {
				continue
			}
			seen[id] = true
			keys = append(keys, domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
		}
	}
	slices.SortFunc(keys, func(a, b domain.CardKey) int { return cmp.Compare(a.ID, b.ID) })
	return keys
}

// listDuplicates lists every non-foil card the player holds at least twice, at its reference value,
// which the market refuses for a card already listed or one worth under a coin.
func listDuplicates(s Session) {
	v := s.View()
	var keys []domain.CardKey
	for key, qty := range v.OwnedCards {
		if key.Finish == domain.FinishNonFoil && qty >= 2 {
			keys = append(keys, key)
		}
	}
	slices.SortFunc(keys, func(a, b domain.CardKey) int { return cmp.Compare(a.ID, b.ID) })
	for _, key := range keys {
		price, err := domain.PriceToCoins(priceOf(v, key.ID), v.CoinRate, referencePct)
		if err != nil || price == 0 {
			continue
		}
		_, _ = s.MarketList(key, price)
	}
}
