package domain

import (
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrUnknownNPC is the reason of a refusal for an NPC ID the crowd does not have.
	ErrUnknownNPC = errors.New("exchange: unknown NPC")
	// ErrNPCNotTrading is the reason of a refusal for an NPC whose archetype has no behaviour.
	ErrNPCNotTrading = errors.New("exchange: the NPC does not trade")
	// ErrEmptyOffer is the reason of a refusal when the NPC is asked to give no card.
	ErrEmptyOffer = errors.New("exchange: the NPC must give at least one card")
	// ErrNothingOffered is the reason of a refusal when the player gives no card and no coin.
	ErrNothingOffered = errors.New("exchange: the player must give a card or coins")
	// ErrTooManyCards is the reason of a refusal when a side holds more cards than allowed.
	ErrTooManyCards = errors.New("exchange: too many cards on one side")
	// ErrPlayerLacksCard is the reason of a refusal when the player does not hold what it offers.
	ErrPlayerLacksCard = errors.New("exchange: the player does not hold the offered cards")
	// ErrNPCLacksCard is the reason of a refusal when the NPC's shelf lacks what it is asked for.
	ErrNPCLacksCard = errors.New("exchange: the NPC does not hold the requested cards")
	// ErrBelowMargin is the reason of a refusal when the NPC would receive less than it gives plus
	// its margin and may not counter.
	ErrBelowMargin = errors.New("exchange: the NPC would receive less than it gives plus its margin")
	// ErrOfferNotFound is returned for an offer ID that is not open: it never existed, was
	// answered, or has lapsed.
	ErrOfferNotFound = errors.New("exchange: no such open offer")
	// ErrNotCounterable is returned when the player counters an offer that is not an NPC's own
	// proposal. An NPC counters once, see ADR-0012.
	ErrNotCounterable = errors.New("exchange: only an offer proposed by an NPC can be countered")
	// ErrCounterVetoed is the reason of a refusal when a trait of the NPC vetoes the player's
	// counter-offer. No offer is stored; the NPC's counter count is unchanged.
	ErrCounterVetoed = errors.New("exchange: the NPC's trait vetoed the player's counter")
)

// Outcome is how an offer ended or was answered.
type Outcome int

const (
	// OutcomeAccepted means the trade was executed.
	OutcomeAccepted Outcome = iota + 1
	// OutcomeCountered means the NPC answered with a counter-offer, now open.
	OutcomeCountered
	// OutcomeRefused means nothing happened: see Decision.Reason.
	OutcomeRefused
	// OutcomeExpired means an open offer lapsed unanswered.
	OutcomeExpired
)

// Decision is the Exchange's answer to a proposal or an acceptance.
type Decision struct {
	Outcome Outcome
	// Offer is the trade executed when accepted, or the NPC's counter-offer, open until it lapses,
	// when countered. It is the zero Offer when refused.
	Offer Offer
	// Reason says why an offer was refused, and is nil otherwise.
	Reason error
}

// ExchangeStats are the trade counters of a period, see Exchange.TakeDayStats.
type ExchangeStats struct {
	// PlayerProposals counts the offers the player made, counters to NPC offers included.
	PlayerProposals int
	// NPCOffers counts the offers NPCs made to the player.
	NPCOffers int
	Accepted  int
	// Countered counts the counter-offers NPCs made.
	Countered int
	Refused   int
	Expired   int
	// VolumeCoins is the reference value of the cards moved by accepted trades, in coins.
	VolumeCoins Coins
	// TopUpCoins is the coins the player paid on top of cards.
	TopUpCoins Coins
}

// HistoryEntry records how one offer was answered.
type HistoryEntry struct {
	Day     int
	Offer   Offer
	Outcome Outcome
	Reason  error
}

// ExchangeParams configures an Exchange. Every field comes from the configuration.
type ExchangeParams struct {
	// ExpiryDays is how many days an offer stays open, the day it is made included.
	ExpiryDays int
	// MaxCardsPerSide bounds the cards each side of an offer gives, counting quantities.
	MaxCardsPerSide int
	// DuplicateKeep is how many copies of a printing the player keeps out of NPC proposals: the
	// copies above it are the player's duplicates. It must be in [1, PlaysetSize].
	DuplicateKeep int
	// MaxOffersPerDay bounds the offers the crowd makes the player each day.
	MaxOffersPerDay int
	// HistorySize is how many answered offers History keeps. Zero keeps none.
	HistorySize int
	// CoinRate is the coins per US dollar, to put a coin top-up and a card on one scale.
	CoinRate CoinRate
}

