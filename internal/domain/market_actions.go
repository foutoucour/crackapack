package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// AskAround asks the NPCs that hold card on their shelf what they want for it. Each answers after
// the delay of its archetype, see Advance, with its asking price: the reference value of the card
// plus its margin, in coins. A request whose answers have all ended is replaced.
//
// It returns ErrAlreadyAsked while the player is still asking around for the card, and
// ErrNoHolder when no NPC that trades holds it, and then changes nothing.
func (m *Market) AskAround(card CardKey, now time.Time) (MarketID, error) {
	over := MarketID(0)
	for _, r := range m.requests {
		if r.Card != card {
			continue
		}
		if m.Pending(r.ID) > 0 || slices.ContainsFunc(m.answers, func(a MarketAnswer) bool { return a.To == r.ID }) {
			return 0, fmt.Errorf("%w: %s", ErrAlreadyAsked, card.ID)
		}
		over = r.ID
		break
	}
	var holders []*NPC
	for _, id := range m.crowd.HoldersOf(card) {
		if npc := m.crowd.byID[id]; npc.Behaviour != nil {
			holders = append(holders, npc)
		}
	}
	if len(holders) == 0 {
		return 0, fmt.Errorf("%w: %s", ErrNoHolder, card.ID)
	}
	if over != 0 {
		m.end(over)
	}
	id := m.newID()
	m.requests = append(m.requests, MarketRequest{ID: id, Card: card, At: now, Asked: len(holders)})
	m.expect(id, holders, now)
	return id, nil
}

// List puts one copy of card on the market at price, and reserves it. The NPCs that wish for the
// card, and those of the speculator behaviour that lack it, make an offer after the delay of their
// archetype, see Advance.
//
// It returns ErrInvalidPrice for a price below one coin, ErrAlreadyListed, ErrPlayerLacksCard and
// ErrReserved when every copy the player holds is held for the market already.
func (m *Market) List(card CardKey, price Coins, now time.Time) (MarketID, error) {
	if price < 1 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidPrice, price)
	}
	if slices.ContainsFunc(m.listings, func(l MarketListing) bool { return l.Card == card }) {
		return 0, fmt.Errorf("%w: %s", ErrAlreadyListed, card.ID)
	}
	if err := m.available(card, 1); err != nil {
		return 0, err
	}
	var buyers []*NPC
	for _, npc := range m.crowd.npcs {
		if m.wants(npc, card) {
			buyers = append(buyers, npc)
		}
	}
	id := m.newID()
	m.listings = append(m.listings, MarketListing{ID: id, Card: card, Price: price, At: now})
	m.expect(id, buyers, now)
	return id, nil
}

// Unlist takes the listing id off the market, with its offers, and frees its card.
func (m *Market) Unlist(id MarketID) error {
	if _, ok := m.listing(id); !ok {
		return fmt.Errorf("%w: listing %d", ErrMarketNotFound, id)
	}
	m.end(id)
	return nil
}

// Withdraw stops the request id, with its answers.
func (m *Market) Withdraw(id MarketID) error {
	if _, ok := m.request(id); !ok {
		return fmt.Errorf("%w: request %d", ErrMarketNotFound, id)
	}
	m.end(id)
	return nil
}

// Accept takes the answer id at once: the player pays the asking price and gets the NPC's card,
// or gives the listed card for the NPC's coins or cards. The request or the listing is then over,
// with its other answers. It returns the answer taken.
//
// It returns ErrMarketNotFound, ErrAwaitingReply for an answer the player countered, and
// ErrInsufficientBalance, leaving the answer open, when the player cannot pay. When the NPC no
// longer holds its cards the answer ends as failed, and it returns ErrNPCLacksCard.
func (m *Market) Accept(id MarketID, now time.Time) (MarketAnswer, error) {
	i, ok := m.answer(id)
	if !ok {
		return MarketAnswer{}, fmt.Errorf("%w: answer %d", ErrMarketNotFound, id)
	}
	a := m.answers[i]
	if a.Counter != nil {
		return MarketAnswer{}, fmt.Errorf("%w: answer %d", ErrAwaitingReply, id)
	}
	if err := m.settle(a, a.terms(), now); err != nil {
		return MarketAnswer{}, fmt.Errorf("market: accept answer %d: %w", id, err)
	}
	return cloneAnswer(a), nil
}

