package domain

import (
	"fmt"
	"maps"
	"slices"
)

// StartDay opens day: it lapses the offers whose last day has passed and resets the daily cap on
// the offers NPCs make. Call it before the first tick of each day.
func (x *Exchange) StartDay(day int) {
	x.day = day
	x.dayOffers = 0
	for _, id := range slices.Sorted(maps.Keys(x.open)) {
		o := x.open[id]
		if o.LastDay >= day {
			continue
		}
		x.close(id)
		x.stats.Expired++
		x.record(o, OutcomeExpired, nil)
	}
}

// ProposeFromNPCs lets NPCs whose wish list matches a duplicate of the player propose a trade.
// Candidates are the NPCs that wish for a printing the player holds above DuplicateKeep copies,
// visited in ID order. An NPC with an open offer waits, and the rest propose with their
// OfferChancePct. Call it once per tick, after the crowd's tick.
func (x *Exchange) ProposeFromNPCs() error {
	if x.dayOffers >= x.params.MaxOffersPerDay {
		return nil
	}
	duplicates, err := x.player.DuplicatesAbove(x.params.DuplicateKeep)
	if err != nil {
		return fmt.Errorf("exchange: player duplicates: %w", err)
	}
	if len(duplicates) == 0 {
		return nil
	}
	candidates := make(map[int]struct{})
	for _, s := range duplicates {
		for id := range x.crowd.wishers[s.Key.ID] {
			candidates[id] = struct{}{}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(candidates)) {
		if x.dayOffers >= x.params.MaxOffersPerDay {
			return nil
		}
		if x.openCount[id] > 0 {
			continue
		}
		npc := x.crowd.byID[id]
		if x.crowd.rng.IntN(100) >= npc.OfferChancePct {
			continue
		}
		x.offerFrom(npc, duplicates)
	}
	return nil
}

// offerFrom asks npc's behaviour for an offer to the player and stores it when there is one.
func (x *Exchange) offerFrom(npc *NPC, duplicates []Surplus) {
	shelf := npc.Shelf()
	if len(shelf) == 0 {
		return
	}
	want, give, ok := npc.Behaviour.Propose(ProposalContext{
		NPC: npc, Wanted: wantedLines(npc, duplicates), Shelf: shelf,
		PlayerCopies: x.player.Copies, MaxCards: x.params.MaxCardsPerSide,
	})
	if !ok {
		return
	}
	x.store(Offer{
		ID: x.newID(), NPC: npc.ID, Origin: OriginNPC, PlayerGives: want, NPCGives: give,
		CreatedDay: x.day, LastDay: x.day + x.params.ExpiryDays - 1,
	})
	x.dayOffers++
	x.stats.NPCOffers++
}

// wantedLines returns what npc asks of the player: for each wish, the player's duplicates of that
// printing, up to the copies the NPC still lacks.
func wantedLines(npc *NPC, duplicates []Surplus) []TradeLine {
	var lines []TradeLine
	for _, w := range npc.wishes {
		need := w.Want - npc.collection.Copies(w.ID)
		for _, s := range duplicates {
			if need <= 0 {
				break
			}
			if s.Key.ID != w.ID {
				continue
			}
			qty := min(s.Quantity, need)
			lines = append(lines, TradeLine{Key: s.Key, Qty: qty})
			need -= qty
		}
	}
	return lines
}

// Counter answers the open offer offerID, proposed by an NPC, with other cards: the player gives
// playerGives and topUp coins instead of what the offer asked, for the same cards from the NPC.
// The NPC accepts or refuses and does not counter again, see ADR-0012. The original offer is
// closed either way.
//
// It returns ErrOfferNotFound for an offer that is not open and ErrNotCounterable for an offer
// that is not an NPC's own proposal.
func (x *Exchange) Counter(offerID int, playerGives []TradeLine, topUp Coins) (Decision, error) {
	o, ok := x.open[offerID]
	if !ok {
		return Decision{}, fmt.Errorf("%w: %d", ErrOfferNotFound, offerID)
	}
	if o.Origin != OriginNPC {
		return Decision{}, fmt.Errorf("%w: offer %d", ErrNotCounterable, offerID)
	}
	x.close(offerID)
	x.record(o, OutcomeCountered, nil)
	x.stats.PlayerProposals++
	return x.evaluate(o.NPC, playerGives, o.NPCGives, topUp, false)
}
