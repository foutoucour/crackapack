package play

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

var (
	// ErrNoMarket is returned by every market action of a game without a market.
	ErrNoMarket = errors.New("play: the game has no market")
	// ErrNobodyHasIt is returned by AskAround when no NPC has the card on its shelf.
	ErrNobodyHasIt = errors.New("play: no NPC has the card to trade")
	// ErrAlreadyAsked is returned by AskAround while the player is still asking around for the card.
	ErrAlreadyAsked = errors.New("play: already asking around for the card")
	// ErrAlreadyListed is returned by List for a card on the market already.
	ErrAlreadyListed = errors.New("play: the card is on the market already")
	// ErrReserved is returned when the copies of a card the player would give or sell are held for
	// a listing or a counter.
	ErrReserved = errors.New("play: the copies are held for the market")
	// ErrNotHeld is returned when the player lists or gives a card it does not hold.
	ErrNotHeld = errors.New("play: the player does not hold the card")
	// ErrInvalidPrice is returned for an asking price, or the coins asked in a counter, below a coin.
	ErrInvalidPrice = errors.New("play: a price is at least one coin")
	// ErrCoinsOnly is returned for cards added to a counter on an offer, which asks for coins.
	ErrCoinsOnly = errors.New("play: a counter on an offer asks for coins only")
	// ErrNothingOffered is returned for a counter on an asking price with no coin and no card.
	ErrNothingOffered = errors.New("play: a counter offers coins or cards")
	// ErrNegative is returned for a counter with a negative number of coins or of copies.
	ErrNegative = errors.New("play: a counter gives no negative coins or copies")
	// ErrTooManyCards is returned for a counter with more cards than a trade allows.
	ErrTooManyCards = errors.New("play: too many cards in the counter")
	// ErrAwaitingReply is returned for an answer the player countered, until the NPC replies.
	ErrAwaitingReply = errors.New("play: the NPC has not replied to the counter yet")
	// ErrGone is returned when the NPC no longer has the cards of its answer, which then ends.
	ErrGone = errors.New("play: the NPC no longer has the cards")
	// ErrNotOnMarket is returned for a request, a listing or an answer that is not on the market:
	// it never was, or it is over.
	ErrNotOnMarket = errors.New("play: not on the market")
)

// MarketView is what the player sees of the market: what it asked for and listed, the answers of
// the NPCs, and how the latest answers ended. Like a marketplace, it shows who answered and their
// price, the NPCs still to answer a request but not who they are, and nothing of what an NPC holds
// or wants beyond its answer.
type MarketView struct {
	// Requests and Listings are newest first.
	Requests []Request
	Listings []Listing
	// Results are the latest answers that ended, newest first.
	Results []Result
	// Waiting is the answers waiting for the player: open, and not countered.
	Waiting int
	// Now is the time the view describes, by the game's clock, to count the time to a reply or a
	// lapse from.
	Now time.Time
}

// Request is the player asking around for a card.
type Request struct {
	ID     domain.MarketID
	Card   domain.Card
	Finish domain.Finish
	At     time.Time
	// Asked is the NPCs that had the card when the player asked, and Pending those of them that
	// have not answered yet.
	Asked, Pending int
	// Answers are the asking prices, oldest first.
	Answers []Answer
}

// Listing is a card the player put on the market.
type Listing struct {
	ID     domain.MarketID
	Card   domain.Card
	Finish domain.Finish
	Price  domain.Coins
	At     time.Time
	// Answers are the offers, oldest first.
	Answers []Answer
}

// Answer is an NPC's answer: to a request, the coins it asks for its card; to a listing, the coins
// it pays or the cards it gives.
type Answer struct {
	ID domain.MarketID
	// NPC is who answered, and Archetype its personality.
	NPC       int
	Archetype string
	Coins     domain.Coins
	Cards     []CardLine
	At        time.Time
	// Expires is when the answer lapses, unless the player countered it.
	Expires time.Time
	// Counter is the player's counter, waiting for the NPC's reply, nil when none.
	Counter *CounterTerms
}