// Decline turns the answer id down: it ends, nothing moves, and the NPC is told of a decline, see
// ADR-0024. It returns ErrMarketNotFound and ErrAwaitingReply.
func (m *Market) Decline(id MarketID) error {
	i, ok := m.answer(id)
	if !ok {
		return fmt.Errorf("%w: answer %d", ErrMarketNotFound, id)
	}
	a := m.answers[i]
	if a.Counter != nil {
		return fmt.Errorf("%w: answer %d", ErrAwaitingReply, id)
	}
	m.dropAnswer(id)
	tellDecline(m.crowd.byID[a.NPC], a.offer())
	return nil
}

// Counter answers the answer id with the player's own terms, which the NPC accepts or refuses
// after the delay of its archetype, once, see Advance. On an asking price the player offers coins
// and cards for the NPC's card, and the cards are reserved meanwhile; on an offer the player asks
// for coins, with no card.
//
// It returns ErrMarketNotFound and ErrAwaitingReply; on an asking price ErrInvalidLine,
// ErrNothingOffered, ErrTooManyCards, ErrPlayerLacksCard, ErrReserved and ErrInsufficientBalance;
// on an offer ErrCoinsOnly and ErrInvalidPrice.
func (m *Market) Counter(id MarketID, coins Coins, cards []TradeLine, now time.Time) error {
	i, ok := m.answer(id)
	if !ok {
		return fmt.Errorf("%w: answer %d", ErrMarketNotFound, id)
	}
	a := &m.answers[i]
	if a.Counter != nil {
		return fmt.Errorf("%w: answer %d", ErrAwaitingReply, id)
	}
	var lines []TradeLine
	var err error
	if a.Kind == AnswerOffer {
		err = checkCoinsOnly(coins, cards)
	} else {
		lines, err = m.checkCounter(coins, cards)
	}
	if err != nil {
		return fmt.Errorf("market: counter answer %d: %w", id, err)
	}
	a.Counter = &MarketCounter{Coins: coins, Cards: lines, ReplyAt: now.Add(m.crowd.byID[a.NPC].ResponseDelay)}
	return nil
}

func checkCoinsOnly(coins Coins, cards []TradeLine) error {
	switch {
	case len(cards) > 0:
		return ErrCoinsOnly
	case coins < 1:
		return fmt.Errorf("%w: %d", ErrInvalidPrice, coins)
	}
	return nil
}

// checkCounter checks the player can give coins and cards, which are not held for the market, and
// returns the cards one line per card.
func (m *Market) checkCounter(coins Coins, cards []TradeLine) ([]TradeLine, error) {
	if coins < 0 {
		return nil, fmt.Errorf("%w: coins %d", ErrInvalidLine, coins)
	}
	lines, err := normalizeLines(cards)
	if err != nil {
		return nil, err
	}
	switch {
	case len(lines) == 0 && coins == 0:
		return nil, ErrNothingOffered
	case cardCount(lines) > m.params.MaxCardsPerSide:
		return nil, fmt.Errorf("%w: at most %d", ErrTooManyCards, m.params.MaxCardsPerSide)
	case coins > m.journal.Balance(AccountPlayer):
		return nil, fmt.Errorf("%w: %d coins", ErrInsufficientBalance, coins)
	}
	for _, l := range lines {
		if err := m.available(l.Key, l.Qty); err != nil {
			return nil, err
		}
	}
	return lines, nil
}

// available checks that the player holds qty copies of key that the market does not hold already.
func (m *Market) available(key CardKey, qty int) error {
	have := m.player.Quantity(key)
	switch {
	case have < qty:
		return fmt.Errorf("%w: %s x%d, the player holds %d", ErrPlayerLacksCard, key.ID, qty, have)
	case have-m.Reserved(key) < qty:
		return fmt.Errorf("%w: %s x%d", ErrReserved, key.ID, qty)
	}
	return nil
}

