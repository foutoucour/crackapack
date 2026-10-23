package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

var (
	// ErrMarketNotFound is returned for a request, a listing or an answer the market does not have:
	// it never existed or it is over.
	ErrMarketNotFound = errors.New("market: no such request, listing or answer")
	// ErrNoHolder is returned by AskAround when no NPC holds the card on its shelf.
	ErrNoHolder = errors.New("market: no NPC has the card on its shelf")
	// ErrAlreadyAsked is returned by AskAround for a card the player is still asking around for.
	ErrAlreadyAsked = errors.New("market: the player is already asking around for the card")
	// ErrAlreadyListed is returned by List for a card the player has listed already.
	ErrAlreadyListed = errors.New("market: the card is already listed")
	// ErrReserved is returned when the player's copies of a card are held for a listing or a
	// counter, so that none is left to give.
	ErrReserved = errors.New("market: the copies are held for a listing or a counter")
	// ErrInvalidPrice is returned for an asking price, or the coins a counter asks for, below one
	// coin.
	ErrInvalidPrice = errors.New("market: a price is at least one coin")
	// ErrCoinsOnly is returned by Counter for cards added to a counter on an offer: the player
	// asks the NPC for coins instead of what it offered.
	ErrCoinsOnly = errors.New("market: a counter on an offer asks for coins, with no card")
	// ErrAwaitingReply is returned for an answer the player countered, until the NPC replies.
	ErrAwaitingReply = errors.New("market: the answer waits for the NPC's reply to the counter")
)

// MarketID identifies a request, a listing or an answer of a market. IDs start at 1 and grow.
type MarketID int

// AnswerKind says what an answer of an NPC answers.
type AnswerKind int

const (
	// AnswerAsk is an NPC's answer to a request: what it asks, in coins, for the card on its shelf.
	AnswerAsk AnswerKind = iota + 1
	// AnswerOffer is an NPC's offer for a listed card: coins, or cards from its shelf.
	AnswerOffer
)

// MarketOutcome is how an answer ended, other than the player declining it.
type MarketOutcome int

const (
	// MarketAccepted means the trade went through: the player accepted, or the NPC took a counter.
	MarketAccepted MarketOutcome = iota + 1
	// MarketRefused means the NPC refused the player's counter.
	MarketRefused
	// MarketExpired means the answer lapsed unanswered.
	MarketExpired
	// MarketFailed means the trade could not go through: the NPC no longer had its cards, or the
	// player could not pay the counter it made.
	MarketFailed
)

// MarketRequest is the player asking around for a card.
type MarketRequest struct {
	ID   MarketID
	Card CardKey
	At   time.Time
	// Asked is the NPCs asked: those that held the card on their shelf.
	Asked int
}

// MarketListing is a card the player put on the market at an asking price. One copy of it is
// reserved until it is sold or unlisted.
type MarketListing struct {
	ID    MarketID
	Card  CardKey
	Price Coins
	At    time.Time
}

// MarketAnswer is an NPC's answer to a request or to a listing, open until it lapses at Expires.
type MarketAnswer struct {
	ID MarketID
	// To is the request or the listing answered, and Card its card.
	To   MarketID
	Kind AnswerKind
	Card CardKey
	NPC  int
	// Coins is, to a request, the NPC's asking price for its card; to a listing, what it pays.
	Coins Coins
	// Cards is what the NPC gives for a listed card instead of coins.
	Cards       []TradeLine
	At, Expires time.Time
	// Counter is the player's counter waiting for the NPC's reply, nil when none.
	Counter *MarketCounter
}

// MarketCounter is the player's counter to an answer.
type MarketCounter struct {
	// Coins is, on an asking price, what the player pays; on an offer, what the player asks for.
	Coins Coins
	// Cards is what the player gives with the coins, on an asking price only.
	Cards []TradeLine
	// ReplyAt is the instant the NPC replies.
	ReplyAt time.Time
}

// MarketPending is an answer to come: NPC answers To at Due.
type MarketPending struct {
	To  MarketID
	NPC int
	Due time.Time
}

// MarketResult is how an answer ended, at At.
type MarketResult struct {
	Answer  MarketAnswer
	Outcome MarketOutcome
	At      time.Time
}

// marketResultsKept bounds the results a market remembers: the latest, for the player to read.
const marketResultsKept = 20

// MarketParams configures a Market. Every field comes from the configuration.
type MarketParams struct {
	// Expiry is how long an answer stays open once it arrives.
	Expiry time.Duration
	// MaxCardsPerSide bounds the cards each side of a trade gives, counting quantities.
	MaxCardsPerSide int
	// CoinRate is the coins per US dollar, to put coins and cards on one scale.
	CoinRate CoinRate
}

// Market is where the player trades with the NPCs of a crowd on its own initiative, see
// docs/core-features.md#trades-with-npcs: it asks around for a card and the NPCs that hold it ask a
// price, or it lists a card and the NPCs that want it make offers. Each NPC answers after the delay
// of its archetype, and replies once to a counter, by the acceptance rule of the Exchange. Time is
// given by the caller, which calls Advance to deliver what is due. It is not safe for concurrent
// use.
type Market struct {
	params  MarketParams
	crowd   *Crowd
	player  *Collection
	journal *Journal
	valuer  Valuer

	nextID   MarketID
	requests []MarketRequest
	listings []MarketListing
	// answers are open or wait for a reply, in creation order.
	answers []MarketAnswer
	pending []MarketPending
	// results are the latest results, oldest first.
	results []MarketResult
}