// CounterTerms is the player's counter: on an asking price the coins and cards the player gives,
// on an offer the coins it asks for.
type CounterTerms struct {
	Coins   domain.Coins
	Cards   []CardLine
	ReplyAt time.Time
}

// CardLine is copies of a card in a finish.
type CardLine struct {
	Card   domain.Card
	Finish domain.Finish
	Qty    int
}

// Result is how an answer to a request or a listing of Card ended.
type Result struct {
	Card    domain.Card
	Finish  domain.Finish
	Kind    domain.AnswerKind
	Answer  Answer
	Outcome domain.MarketOutcome
	At      time.Time
}

// Deal is what accepting an answer did: the sets the cards it brought unlocked.
type Deal struct {
	Unlocked []UnlockedSet
}

// Market returns what the player sees of the market, once the answers due are delivered. It
// returns ErrNoMarket in a game without one.
func (g *Game) Market() (MarketView, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, err := g.syncMarket()
	if err != nil {
		return MarketView{}, err
	}
	v := MarketView{Now: g.now()}
	answers := m.Answers()
	for _, r := range slices.Backward(m.Requests()) {
		card := g.cards[r.Card.ID]
		v.Requests = append(v.Requests, Request{
			ID: r.ID, Card: card, Finish: r.Card.Finish, At: r.At, Asked: r.Asked, Pending: m.Pending(r.ID),
			Answers: g.answersTo(r.ID, answers),
		})
	}
	for _, l := range slices.Backward(m.Listings()) {
		v.Listings = append(v.Listings, Listing{
			ID: l.ID, Card: g.cards[l.Card.ID], Finish: l.Card.Finish, Price: l.Price, At: l.At,
			Answers: g.answersTo(l.ID, answers),
		})
	}
	for _, r := range slices.Backward(m.Results()) {
		v.Results = append(v.Results, Result{
			Card: g.cards[r.Answer.Card.ID], Finish: r.Answer.Card.Finish, Kind: r.Answer.Kind,
			Answer: g.answerView(r.Answer), Outcome: r.Outcome, At: r.At,
		})
	}
	for _, a := range answers {
		if a.Counter == nil {
			v.Waiting++
		}
	}
	return v, nil
}

// AskAround asks the NPCs that have the card id in finish what they want for it. They answer
// after the delay of their personality, see Market. It returns ErrUnknownCard, ErrLocked when no
// unlocked set's binder holds the card, ErrAlreadyAsked, ErrNobodyHasIt or ErrNoMarket, wrapped,
// and then changes nothing.
func (g *Game) AskAround(id domain.CardID, finish domain.Finish) (domain.MarketID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, key, err := g.marketFor(id, finish)
	if err != nil {
		return 0, fmt.Errorf("ask around for %q: %w", id, err)
	}
	if !g.buyable(id) {
		return 0, fmt.Errorf("ask around for %q: %w", id, ErrLocked)
	}
	rid, err := m.AskAround(key, g.now())
	if err != nil {
		return 0, fmt.Errorf("ask around for %q: %w", id, marketError(err))
	}
	if err := g.deliver(); err != nil {
		return rid, fmt.Errorf("ask around for %q: %w", id, err)
	}
	return rid, nil
}

// Withdraw stops the request id. It returns ErrNotOnMarket or ErrNoMarket, wrapped.
func (g *Game) Withdraw(id domain.MarketID) error {
	return g.marketAction(fmt.Sprintf("withdraw request %d", id), func(m *domain.Market) error { return m.Withdraw(id) })
}

// List puts a copy of the card id in finish on the market at price, and reserves it until it is
// sold or unlisted. It returns ErrUnknownCard, ErrInvalidPrice, ErrAlreadyListed, ErrNotHeld,
// ErrReserved or ErrNoMarket, wrapped, and then changes nothing.
func (g *Game) List(id domain.CardID, finish domain.Finish, price domain.Coins) (domain.MarketID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, key, err := g.marketFor(id, finish)
	if err != nil {
		return 0, fmt.Errorf("list %q: %w", id, err)
	}
	lid, err := m.List(key, price, g.now())
	if err != nil {
		return 0, fmt.Errorf("list %q: %w", id, marketError(err))
	}
	if err := g.deliver(); err != nil {
		return lid, fmt.Errorf("list %q: %w", id, err)
	}
	return lid, nil
}