// wants reports whether npc makes an offer for card: it trades, and wishes for more copies of it,
// or speculates and lacks it.
func (m *Market) wants(npc *NPC, card CardKey) bool {
	if npc.Behaviour == nil {
		return false
	}
	held := npc.collection.Copies(card.ID)
	if npc.speculates && held == 0 {
		return true
	}
	return slices.ContainsFunc(npc.wishes, func(w Wish) bool { return w.ID == card.ID && w.Want > held })
}

// expect schedules the answer of each of npcs to id, after its delay from now.
func (m *Market) expect(id MarketID, npcs []*NPC, now time.Time) {
	for _, npc := range npcs {
		m.pending = append(m.pending, MarketPending{To: id, NPC: npc.ID, Due: now.Add(npc.ResponseDelay)})
	}
}

// deal is what each side of a trade gives.
type deal struct {
	playerGives, npcGives []TradeLine
	playerPays, npcPays   Coins
}

// terms is the deal of answer a as the NPC made it.
func (a MarketAnswer) terms() deal {
	if a.Kind == AnswerAsk {
		return deal{npcGives: lineOf(a.Card), playerPays: a.Coins}
	}
	return deal{playerGives: lineOf(a.Card), npcGives: a.Cards, npcPays: a.Coins}
}

// countered is the deal of the player's counter on answer a.
func (a MarketAnswer) countered() deal {
	if a.Kind == AnswerAsk {
		return deal{playerGives: a.Counter.Cards, npcGives: lineOf(a.Card), playerPays: a.Counter.Coins}
	}
	return deal{playerGives: lineOf(a.Card), npcPays: a.Counter.Coins}
}

// settle trades d on answer a and books it: an accepted trade ends the request or the listing, a
// trade that cannot go through because a side lacks its cards ends the answer as failed.
// ErrInsufficientBalance leaves all as it was. Any error is returned.
func (m *Market) settle(a MarketAnswer, d deal, at time.Time) error {
	npc := m.crowd.byID[a.NPC]
	err := m.trade(npc, d)
	switch {
	case err == nil:
		npc.declineStreak = 0
		m.record(a, MarketAccepted, at)
		m.end(a.To)
	case errors.Is(err, ErrPlayerLacksCard), errors.Is(err, ErrNPCLacksCard):
		m.dropAnswer(a.ID)
		m.record(a, MarketFailed, at)
	}
	return err
}

// trade moves the cards and coins of a deal atomically: it checks both sides hold what they give,
// posts what the player pays, the only step that can still fail, or what the NPC pays, which the
// market's account issues since NPCs hold no coins, and moves the cards.
func (m *Market) trade(npc *NPC, d deal) error {
	if err := holdingsError(m.player, npc, d.playerGives, d.npcGives); err != nil {
		return err
	}
	memo := fmt.Sprintf("market: NPC %d", npc.ID)
	if d.playerPays > 0 {
		if err := m.journal.Post(Entry{From: AccountPlayer, To: AccountTradeTopUp, Amount: d.playerPays, Memo: memo}); err != nil {
			return err
		}
	}
	if d.npcPays > 0 {
		if err := m.journal.Post(Entry{From: AccountMarketSale, To: AccountPlayer, Amount: d.npcPays, Memo: memo}); err != nil {
			return err
		}
	}
	return m.crowd.swapWithPlayer(m.player, npc, d.playerGives, d.npcGives)
}

// offer is answer a as the Exchange's rules read an NPC's own proposal.
func (a MarketAnswer) offer() Offer {
	if a.Kind == AnswerAsk {
		return Offer{NPC: a.NPC, Origin: OriginNPC, NPCGives: lineOf(a.Card), TopUp: a.Coins}
	}
	return Offer{NPC: a.NPC, Origin: OriginNPC, PlayerGives: lineOf(a.Card), NPCGives: a.Cards}
}

func lineOf(key CardKey) []TradeLine { return []TradeLine{{Key: key, Qty: 1}} }
