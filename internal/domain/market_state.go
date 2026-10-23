package domain

import (
	"errors"
	"fmt"
	"slices"
)

// MarketState is everything a market holds, so that a save can keep it and Restore can bring it
// back: the player's requests and listings, the NPCs' answers, the answers to come and the latest
// results.
type MarketState struct {
	// NextID is the ID the next request, listing or answer gets.
	NextID   MarketID
	Requests []MarketRequest
	Listings []MarketListing
	Answers  []MarketAnswer
	Pending  []MarketPending
	Results  []MarketResult
}

// State returns what the market holds.
func (m *Market) State() MarketState {
	return MarketState{
		NextID:   m.nextID,
		Requests: m.Requests(),
		Listings: m.Listings(),
		Answers:  m.Answers(),
		Pending:  slices.Clone(m.pending),
		Results:  m.Results(),
	}
}

// Restore sets the market to s, which State returned, over the player's collection and the crowd
// as they stand. It returns an error, and changes nothing, when s cannot be the state of this
// market: an ID used twice or not below NextID, a card asked for or listed twice, an answer or an
// answer to come for nothing or from an NPC the crowd lacks, a price below a coin, a malformed
// trade line, or more copies reserved than the player holds. Only the latest results are kept.
func (m *Market) Restore(s MarketState) error {
	next := &Market{
		params: m.params, crowd: m.crowd, player: m.player, journal: m.journal, valuer: m.valuer,
		nextID:   s.NextID,
		requests: slices.Clone(s.Requests),
		listings: slices.Clone(s.Listings),
		pending:  slices.Clone(s.Pending),
	}
	for _, a := range s.Answers {
		next.answers = append(next.answers, cloneAnswer(a))
	}
	for _, r := range s.Results {
		next.record(r.Answer, r.Outcome, r.At)
	}
	if err := next.check(); err != nil {
		return fmt.Errorf("market: %w", err)
	}
	*m = *next
	return nil
}

// ForgetNPCs drops the answers of the NPCs and those to come, with the counters waiting for a
// reply, for a crowd made of other NPCs. The requests and listings stay.
func (m *Market) ForgetNPCs() {
	m.answers, m.pending = nil, nil
}

// check reports why m cannot be a market over its crowd and player.
func (m *Market) check() error {
	if m.nextID < 1 {
		return fmt.Errorf("next ID %d is below 1", m.nextID)
	}
	ids := make(map[MarketID]bool)
	useID := func(id MarketID) error {
		if id < 1 || id >= m.nextID || ids[id] {
			return fmt.Errorf("ID %d is used twice or is not in 1..%d", id, m.nextID-1)
		}
		ids[id] = true
		return nil
	}
	cards := make(map[CardKey]bool)
	for _, r := range m.requests {
		if err := errors.Join(useID(r.ID), once(cards, r.Card, "asked for"), nonNegative(r.Asked, "asked")); err != nil {
			return err
		}
	}
	clear(cards)
	for _, l := range m.listings {
		if err := errors.Join(useID(l.ID), once(cards, l.Card, "listed")); err != nil {
			return err
		}
		if l.Price < 1 {
			return fmt.Errorf("listing %d: %w: %d", l.ID, ErrInvalidPrice, l.Price)
		}
	}
	for _, a := range m.answers {
		if err := errors.Join(useID(a.ID), m.checkAnswer(a)); err != nil {
			return err
		}
	}
	for _, p := range m.pending {
		if !m.open(p.To) || m.crowd.byID[p.NPC] == nil {
			return fmt.Errorf("an answer to come from NPC %d to %d has no NPC or nothing to answer", p.NPC, p.To)
		}
	}
	for _, key := range m.reservedKeys() {
		if have, held := m.player.Quantity(key), m.Reserved(key); held > have {
			return fmt.Errorf("%s is reserved %d times and the player holds %d", key.ID, held, have)
		}
	}
	return nil
}

func (m *Market) checkAnswer(a MarketAnswer) error {
	var to CardKey
	if i, ok := m.request(a.To); ok && a.Kind == AnswerAsk && len(a.Cards) == 0 {
		to = m.requests[i].Card
	} else if i, ok := m.listing(a.To); ok && a.Kind == AnswerOffer {
		to = m.listings[i].Card
	} else {
		return fmt.Errorf("answer %d of kind %d answers nothing it can answer: %d", a.ID, a.Kind, a.To)
	}
	switch {
	case a.Card != to:
		return fmt.Errorf("answer %d is about %s, and what it answers about %s", a.ID, a.Card.ID, to.ID)
	case m.crowd.byID[a.NPC] == nil:
		return fmt.Errorf("answer %d comes from NPC %d, which the crowd lacks", a.ID, a.NPC)
	case a.Coins < 0:
		return fmt.Errorf("answer %d: %w: %d coins", a.ID, ErrInvalidPrice, a.Coins)
	}
	if err := checkLines(a.Cards); err != nil {
		return fmt.Errorf("answer %d: %w", a.ID, err)
	}
	if a.Counter != nil {
		if err := errors.Join(checkLines(a.Counter.Cards), nonNegative(int(a.Counter.Coins), "counter coins")); err != nil {
			return fmt.Errorf("answer %d: %w", a.ID, err)
		}
	}
	return nil
}

// open reports whether id is a request or a listing.
func (m *Market) open(id MarketID) bool {
	_, isRequest := m.request(id)
	_, isListing := m.listing(id)
	return isRequest || isListing
}

// reservedKeys is every card a listing or a counter holds.
func (m *Market) reservedKeys() []CardKey {
	var keys []CardKey
	for _, l := range m.listings {
		keys = append(keys, l.Card)
	}
	for _, a := range m.answers {
		if a.Counter != nil {
			for _, l := range a.Counter.Cards {
				keys = append(keys, l.Key)
			}
		}
	}
	return keys
}

func checkLines(lines []TradeLine) error {
	for _, l := range lines {
		if l.Qty <= 0 {
			return fmt.Errorf("%w: %s x%d", ErrInvalidLine, l.Key.ID, l.Qty)
		}
	}
	return nil
}

func once(seen map[CardKey]bool, key CardKey, what string) error {
	if seen[key] {
		return fmt.Errorf("%s is %s twice", key.ID, what)
	}
	seen[key] = true
	return nil
}

func nonNegative(n int, what string) error {
	if n < 0 {
		return fmt.Errorf("%s %d is negative", what, n)
	}
	return nil
}
