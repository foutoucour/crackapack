package domain

import (
	"errors"
	"fmt"
)

// Propose offers the NPC npcID a swap: the player gives playerGives and topUp coins, the NPC gives
// npcGives from its shelf.
//
// The NPC accepts when it receives at least the reference value it gives plus its margin, and the
// trade executes at once. Otherwise it counters once, with the same cards and the smallest top-up
// that satisfies it: the counter is an open offer the player can Accept. A malformed or
// impossible offer is refused, with the reason in the Decision. The error is for failures of the
// Exchange itself, which abort a run.
func (x *Exchange) Propose(npcID int, playerGives, npcGives []TradeLine, topUp Coins) (Decision, error) {
	x.stats.PlayerProposals++
	return x.evaluate(npcID, playerGives, npcGives, topUp, true)
}

// Accept executes the open offer offerID, as the player's answer to an NPC's offer or counter.
//
// It returns ErrOfferNotFound for an offer that is not open, and ErrInsufficientBalance, leaving
// the offer open, when the player cannot pay the top-up. When what the offer needs is gone, the NPC
// traded or sold the card, or the player sold its own, the offer is closed and refused.
func (x *Exchange) Accept(offerID int) (Decision, error) {
	offer, ok := x.open[offerID]
	if !ok {
		return Decision{}, fmt.Errorf("%w: %d", ErrOfferNotFound, offerID)
	}
	err := x.execute(x.crowd.byID[offer.NPC], offer)
	switch {
	case err == nil:
		x.close(offerID)
		return x.accepted(offer), nil
	case errors.Is(err, ErrInsufficientBalance):
		return Decision{}, err
	case errors.Is(err, ErrPlayerLacksCard), errors.Is(err, ErrNPCLacksCard):
		x.close(offerID)
		return x.refuse(offer, err), nil
	}
	return Decision{}, fmt.Errorf("exchange: accept offer %d: %w", offerID, err)
}

// evaluate prices an offer and either executes it, counters it or refuses it. When canCounter is
// false a short offer is refused instead of countered.
func (x *Exchange) evaluate(npcID int, playerGives, npcGives []TradeLine, topUp Coins, canCounter bool) (Decision, error) {
	npc, offer, err := x.prepare(npcID, playerGives, npcGives, topUp)
	if err != nil {
		return x.refuse(offer, err), nil
	}
	need := requiredTopUp(linesValue(x.valuer, offer.PlayerGives), linesValue(x.valuer, offer.NPCGives),
		npc.Behaviour.Margin(offer), x.params.CoinRate)
	if topUp < need {
		if !canCounter {
			return x.refuse(offer, ErrBelowMargin), nil
		}
		return x.counterWith(offer, need), nil
	}
	err = x.execute(npc, offer)
	switch {
	case errors.Is(err, ErrInsufficientBalance):
		return x.refuse(offer, err), nil
	case err != nil:
		return Decision{}, fmt.Errorf("exchange: trade with NPC %d: %w", npcID, err)
	}
	return x.accepted(offer), nil
}

// prepare validates an offer and returns the NPC and the offer with its lines normalised. On an
// error the returned Offer is still filled in as far as it got, for the history.
func (x *Exchange) prepare(npcID int, playerGives, npcGives []TradeLine, topUp Coins) (*NPC, Offer, error) {
	offer := Offer{NPC: npcID, Origin: OriginPlayer, TopUp: topUp, CreatedDay: x.day, LastDay: x.day}
	npc, ok := x.crowd.byID[npcID]
	if !ok {
		return nil, offer, fmt.Errorf("%w: %d", ErrUnknownNPC, npcID)
	}
	if npc.Behaviour == nil {
		return nil, offer, fmt.Errorf("%w: NPC %d", ErrNPCNotTrading, npcID)
	}
	var err error
	if offer.PlayerGives, err = normalizeLines(playerGives); err != nil {
		return nil, offer, err
	}
	if offer.NPCGives, err = normalizeLines(npcGives); err != nil {
		return nil, offer, err
	}
	switch {
	case topUp < 0:
		return nil, offer, fmt.Errorf("%w: top-up is %d", ErrInvalidLine, topUp)
	case len(offer.NPCGives) == 0:
		return nil, offer, ErrEmptyOffer
	case len(offer.PlayerGives) == 0 && topUp == 0:
		return nil, offer, ErrNothingOffered
	case cardCount(offer.PlayerGives) > x.params.MaxCardsPerSide || cardCount(offer.NPCGives) > x.params.MaxCardsPerSide:
		return nil, offer, fmt.Errorf("%w: at most %d", ErrTooManyCards, x.params.MaxCardsPerSide)
	}
	if err := x.holdingsError(npc, offer); err != nil {
		return nil, offer, err
	}
	return npc, offer, nil
}

