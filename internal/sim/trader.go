package sim

import (
	"cmp"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

const (
	// maxSwapProposals bounds the swaps a TraderPolicy proposes in one session, so a session's
	// cost does not depend on the size of the crowd.
	maxSwapProposals = 20

	// tradeCardCap is the most cards a TraderPolicy puts on its side of an offer. The Exchange
	// may allow fewer, and refuses an offer that goes over.
	tradeCardCap = 10
)

// TraderPolicy is a PlayerPolicy that collects like the CollectorPolicy and also trades with NPCs.
//
// Per session, in order:
//  1. Accept the offers NPCs made that bring a card the player lacks.
//  2. Swap: for each missing card an NPC holds on its shelf, offer the player's cheapest duplicates
//     for it, and accept the NPC's counter when it fits above the coin floor.
//  3. Play the collector's session: open boosters, sell duplicates above a playset, buy missing
//     cards from the Counter.
//
// In a run without trades the first two steps do nothing, and it plays as the collector.
type TraderPolicy struct{ collector CollectorPolicy }

// NewTraderPolicy returns a TraderPolicy ready to use.
func NewTraderPolicy() TraderPolicy { return TraderPolicy{collector: NewCollectorPolicy()} }

// RunSession implements PlayerPolicy.
func (p TraderPolicy) RunSession(s Session) {
	acceptUsefulOffers(s)
	proposeSwaps(s)
	p.collector.RunSession(s)
}

// acceptUsefulOffers accepts each open NPC offer that gives at least one card the player lacks.
func acceptUsefulOffers(s Session) {
	owned := ownedIDs(s.View())
	for _, o := range s.Offers() {
		if o.Origin != domain.OriginNPC || !givesMissingCard(o, owned) {
			continue
		}
		if d, err := s.AcceptOffer(o.ID); err == nil && d.Outcome == domain.OutcomeAccepted {
			owned = ownedIDs(s.View())
		}
	}
}

func ownedIDs(v WorldView) map[domain.CardID]bool {
	owned := make(map[domain.CardID]bool, len(v.OwnedCards))
	for key, qty := range v.OwnedCards {
		if qty > 0 {
			owned[key.ID] = true
		}
	}
	return owned
}

func givesMissingCard(o domain.Offer, owned map[domain.CardID]bool) bool {
	for _, l := range o.NPCGives {
		if !owned[l.Key.ID] {
			return true
		}
	}
	return false
}

// proposeSwaps offers duplicates for the missing cards NPCs hold, cheapest missing card first.
func proposeSwaps(s Session) {
	v := s.View()
	proposals := 0
	for _, want := range buildBuyCandidates(v) {
		for _, npcID := range s.HoldersOf(want.key) {
			if proposals >= maxSwapProposals {
				return
			}
			give := cheapestDuplicates(v, priceOf(v, want.key.ID))
			if len(give) == 0 {
				return
			}
			proposals++
			if trySwap(s, v, npcID, want.key, give) {
				v = s.View()
				break
			}
		}
	}
}

// trySwap proposes give for one copy of want and accepts the NPC's counter when its top-up fits
// above the coin floor. It reports whether a trade was made.
func trySwap(s Session, v WorldView, npcID int, want domain.CardKey, give []domain.TradeLine) bool {
	d := s.ProposeTrade(npcID, give, []domain.TradeLine{{Key: want, Qty: 1}}, 0)
	switch d.Outcome {
	case domain.OutcomeAccepted:
		return true
	case domain.OutcomeCountered:
		floor := v.DailyReward * domain.Coins(collectorReserveDays)
		if d.Offer.TopUp > v.Balance-floor {
			return false
		}
		accepted, err := s.AcceptOffer(d.Offer.ID)
		return err == nil && accepted.Outcome == domain.OutcomeAccepted
	}
	return false
}

// cheapestDuplicates returns non-foil copies the player holds above the first, cheapest first,
// until their value reaches targetCents or tradeCardCap cards are reached.
func cheapestDuplicates(v WorldView, targetCents domain.USDCents) []domain.TradeLine {
	totals := make(map[domain.CardID]int, len(v.OwnedCards))
	for key, qty := range v.OwnedCards {
		totals[key.ID] += qty
	}
	type duplicate struct {
		key   domain.CardKey
		spare int
		price domain.USDCents
	}
	var duplicates []duplicate
	for key, qty := range v.OwnedCards {
		if key.Finish != domain.FinishNonFoil {
			continue
		}
		if spare := min(qty, totals[key.ID]-1); spare > 0 {
			duplicates = append(duplicates, duplicate{key: key, spare: spare, price: priceOf(v, key.ID)})
		}
	}
	slices.SortFunc(duplicates, func(a, b duplicate) int {
		if c := cmp.Compare(a.price, b.price); c != 0 {
			return c
		}
		return cmp.Compare(a.key.ID, b.key.ID)
	})

	var lines []domain.TradeLine
	var cents domain.USDCents
	var cards int
	for _, d := range duplicates {
		qty := 0
		for qty < d.spare && cards < tradeCardCap && cents < targetCents {
			qty++
			cards++
			cents += d.price
		}
		if qty > 0 {
			lines = append(lines, domain.TradeLine{Key: d.key, Qty: qty})
		}
	}
	return lines
}

// priceOf returns the reference value of a card in USD cents.
func priceOf(v WorldView, id domain.CardID) domain.USDCents {
	if p, ok := v.Prices[id]; ok {
		return p
	}
	return v.FallbackPrice
}
