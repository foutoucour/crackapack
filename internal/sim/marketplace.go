package sim

import (
	"errors"
	"fmt"
	"time"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// ErrMarketDisabled is returned by every market method of a run whose player does not use the
// market.
var ErrMarketDisabled = errors.New("sim: the run has no market")

// tickDuration is the time one tick stands for.
const tickDuration = 24 * time.Hour / TicksPerDay

// MarketPlayer is implemented by a PlayerPolicy that trades on the market. The engine then gives
// the run a market between the player and the crowd, as the game has one, and no Exchange unless
// the config enables trades.
type MarketPlayer interface {
	UsesMarket() bool
}

// Marketplace is the player's side of the market, see docs/core-features.md#the-market: it asks
// around for a card or lists one, and the NPCs answer after the delay of their archetype.
//
// Refusals such as a card no NPC holds, a card already asked for or listed, an answer that is gone
// and a balance too low are normal outcomes, returned to the policy. Any other error aborts the
// run.
type Marketplace interface {
	// MarketAsk asks around for card: each NPC that holds it answers with its asking price.
	MarketAsk(card domain.CardKey) (domain.MarketID, error)
	// MarketList puts one copy of card on the market at price coins: NPCs that want it make offers.
	MarketList(card domain.CardKey, price domain.Coins) (domain.MarketID, error)
	// MarketAnswers returns the open answers, including those the player countered.
	MarketAnswers() []domain.MarketAnswer
	// MarketAccept takes the answer id at once.
	MarketAccept(id domain.MarketID) error
}

// usesMarket reports whether policy trades on the market.
func usesMarket(policy PlayerPolicy) bool {
	mp, ok := policy.(MarketPlayer)
	return ok && mp.UsesMarket()
}

// newMarket builds the market of a player that uses it, or returns nil for one that does not.
func newMarket(cfg config.Config, policy PlayerPolicy, crowd *domain.Crowd, player *domain.Collection, journal *domain.Journal, valuer domain.Valuer) (*domain.Market, error) {
	if !usesMarket(policy) {
		return nil, nil
	}
	if crowd == nil {
		return nil, errors.New("sim: a market needs NPCs: set npcs.population")
	}
	market, err := domain.NewMarket(domain.MarketParams{
		Expiry:          time.Duration(cfg.Trades.OfferExpiryDays) * 24 * time.Hour,
		MaxCardsPerSide: cfg.Trades.MaxCardsPerSide,
		CoinRate:        cfg.CoinRate,
	}, crowd, player, journal, valuer)
	if err != nil {
		return nil, fmt.Errorf("sim: market: %w", err)
	}
	return market, nil
}

// tickTime is the instant of tick of day, 1-based, on the market's clock, which starts at the Unix
// epoch: only the distance between two instants matters, since the market works in delays.
func tickTime(day, tick int) time.Time {
	return time.Unix(0, 0).UTC().Add(time.Duration(day-1)*24*time.Hour + time.Duration(tick)*tickDuration)
}

// tickMarket delivers the answers due by now. It does nothing in a run without a market.
func (e *Engine) tickMarket(now time.Time) error {
	if e.market == nil {
		return nil
	}
	return e.market.Advance(now)
}

func (p *port) MarketAsk(card domain.CardKey) (domain.MarketID, error) {
	if p.e.market == nil {
		return 0, ErrMarketDisabled
	}
	id, err := p.e.market.AskAround(card, p.now)
	if err == nil {
		p.e.marketAsks++
	}
	return id, p.marketResult("ask around", err)
}

func (p *port) MarketList(card domain.CardKey, price domain.Coins) (domain.MarketID, error) {
	if p.e.market == nil {
		return 0, ErrMarketDisabled
	}
	id, err := p.e.market.List(card, price, p.now)
	if err == nil {
		p.e.marketListings++
	}
	return id, p.marketResult("list", err)
}

func (p *port) MarketAnswers() []domain.MarketAnswer {
	if p.e.market == nil {
		return nil
	}
	return p.e.market.Answers()
}

func (p *port) MarketAccept(id domain.MarketID) error {
	if p.e.market == nil {
		return ErrMarketDisabled
	}
	_, err := p.e.market.Accept(id, p.now)
	if err == nil {
		p.e.marketDeals++
	}
	return p.marketResult("accept answer", err)
}

// addMarketSummary adds what the player did on the market to s: the requests it made, the cards it
// listed and the deals it took.
func addMarketSummary(s Summary, e *Engine) {
	s["market_asks"] = float64(e.marketAsks)
	s["market_listings"] = float64(e.marketListings)
	s["market_deals"] = float64(e.marketDeals)
}

// marketResult passes err on to the policy when it is a normal outcome, and aborts the run on any
// other error.
func (p *port) marketResult(action string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNoHolder),
		errors.Is(err, domain.ErrAlreadyAsked),
		errors.Is(err, domain.ErrAlreadyListed),
		errors.Is(err, domain.ErrReserved),
		errors.Is(err, domain.ErrMarketNotFound),
		errors.Is(err, domain.ErrAwaitingReply),
		errors.Is(err, domain.ErrPlayerLacksCard),
		errors.Is(err, domain.ErrNPCLacksCard),
		errors.Is(err, domain.ErrInsufficientBalance):
		return err
	}
	p.fatalErr = fmt.Errorf("%s: %w", action, err)
	return err
}
