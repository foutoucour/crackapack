package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// marketEvent is something the market does at an instant: an NPC answers, an NPC replies to a
// counter, or an answer lapses. At one instant they happen in that order, each kind in the order
// it was made, so that the market does the same whenever Advance is called.
type marketEvent struct {
	at    time.Time
	kind  int
	index int
}

const (
	eventAnswer = iota
	eventReply
	eventExpiry
)

func (e marketEvent) before(o marketEvent) bool {
	if !e.at.Equal(o.at) {
		return e.at.Before(o.at)
	}
	if e.kind != o.kind {
		return e.kind < o.kind
	}
	return e.index < o.index
}

// Advance delivers what is due by now, in the order it fell due: the answers of the NPCs, their
// replies to the player's counters, and the lapse of the answers left open. The NPCs answer as they
// stand when Advance is called, so a caller that also moves the crowd interleaves the two in time.
// An error is a failure of the market itself.
func (m *Market) Advance(now time.Time) error {
	for {
		e, ok := m.next(now)
		if !ok {
			return nil
		}
		var err error
		switch e.kind {
		case eventAnswer:
			m.deliver(e.index)
		case eventReply:
			err = m.reply(e.index)
		case eventExpiry:
			a := m.answers[e.index]
			m.dropAnswer(a.ID)
			m.record(a, MarketExpired, a.Expires)
		}
		if err != nil {
			return err
		}
	}
}

// next returns the first event due by now.
func (m *Market) next(now time.Time) (marketEvent, bool) {
	var first marketEvent
	found := false
	consider := func(e marketEvent) {
		if !e.at.After(now) && (!found || e.before(first)) {
			first, found = e, true
		}
	}
	for i, p := range m.pending {
		consider(marketEvent{at: p.Due, kind: eventAnswer, index: i})
	}
	for i, a := range m.answers {
		if a.Counter != nil {
			consider(marketEvent{at: a.Counter.ReplyAt, kind: eventReply, index: i})
		} else {
			consider(marketEvent{at: a.Expires, kind: eventExpiry, index: i})
		}
	}
	return first, found
}

// deliver makes the answer of the pending entry i, if its NPC still has one to give.
func (m *Market) deliver(i int) {
	p := m.pending[i]
	m.pending = slices.Delete(m.pending, i, i+1)
	npc := m.crowd.byID[p.NPC]
	a := MarketAnswer{To: p.To, NPC: p.NPC, At: p.Due, Expires: p.Due.Add(m.params.Expiry)}
	if r, ok := m.request(p.To); ok {
		a.Kind, a.Card = AnswerAsk, m.requests[r].Card
		if npc.ShelfQuantity(a.Card) == 0 {
			return
		}
		a.Coins = max(1, requiredTopUp(0, m.valuer.Price(a.Card.ID), npcMargin(npc, a.offer(), m.crowd.tick), m.params.CoinRate))
	} else if l, ok := m.listing(p.To); ok {
		listing := m.listings[l]
		a.Kind, a.Card = AnswerOffer, listing.Card
		if !m.wants(npc, a.Card) {
			return
		}
		if a.Cards = m.cardsFor(npc, a.Card); len(a.Cards) == 0 {
			a.Coins = min(listing.Price, m.coinsFor(npc, a.offer()))
			if a.Coins < 1 {
				return
			}
		}
	} else {
		return
	}
	a.ID = m.newID()
	m.answers = append(m.answers, a)
}

// cardsFor is what npc gives from its shelf for card: what its behaviour proposes, kept to the
// cards the player lacks, which make an offer worth having. It is nil when there is none.
func (m *Market) cardsFor(npc *NPC, card CardKey) []TradeLine {
	_, give, ok := npc.Behaviour.Propose(ProposalContext{
		NPC: npc, Wanted: lineOf(card), Shelf: npc.Shelf(), PlayerCopies: m.player.Copies, MaxCards: m.params.MaxCardsPerSide,
	})
	if !ok {
		return nil
	}
	if give = slices.DeleteFunc(give, func(l TradeLine) bool { return m.player.Copies(l.Key.ID) > 0 }); len(give) == 0 {
		return nil
	}
	return give
}

// coinsFor is the most coins npc pays for the cards the player gives in o: their reference value
// less its margin, rounded down, so that it never gives more value than it gets.
func (m *Market) coinsFor(npc *NPC, o Offer) Coins {
	cents := int64(linesValue(m.valuer, o.PlayerGives)) * int64(m.params.CoinRate)
	return Coins(cents / (100 + int64(npcMargin(npc, o, m.crowd.tick))))
}

// reply is the NPC's answer to the counter on the answer i: it accepts terms that meet its
// acceptance rule and that no trait of it vetoes, and refuses the others, which is a decline of its
// own answer, see ADR-0024. An accepted counter trades at once.
func (m *Market) reply(i int) error {
	a := m.answers[i]
	c := *a.Counter
	npc := m.crowd.byID[a.NPC]
	terms := Offer{NPC: a.NPC, Origin: OriginNPC, PlayerGives: c.Cards, NPCGives: lineOf(a.Card), TopUp: c.Coins}
	if a.Kind == AnswerOffer {
		terms = Offer{NPC: a.NPC, Origin: OriginNPC, PlayerGives: lineOf(a.Card)}
	}
	var fair bool
	switch a.Kind {
	case AnswerAsk:
		fair = c.Coins >= requiredTopUp(linesValue(m.valuer, c.Cards), m.valuer.Price(a.Card.ID), npcMargin(npc, terms, m.crowd.tick), m.params.CoinRate)
	case AnswerOffer:
		fair = c.Coins <= m.coinsFor(npc, terms)
	}
	if !fair || vetoesCounter(npc, terms) {
		m.dropAnswer(a.ID)
		m.record(a, MarketRefused, c.ReplyAt)
		tellDecline(npc, a.offer())
		return nil
	}
	err := m.settle(a, a.countered(), c.ReplyAt)
	if errors.Is(err, ErrInsufficientBalance) {
		m.dropAnswer(a.ID)
		m.record(a, MarketFailed, c.ReplyAt)
		return nil
	}
	if err != nil && !errors.Is(err, ErrPlayerLacksCard) && !errors.Is(err, ErrNPCLacksCard) {
		return fmt.Errorf("market: reply to the counter on answer %d: %w", a.ID, err)
	}
	return nil
}
