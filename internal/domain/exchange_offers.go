package domain

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// StartDay opens day: it lapses the offers whose last day has passed and resets the daily cap on
// the offers NPCs make. Call it before the first tick of each day.
func (x *Exchange) StartDay(day int) {
	x.day = day
	x.dayOffers = 0
	// Collect expired IDs before closing them: close modifies openIDs, so we must not iterate it
	// and close at the same time. openIDs is in creation order; IDs at the front expire first.
	var toExpire []int
	for _, id := range x.openIDs {
		if x.open[id].LastDay < day {
			toExpire = append(toExpire, id)
		}
	}
	for _, id := range toExpire {
		o := x.open[id]
		x.close(id)
		x.stats.Expired++
		x.record(o, OutcomeExpired, nil)
	}
}

// ProposeFromNPCs lets NPCs whose wish list matches a duplicate of the player propose a trade.
// Candidates are the NPCs that wish for a printing the player holds above DuplicateKeep copies. An
// NPC with an open offer waits, and the rest propose with their OfferChancePct.
//
// A duplicate is offered for once: copies already asked for by an open NPC offer, or by an offer
// made earlier in the same call, are not duplicates any more. Candidates are visited in a random
// order drawn from the crowd's stream, so the daily cap does not favour the low NPC IDs, which are
// the first archetypes by name. Call it once per tick, after the crowd's tick.
func (x *Exchange) ProposeFromNPCs() error {
	if x.dayOffers >= x.params.MaxOffersPerDay {
		return nil
	}
	duplicates, err := x.player.DuplicatesAbove(x.params.DuplicateKeep)
	if err != nil {
		return fmt.Errorf("exchange: player duplicates: %w", err)
	}
	for _, o := range x.open {
		if o.Origin == OriginNPC {
			duplicates = reserve(duplicates, o.PlayerGives)
		}
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
	ids := slices.Sorted(maps.Keys(candidates))
	x.crowd.rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	for _, id := range ids {
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
		if offer, ok := x.offerFrom(npc, duplicates); ok {
			duplicates = reserve(duplicates, offer.PlayerGives)
		}
	}
	return nil
}

// reserve takes the lines out of the duplicates and drops the duplicates left with no copy.
func reserve(duplicates []Surplus, lines []TradeLine) []Surplus {
	for _, l := range lines {
		for i := range duplicates {
			if duplicates[i].Key == l.Key {
				duplicates[i].Quantity -= l.Qty
			}
		}
	}
	return slices.DeleteFunc(duplicates, func(s Surplus) bool { return s.Quantity <= 0 })
}

// offerFrom asks npc's behaviour for an offer to the player and stores it when there is one.
func (x *Exchange) offerFrom(npc *NPC, duplicates []Surplus) (Offer, bool) {
	shelf := npc.Shelf()
	if len(shelf) == 0 {
		return Offer{}, false
	}
	want, give, ok := npc.Behaviour.Propose(ProposalContext{
		NPC: npc, Wanted: wantedLines(npc, duplicates), Shelf: shelf,
		PlayerCopies: x.player.Copies, MaxCards: x.params.MaxCardsPerSide,
	})
	if !ok {
		return Offer{}, false
	}
	offer := Offer{
		ID: x.newID(), NPC: npc.ID, Origin: OriginNPC, PlayerGives: want, NPCGives: give,
		CreatedDay: x.day, LastDay: x.day + x.params.ExpiryDays - 1,
	}
	x.store(offer)
	x.dayOffers++
	x.stats.NPCOffers++
	return offer, true
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
// When the NPC refuses on terms (ErrBelowMargin or ErrCounterVetoed), the NPC's decline streak
// is incremented with the original offer as context, because the player turned down the NPC's
// final word. A counter refused for any other reason (player error or stale state) leaves the
// streak unchanged.
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
	d, err := x.evaluate(o.NPC, playerGives, o.NPCGives, topUp, false)
	if err == nil && d.Outcome == OutcomeRefused &&
		(errors.Is(d.Reason, ErrBelowMargin) || errors.Is(d.Reason, ErrCounterVetoed)) {
		if npc, ok := x.crowd.byID[o.NPC]; ok {
			x.declined(npc, o)
		}
	}
	return d, err
}
