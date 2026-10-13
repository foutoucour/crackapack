package sim

import (
	"errors"
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ErrTradesDisabled is returned, or given as the reason of a refusal, by the trade actions of a
// run whose configuration has trades off.
var ErrTradesDisabled = errors.New("sim: trades are not enabled in this run")

// Trading is the trade side of a Session: what the player sees of the NPCs and what it can do
// with them. An NPC counters a short offer once, and that counter is an open offer the player
// answers with AcceptOffer. See docs/simulation.md.
//
// Refusals, a missing offer and an insufficient balance are normal outcomes. Any other error
// aborts the run.
type Trading interface {
	// Offers returns the open offers, oldest first.
	Offers() []domain.Offer
	// NPCCount returns the number of NPCs. Their IDs run from 0 to NPCCount()-1.
	NPCCount() int
	// NPCShelf returns what the NPC can trade away, sorted by card.
	NPCShelf(npcID int) []domain.TradeLine
	// HoldersOf returns the IDs of the NPCs holding key on their shelf, ascending.
	HoldersOf(key domain.CardKey) []int
	// ProposeTrade offers the NPC a swap: the player gives give and topUp coins, the NPC gives ask
	// from its shelf. The NPC accepts, counters once or refuses.
	ProposeTrade(npcID int, give, ask []domain.TradeLine, topUp domain.Coins) domain.Decision
	// AcceptOffer accepts an open offer. It returns domain.ErrOfferNotFound for an offer that is
	// not open and domain.ErrInsufficientBalance, leaving the offer open, when the player cannot
	// pay its top-up.
	AcceptOffer(offerID int) (domain.Decision, error)
	// CounterOffer answers an offer an NPC proposed with other cards. The NPC accepts or refuses.
	CounterOffer(offerID int, give []domain.TradeLine, topUp domain.Coins) (domain.Decision, error)
}

func (p *port) Offers() []domain.Offer {
	if p.e.exchange == nil {
		return nil
	}
	return p.e.exchange.Offers()
}

func (p *port) NPCCount() int {
	if p.e.exchange == nil {
		return 0
	}
	return p.e.exchange.NPCCount()
}

func (p *port) NPCShelf(npcID int) []domain.TradeLine {
	if p.e.exchange == nil {
		return nil
	}
	return p.e.exchange.Shelf(npcID)
}

func (p *port) HoldersOf(key domain.CardKey) []int {
	if p.e.exchange == nil {
		return nil
	}
	return p.e.exchange.HoldersOf(key)
}

func (p *port) ProposeTrade(npcID int, give, ask []domain.TradeLine, topUp domain.Coins) domain.Decision {
	if p.e.exchange == nil {
		return domain.Decision{Outcome: domain.OutcomeRefused, Reason: ErrTradesDisabled}
	}
	d, err := p.e.exchange.Propose(npcID, give, ask, topUp)
	if err != nil {
		p.fatalErr = fmt.Errorf("propose trade: %w", err)
	}
	return d
}

func (p *port) AcceptOffer(offerID int) (domain.Decision, error) {
	if p.e.exchange == nil {
		return domain.Decision{}, ErrTradesDisabled
	}
	return p.tradeResult("accept offer", func() (domain.Decision, error) { return p.e.exchange.Accept(offerID) })
}

func (p *port) CounterOffer(offerID int, give []domain.TradeLine, topUp domain.Coins) (domain.Decision, error) {
	if p.e.exchange == nil {
		return domain.Decision{}, ErrTradesDisabled
	}
	return p.tradeResult("counter offer", func() (domain.Decision, error) {
		return p.e.exchange.Counter(offerID, give, topUp)
	})
}

// tradeResult runs a trade action and passes its error on to the policy when it is a normal
// outcome, and aborts the run on any other error.
func (p *port) tradeResult(action string, run func() (domain.Decision, error)) (domain.Decision, error) {
	d, err := run()
	switch {
	case err == nil:
		return d, nil
	case errors.Is(err, domain.ErrInsufficientBalance),
		errors.Is(err, domain.ErrOfferNotFound),
		errors.Is(err, domain.ErrNotCounterable):
		return d, err
	}
	p.fatalErr = fmt.Errorf("%s: %w", action, err)
	return d, err
}
