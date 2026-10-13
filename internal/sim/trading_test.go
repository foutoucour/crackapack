package sim_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// scriptPolicy runs script once, in the first session, and does nothing afterwards.
type scriptPolicy struct {
	script func(s sim.Session)
	ran    *bool
}

func (p scriptPolicy) RunSession(s sim.Session) {
	if *p.ran {
		return
	}
	*p.ran = true
	p.script(s)
}

func TestSession_tradingIsDisabledWithoutTrades(t *testing.T) {
	t.Parallel()
	var ran bool
	var decision domain.Decision
	var acceptErr, counterErr error
	var offers []domain.Offer
	var npcCount int
	policy := scriptPolicy{ran: &ran, script: func(s sim.Session) {
		offers = s.Offers()
		npcCount = s.NPCCount()
		decision = s.ProposeTrade(0, nil, nil, 0)
		_, acceptErr = s.AcceptOffer(1)
		_, counterErr = s.CounterOffer(1, nil, 0)
	}}

	runResult(t, smallConfig(), policy)

	if !ran {
		t.Fatal("the policy never ran")
	}
	if len(offers) != 0 || npcCount != 0 {
		t.Errorf("Offers = %v, NPCCount = %d, want none without trades", offers, npcCount)
	}
	if decision.Outcome != domain.OutcomeRefused || !errors.Is(decision.Reason, sim.ErrTradesDisabled) {
		t.Errorf("ProposeTrade = %+v, want a refusal with ErrTradesDisabled", decision)
	}
	if !errors.Is(acceptErr, sim.ErrTradesDisabled) || !errors.Is(counterErr, sim.ErrTradesDisabled) {
		t.Errorf("AcceptOffer = %v, CounterOffer = %v, want ErrTradesDisabled", acceptErr, counterErr)
	}
}

func TestSession_badTradeRequestsAreRefusalsNotRunAborts(t *testing.T) {
	t.Parallel()
	var ran bool
	var decision domain.Decision
	var acceptErr error
	policy := scriptPolicy{ran: &ran, script: func(s sim.Session) {
		decision = s.ProposeTrade(9999, []domain.TradeLine{{Key: domain.CardKey{ID: "x", Finish: domain.FinishNonFoil}, Qty: 1}}, nil, 0)
		_, acceptErr = s.AcceptOffer(424242)
	}}

	runResult(t, tradesConfig(5), policy)

	if !ran {
		t.Fatal("the policy never ran")
	}
	if decision.Outcome != domain.OutcomeRefused || !errors.Is(decision.Reason, domain.ErrUnknownNPC) {
		t.Errorf("ProposeTrade of an unknown NPC = %+v, want a refusal with ErrUnknownNPC", decision)
	}
	if !errors.Is(acceptErr, domain.ErrOfferNotFound) {
		t.Errorf("AcceptOffer of an unknown offer = %v, want ErrOfferNotFound", acceptErr)
	}
}

// everySession runs f in every session of a run.
type everySession struct{ f func(sim.Session) }

func (p everySession) RunSession(s sim.Session) { p.f(s) }

func TestSession_exposesTheCrowdAndItsShelves(t *testing.T) {
	t.Parallel()
	cfg := tradesConfig(8)
	cfg.RunDays = 6
	var npcCount, shelvesSeen int
	holdersConsistent := true
	policy := everySession{f: func(s sim.Session) {
		npcCount = s.NPCCount()
		for id := range s.NPCCount() {
			shelf := s.NPCShelf(id)
			if len(shelf) > 0 {
				shelvesSeen++
			}
			for _, line := range shelf {
				holdersConsistent = holdersConsistent && slices.Contains(s.HoldersOf(line.Key), id)
			}
		}
	}}

	runResult(t, cfg, policy)

	if npcCount != 8 {
		t.Errorf("NPCCount = %d, want 8", npcCount)
	}
	if shelvesSeen == 0 {
		t.Error("no NPC ever had a shelf over 6 days, want some surplus waiting")
	}
	if !holdersConsistent {
		t.Error("HoldersOf does not list an NPC for a card on its shelf")
	}
}