// NewMarket builds a Market between player and the NPCs of crowd. The player's coins are posted to
// journal. The crowd must hold surplus on a shelf, see WithSurplusHold, since NPCs trade only what
// they hold there.
func NewMarket(p MarketParams, crowd *Crowd, player *Collection, journal *Journal, valuer Valuer) (*Market, error) {
	switch {
	case crowd == nil:
		return nil, errors.New("market: crowd must not be nil")
	case player == nil:
		return nil, errors.New("market: player collection must not be nil")
	case journal == nil:
		return nil, errors.New("market: journal must not be nil")
	case crowd.holdTicks <= 0:
		return nil, errors.New("market: the crowd holds no surplus, so NPCs have nothing to trade: build it with a surplus hold above zero")
	case p.CoinRate <= 0:
		return nil, fmt.Errorf("market: coin rate must be > 0, got %d", p.CoinRate)
	case p.Expiry <= 0:
		return nil, fmt.Errorf("market: expiry must be > 0, got %v", p.Expiry)
	case p.MaxCardsPerSide < 1:
		return nil, fmt.Errorf("market: max cards per side must be >= 1, got %d", p.MaxCardsPerSide)
	}
	return &Market{params: p, crowd: crowd, player: player, journal: journal, valuer: valuer, nextID: 1}, nil
}

// Requests returns the requests, oldest first.
func (m *Market) Requests() []MarketRequest { return slices.Clone(m.requests) }

// Listings returns the listings, oldest first.
func (m *Market) Listings() []MarketListing { return slices.Clone(m.listings) }

// Answers returns the open answers and those waiting for a reply, oldest first.
func (m *Market) Answers() []MarketAnswer {
	out := make([]MarketAnswer, len(m.answers))
	for i, a := range m.answers {
		out[i] = cloneAnswer(a)
	}
	return out
}

// Results returns the latest results, oldest first.
func (m *Market) Results() []MarketResult {
	out := make([]MarketResult, len(m.results))
	for i, r := range m.results {
		out[i] = MarketResult{Answer: cloneAnswer(r.Answer), Outcome: r.Outcome, At: r.At}
	}
	return out
}

// Pending returns how many NPCs have still to answer the request or listing id.
func (m *Market) Pending(id MarketID) int {
	n := 0
	for _, p := range m.pending {
		if p.To == id {
			n++
		}
	}
	return n
}

// Reserved returns the player's copies of key held for a listing or a counter, which the player
// cannot give or sell meanwhile.
func (m *Market) Reserved(key CardKey) int {
	n := 0
	for _, l := range m.listings {
		if l.Card == key {
			n++
		}
	}
	for _, a := range m.answers {
		if a.Counter == nil {
			continue
		}
		for _, l := range a.Counter.Cards {
			if l.Key == key {
				n += l.Qty
			}
		}
	}
	return n
}

func cloneAnswer(a MarketAnswer) MarketAnswer {
	a.Cards = slices.Clone(a.Cards)
	if a.Counter != nil {
		c := *a.Counter
		c.Cards = slices.Clone(c.Cards)
		a.Counter = &c
	}
	return a
}

func (m *Market) newID() MarketID {
	id := m.nextID
	m.nextID++
	return id
}

func (m *Market) request(id MarketID) (int, bool) {
	i := slices.IndexFunc(m.requests, func(r MarketRequest) bool { return r.ID == id })
	return i, i >= 0
}

func (m *Market) listing(id MarketID) (int, bool) {
	i := slices.IndexFunc(m.listings, func(l MarketListing) bool { return l.ID == id })
	return i, i >= 0
}

func (m *Market) answer(id MarketID) (int, bool) {
	i := slices.IndexFunc(m.answers, func(a MarketAnswer) bool { return a.ID == id })
	return i, i >= 0
}

// record remembers how an answer ended, dropping the oldest results beyond marketResultsKept.
func (m *Market) record(a MarketAnswer, outcome MarketOutcome, at time.Time) {
	m.results = append(m.results, MarketResult{Answer: cloneAnswer(a), Outcome: outcome, At: at})
	if over := len(m.results) - marketResultsKept; over > 0 {
		m.results = slices.Delete(m.results, 0, over)
	}
}

// end drops the request or listing id with its answers and the answers to come.
func (m *Market) end(id MarketID) {
	m.requests = slices.DeleteFunc(m.requests, func(r MarketRequest) bool { return r.ID == id })
	m.listings = slices.DeleteFunc(m.listings, func(l MarketListing) bool { return l.ID == id })
	m.answers = slices.DeleteFunc(m.answers, func(a MarketAnswer) bool { return a.To == id })
	m.pending = slices.DeleteFunc(m.pending, func(p MarketPending) bool { return p.To == id })
}

func (m *Market) dropAnswer(id MarketID) {
	m.answers = slices.DeleteFunc(m.answers, func(a MarketAnswer) bool { return a.ID == id })
}
