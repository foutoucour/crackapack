package sim_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// sellOncePolicy opens boosters, then offers its duplicates to the Counter once, recording what it
// held and what the Counter did. It measures SellDuplicates, which every selling policy uses.
type sellOncePolicy struct {
	copies, distinct *int
	sold, refused    *int
	done             *bool
}

func (p sellOncePolicy) RunSession(s sim.Session) {
	if *p.done {
		return
	}
	openOnlyPolicy{}.RunSession(s)
	*p.copies, *p.distinct = 0, 0
	for _, qty := range s.View().OwnedCards {
		if qty > 1 {
			*p.copies += qty - 1
			*p.distinct++
		}
	}
	// Wait for a card held three times or more: only then do copies outnumber cards.
	if *p.copies <= *p.distinct {
		return
	}
	*p.sold, *p.refused = s.SellDuplicates(1)
	*p.done = true
}

type sellOnceResult struct {
	copies, distinct, sold, refused int
	done                            bool
	summary                         sim.Summary
}

func runSellOnce(t *testing.T, cfg config.Config) sellOnceResult {
	t.Helper()
	var r sellOnceResult
	policy := sellOncePolicy{copies: &r.copies, distinct: &r.distinct, sold: &r.sold, refused: &r.refused, done: &r.done}
	r.summary = runResult(t, cfg, policy).Summary
	if !r.done || r.copies <= r.distinct {
		t.Fatalf("copies %d and distinct cards %d: want more duplicate copies than cards, so that the test tells them apart", r.copies, r.distinct)
	}
	return r
}

// A Counter that cannot pay refuses a card at its price, so it refuses every other copy of that
// card too. The player must not offer the copies one by one: it moves on to the next card.
func TestSellDuplicates_aBrokeCounterIsAskedOncePerCardNotOncePerCopy(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.CounterBudget = config.CounterBudgetConfig{Enabled: true}

	r := runSellOnce(t, cfg)

	if r.sold != 0 {
		t.Errorf("sold %d cards to a Counter with an empty treasury, want 0", r.sold)
	}
	if got := r.summary["counter_broke_refusals"]; got == 0 || got > float64(r.distinct) {
		t.Errorf("counter_broke_refusals = %v, want between 1 and %d, one per card", got, r.distinct)
	}
	// Sub-coin copies are still refused one by one, so the total can exceed the number of cards, but
	// it stays far below one refusal per copy.
	if r.refused >= r.copies/2 {
		t.Errorf("refused %d times for %d duplicate copies, want far fewer than one per copy", r.refused, r.copies)
	}
}

// Copies of a sub-coin card are still refused one by one: that refusal is about the card, not the
// treasury. A Counter with coins refuses nothing for lack of them.
func TestSellDuplicates_aCounterWithCoinsIsNeverBroke(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.CounterBudget = config.CounterBudgetConfig{Enabled: true, Initial: 1_000_000_000}

	r := runSellOnce(t, cfg)

	if r.sold == 0 {
		t.Errorf("sold %d of %d duplicate copies to a Counter with a large treasury, want it to pay for them", r.sold, r.copies)
	}
	if got := r.summary["counter_broke_refusals"]; got != 0 {
		t.Errorf("counter_broke_refusals = %v, want 0 for a Counter with coins", got)
	}
}

// callLog wraps a Session and records the order of the calls that matter to a sale.
type callLog struct {
	sim.Session
	calls *[]string
	keeps *[]int
}

func (c callLog) ProposeTrade(npcID int, give, ask []domain.TradeLine, topUp domain.Coins) domain.Decision {
	*c.calls = append(*c.calls, "trade")
	return c.Session.ProposeTrade(npcID, give, ask, topUp)
}

func (c callLog) AcceptOffer(offerID int) (domain.Decision, error) {
	*c.calls = append(*c.calls, "trade")
	return c.Session.AcceptOffer(offerID)
}

func (c callLog) SellDuplicates(keep int) (int, int) {
	*c.calls = append(*c.calls, "sell")
	*c.keeps = append(*c.keeps, keep)
	return c.Session.SellDuplicates(keep)
}

// recorder holds what a loggedPolicy saw: the calls of each session and every keep it sold above.
type recorder struct {
	sessions [][]string
	keeps    []int
}

type loggedPolicy struct {
	inner sim.PlayerPolicy
	rec   *recorder
}

func (p loggedPolicy) RunSession(s sim.Session) {
	var calls []string
	p.inner.RunSession(callLog{Session: s, calls: &calls, keeps: &p.rec.keeps})
	p.rec.sessions = append(p.rec.sessions, calls)
}

// A sale is permanent, so a policy trades a duplicate before it sells it: the NPCs do not buy from
// the Counter, and a card sold there is gone. The trading policies play their trades first and the
// collector's session, which sells, last.
func TestTradingPoliciesTradeBeforeTheySell(t *testing.T) {
	t.Parallel()
	policies := map[string]sim.PlayerPolicy{"trader": sim.NewTraderPolicy(), "exploiter": sim.NewExploiterPolicy()}
	for name, inner := range policies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := npcConfig(30, 8, 1)
			cfg.Trades = config.TradesConfig{Enabled: true, OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, MaxOpenOffersPerDay: 20}
			cfg.NPCs.SurplusHoldDays = 2
			cfg.RunDays = 10
			rec := &recorder{}

			runResult(t, cfg, loggedPolicy{inner: inner, rec: rec})

			var tradingSessions int
			for i, calls := range rec.sessions {
				firstSale := slices.Index(calls, "sell")
				for j, c := range calls {
					if c == "trade" {
						tradingSessions++
						if firstSale >= 0 && j > firstSale {
							t.Fatalf("session %d: a trade came after the first sale: %v", i, calls)
						}
					}
				}
			}
			if tradingSessions == 0 {
				t.Fatalf("no session of %d traded, so the test proves nothing", len(rec.sessions))
			}
		})
	}
}

func TestSellingPoliciesKeepWhatTheyStillNeed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		policy   sim.PlayerPolicy
		wantKeep int
	}{
		{name: "collector keeps a playset", policy: sim.NewCollectorPolicy(), wantKeep: domain.PlaysetSize},
		{name: "trader keeps a playset", policy: sim.NewTraderPolicy(), wantKeep: domain.PlaysetSize},
		{name: "exploiter keeps a playset", policy: sim.NewExploiterPolicy(), wantKeep: domain.PlaysetSize},
		{name: "cash-out keeps one copy", policy: sim.NewCashOutPolicy(), wantKeep: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{}

			runResult(t, smallConfig(), loggedPolicy{inner: tc.policy, rec: rec})

			if len(rec.keeps) == 0 {
				t.Fatal("the policy never sold, so the test proves nothing")
			}
			for _, k := range rec.keeps {
				if k != tc.wantKeep {
					t.Fatalf("sold above %d copies, want above %d", k, tc.wantKeep)
				}
			}
		})
	}
}