// Unlist takes the listing id off the market, and frees its card. It returns ErrNotOnMarket or
// ErrNoMarket, wrapped.
func (g *Game) Unlist(id domain.MarketID) error {
	return g.marketAction(fmt.Sprintf("unlist %d", id), func(m *domain.Market) error { return m.Unlist(id) })
}

// AcceptAnswer takes the answer id at once: the player pays the asking price and gets the card,
// or gives the listed card for the NPC's coins or cards. It returns the sets the cards unlocked. It
// returns ErrNotOnMarket, ErrAwaitingReply, ErrNotEnoughCoins, ErrGone or ErrNoMarket, wrapped, and
// then changes nothing, but for ErrGone, which ends the answer.
func (g *Game) AcceptAnswer(id domain.MarketID) (Deal, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, err := g.syncMarket()
	if err != nil {
		return Deal{}, fmt.Errorf("accept answer %d: %w", id, err)
	}
	if _, err := m.Accept(id, g.now()); err != nil {
		err = marketError(err)
		if errors.Is(err, ErrGone) {
			g.save()
		}
		return Deal{}, fmt.Errorf("accept answer %d: %w", id, err)
	}
	d := Deal{Unlocked: g.unlockEarned()}
	g.save()
	return d, nil
}

// DeclineAnswer turns the answer id down. It returns ErrNotOnMarket, ErrAwaitingReply or
// ErrNoMarket, wrapped.
func (g *Game) DeclineAnswer(id domain.MarketID) error {
	return g.marketAction(fmt.Sprintf("decline answer %d", id), func(m *domain.Market) error { return m.Decline(id) })
}

// CounterAnswer answers the answer id with the player's terms, which the NPC accepts or refuses
// after its delay, once. On an asking price the player offers coins and cards, which are reserved
// until the reply; on an offer it asks for coins, with no card. It returns ErrNotOnMarket,
// ErrAwaitingReply, ErrNothingOffered, ErrNegative, ErrTooManyCards, ErrNotHeld, ErrReserved, ErrNotEnoughCoins,
// ErrCoinsOnly, ErrInvalidPrice or ErrNoMarket, wrapped, and then changes nothing.
func (g *Game) CounterAnswer(id domain.MarketID, coins domain.Coins, cards []domain.TradeLine) error {
	return g.marketAction(fmt.Sprintf("counter answer %d", id), func(m *domain.Market) error {
		return m.Counter(id, coins, cards, g.now())
	})
}

// Reserved returns the copies of the card id in finish that the market holds for a listing or a
// counter, which the Counter does not buy meanwhile. It is 0 in a game without a market.
func (g *Game) Reserved(id domain.CardID, finish domain.Finish) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reserved(domain.CardKey{ID: id, Finish: finish})
}

// OnNPCShelf reports whether an NPC that trades holds the card id in finish, which is whether
// AskAround would find one. It is false in a game without a market. It answers for the market as
// it stands: call Market first to catch it up.
func (g *Game) OnNPCShelf(id domain.CardID, finish domain.Finish) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.economy == nil || g.economy.market == nil {
		return false
	}
	return g.economy.market.HasHolder(domain.CardKey{ID: id, Finish: finish})
}