// holdingsError checks that the player holds what it gives and the NPC's shelf holds what it gives.
func (x *Exchange) holdingsError(npc *NPC, o Offer) error {
	for _, l := range o.PlayerGives {
		if x.player.Quantity(l.Key) < l.Qty {
			return fmt.Errorf("%w: %s x%d", ErrPlayerLacksCard, l.Key.ID, l.Qty)
		}
	}
	for _, l := range o.NPCGives {
		if npc.ShelfQuantity(l.Key) < l.Qty {
			return fmt.Errorf("%w: %s x%d", ErrNPCLacksCard, l.Key.ID, l.Qty)
		}
	}
	return nil
}

// execute moves the cards and the top-up of o atomically: it checks both sides hold what they
// give, posts the top-up, which is the only step that can still fail, and then moves the cards.
func (x *Exchange) execute(npc *NPC, o Offer) error {
	if err := x.holdingsError(npc, o); err != nil {
		return err
	}
	if o.TopUp > 0 {
		if err := x.journal.Post(Entry{
			From: AccountPlayer, To: AccountTradeTopUp, Amount: o.TopUp,
			Memo: fmt.Sprintf("trade top-up: NPC %d", npc.ID),
		}); err != nil {
			return err
		}
	}
	for _, l := range o.PlayerGives {
		if err := x.player.Remove(l.Key, l.Qty); err != nil {
			return err
		}
	}
	for _, l := range o.NPCGives {
		if err := x.player.Add(l.Key, l.Qty); err != nil {
			return err
		}
	}
	return x.crowd.applyTrade(npc, o.NPCGives, o.PlayerGives)
}

// accepted books an executed offer and returns the Decision.
func (x *Exchange) accepted(o Offer) Decision {
	if o.ID == 0 {
		o.ID = x.newID()
	}
	x.stats.Accepted++
	x.stats.TopUpCoins += o.TopUp
	x.stats.VolumeCoins += x.coinsOf(linesValue(x.valuer, o.PlayerGives) + linesValue(x.valuer, o.NPCGives))
	x.record(o, OutcomeAccepted, nil)
	return Decision{Outcome: OutcomeAccepted, Offer: o}
}

// refuse books a refusal and returns the Decision.
func (x *Exchange) refuse(o Offer, reason error) Decision {
	x.stats.Refused++
	x.record(o, OutcomeRefused, reason)
	return Decision{Outcome: OutcomeRefused, Reason: reason}
}

// counterWith stores the NPC's counter-offer to o: the same cards with the top-up need.
func (x *Exchange) counterWith(o Offer, need Coins) Decision {
	o.ID = x.newID()
	o.Origin = OriginNPCCounter
	o.TopUp = need
	o.CreatedDay = x.day
	o.LastDay = x.day + x.params.ExpiryDays - 1
	x.store(o)
	x.stats.Countered++
	x.record(o, OutcomeCountered, nil)
	return Decision{Outcome: OutcomeCountered, Offer: o}
}

func (x *Exchange) newID() int {
	x.nextID++
	return x.nextID
}

func (x *Exchange) store(o Offer) {
	x.open[o.ID] = o
	x.openCount[o.NPC]++
}

func (x *Exchange) close(id int) {
	o, ok := x.open[id]
	if !ok {
		return
	}
	delete(x.open, id)
	x.openCount[o.NPC]--
	if x.openCount[o.NPC] <= 0 {
		delete(x.openCount, o.NPC)
	}
}

// record appends to the bounded history, dropping the oldest entries beyond HistorySize.
func (x *Exchange) record(o Offer, outcome Outcome, reason error) {
	if x.params.HistorySize == 0 {
		return
	}
	x.history = append(x.history, HistoryEntry{Day: x.day, Offer: o, Outcome: outcome, Reason: reason})
	if over := len(x.history) - x.params.HistorySize; over > 0 {
		x.history = append(x.history[:0:0], x.history[over:]...)
	}
}

// coinsOf converts a value in USD cents to whole coins at the coin rate.
func (x *Exchange) coinsOf(cents USDCents) Coins {
	return Coins(int64(cents) * int64(x.params.CoinRate) / 100)
}