// Exchange runs the trades between the player and the NPCs of a crowd: it prices offers, applies
// the acceptance rule, counters once and moves the cards atomically. It is not safe for
// concurrent use.
type Exchange struct {
	params  ExchangeParams
	crowd   *Crowd
	player  *Collection
	journal *Journal
	valuer  Valuer

	day       int
	dayOffers int // offers NPCs made today
	nextID    int
	open      map[int]Offer
	openIDs   []int       // open offer IDs in creation order; IDs are monotonically increasing
	openCount map[int]int // open offers per NPC
	stats     ExchangeStats
	history   []HistoryEntry
}

// NewExchange builds an Exchange between player and the NPCs of crowd. Top-ups are posted to
// journal.
//
// The crowd must hold surplus on a shelf, see WithSurplusHold: an NPC with no shelf has nothing to
// give, so every trade would be refused.
func NewExchange(p ExchangeParams, crowd *Crowd, player *Collection, journal *Journal, valuer Valuer) (*Exchange, error) {
	switch {
	case crowd == nil:
		return nil, fmt.Errorf("exchange: crowd must not be nil")
	case player == nil:
		return nil, fmt.Errorf("exchange: player collection must not be nil")
	case journal == nil:
		return nil, fmt.Errorf("exchange: journal must not be nil")
	case crowd.holdTicks <= 0:
		return nil, fmt.Errorf("exchange: the crowd holds no surplus, so NPCs have nothing to trade: build it with a surplus hold above zero")
	case p.CoinRate <= 0:
		return nil, fmt.Errorf("exchange: coin rate must be > 0, got %d", p.CoinRate)
	case p.ExpiryDays < 1:
		return nil, fmt.Errorf("exchange: expiry days must be >= 1, got %d", p.ExpiryDays)
	case p.MaxCardsPerSide < 1:
		return nil, fmt.Errorf("exchange: max cards per side must be >= 1, got %d", p.MaxCardsPerSide)
	case p.DuplicateKeep < 1 || p.DuplicateKeep > PlaysetSize:
		return nil, fmt.Errorf("exchange: duplicate keep must be in [1, %d], got %d", PlaysetSize, p.DuplicateKeep)
	case p.MaxOffersPerDay < 0:
		return nil, fmt.Errorf("exchange: max offers per day must be >= 0, got %d", p.MaxOffersPerDay)
	case p.HistorySize < 0:
		return nil, fmt.Errorf("exchange: history size must be >= 0, got %d", p.HistorySize)
	}
	return &Exchange{
		params: p, crowd: crowd, player: player, journal: journal, valuer: valuer,
		open: make(map[int]Offer), openCount: make(map[int]int),
	}, nil
}

// Offers returns the open offers, oldest first. The slice is in creation order because offer IDs
// are monotonically increasing and openIDs is maintained in insertion order.
func (x *Exchange) Offers() []Offer {
	offers := make([]Offer, 0, len(x.openIDs))
	for _, id := range x.openIDs {
		offers = append(offers, x.open[id])
	}
	return offers
}

// NPCCount returns the number of NPCs. Their IDs run from 0 to NPCCount()-1.
func (x *Exchange) NPCCount() int { return len(x.crowd.npcs) }

// Shelf returns what NPC npcID can trade away, sorted by card, or nil for an unknown NPC.
func (x *Exchange) Shelf(npcID int) []TradeLine {
	npc, ok := x.crowd.byID[npcID]
	if !ok {
		return nil
	}
	return npc.Shelf()
}

// HoldersOf returns the IDs of the NPCs holding key on their shelf, ascending.
func (x *Exchange) HoldersOf(key CardKey) []int { return x.crowd.HoldersOf(key) }

// TakeDayStats returns the counters since the last call and resets them.
func (x *Exchange) TakeDayStats() ExchangeStats {
	s := x.stats
	x.stats = ExchangeStats{}
	return s
}

// History returns the last HistorySize answered offers, oldest first.
func (x *Exchange) History() []HistoryEntry { return slices.Clone(x.history) }