// marketAction runs act on the market, caught up, and saves the game when it went through. g.mu is
// taken.
func (g *Game) marketAction(what string, act func(*domain.Market) error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	m, err := g.syncMarket()
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if err := act(m); err != nil {
		return fmt.Errorf("%s: %w", what, marketError(err))
	}
	if err := g.deliver(); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

// deliver hands out what is due now, such as the answer of an NPC that answers at once, with the
// sets its trades unlock, and saves the game, which the action before it changed whatever the
// market does. An error is a failure of the market itself. g.mu must be held, and the market
// caught up.
func (g *Game) deliver() error {
	err := g.economy.advanceMarket(g.now())
	g.unlockEarned()
	g.save()
	return err
}

// syncMarket returns the game's market, caught up with the time gone by, with the sets that its
// trades unlocked. g.mu must be held.
func (g *Game) syncMarket() (*domain.Market, error) {
	if g.economy == nil || g.economy.market == nil {
		return nil, ErrNoMarket
	}
	if err := g.economy.catchUp(); err != nil {
		return nil, err
	}
	g.unlockCaughtUp()
	return g.economy.market, nil
}

// marketFor returns the market, caught up, and the key of the card id in finish. It returns
// ErrUnknownCard and ErrNoMarket. g.mu must be held.
func (g *Game) marketFor(id domain.CardID, finish domain.Finish) (*domain.Market, domain.CardKey, error) {
	key := domain.CardKey{ID: id, Finish: finish}
	card, ok := g.cards[id]
	if !ok || !slices.Contains(card.Finishes, finish) {
		return nil, key, ErrUnknownCard
	}
	m, err := g.syncMarket()
	return m, key, err
}

// reserved is the copies of key the market holds. g.mu must be held.
func (g *Game) reserved(key domain.CardKey) int {
	if g.economy == nil || g.economy.market == nil {
		return 0
	}
	return g.economy.market.Reserved(key)
}

// answersTo is the answers of answers to the request or listing id, as the player sees them.
func (g *Game) answersTo(id domain.MarketID, answers []domain.MarketAnswer) []Answer {
	var out []Answer
	for _, a := range answers {
		if a.To == id {
			out = append(out, g.answerView(a))
		}
	}
	return out
}

func (g *Game) answerView(a domain.MarketAnswer) Answer {
	out := Answer{ID: a.ID, NPC: a.NPC, Coins: a.Coins, Cards: g.cardLines(a.Cards), At: a.At, Expires: a.Expires}
	if npc, ok := g.economy.crowd.NPC(a.NPC); ok {
		out.Archetype = npc.Archetype
	}
	if c := a.Counter; c != nil {
		out.Counter = &CounterTerms{Coins: c.Coins, Cards: g.cardLines(c.Cards), ReplyAt: c.ReplyAt}
	}
	return out
}

func (g *Game) cardLines(lines []domain.TradeLine) []CardLine {
	var out []CardLine
	for _, l := range lines {
		out = append(out, CardLine{Card: g.cards[l.Key.ID], Finish: l.Key.Finish, Qty: l.Qty})
	}
	return out
}

// marketError is the play error for a refusal of the market.
func marketError(err error) error {
	for _, m := range []struct{ domain, play error }{
		{domain.ErrMarketNotFound, ErrNotOnMarket},
		{domain.ErrNoHolder, ErrNobodyHasIt},
		{domain.ErrAlreadyAsked, ErrAlreadyAsked},
		{domain.ErrAlreadyListed, ErrAlreadyListed},
		{domain.ErrReserved, ErrReserved},
		{domain.ErrLockedCopy, ErrCopyLocked},
		{domain.ErrPlayerLacksCard, ErrNotHeld},
		{domain.ErrInvalidPrice, ErrInvalidPrice},
		{domain.ErrCoinsOnly, ErrCoinsOnly},
		{domain.ErrNothingOffered, ErrNothingOffered},
		{domain.ErrInvalidLine, ErrNegative},
		{domain.ErrTooManyCards, ErrTooManyCards},
		{domain.ErrAwaitingReply, ErrAwaitingReply},
		{domain.ErrInsufficientBalance, ErrNotEnoughCoins},
		{domain.ErrNPCLacksCard, ErrGone},
	} {
		if errors.Is(err, m.domain) {
			return fmt.Errorf("%w: %v", m.play, err)
		}
	}
	return err
}
