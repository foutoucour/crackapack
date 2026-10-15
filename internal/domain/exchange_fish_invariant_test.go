package domain_test

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// tradeLinesCents returns the total reference value (USD cents) of a set of trade lines using the
// fixture's price snapshot. This mirrors the unexported linesValue in exchange_rules.go and is
// used here to assert per-offer invariants without depending on production internals.
func tradeLinesCents(fx *exchangeFixture, lines []domain.TradeLine) int64 {
	var total int64
	for _, l := range lines {
		p, ok := fx.prices[l.Key.ID]
		if !ok {
			p = exchangeFallbackPrice
		}
		total += int64(p) * int64(l.Qty)
	}
	return total
}

// TestFishExchangeNPCGivesNeverExceedsPlayerGives verifies the no-value-gain invariant for a
// fish crowd. The fish archetype has margin_pct=0, so for every NPC-initiated offer the NPC
// must give cards worth ≤ what it asks the player to give (plus any top-up the player adds).
// Random player-initiated Propose calls are also checked: the player's combined coin+card value
// must never rise after a trade.
func TestFishExchangeNPCGivesNeverExceedsPlayerGives(t *testing.T) {
	t.Parallel()
	spec := domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "speculator",
		WishSize:       8,
		MarginPct:      0, // fish has no margin
		OfferChancePct: 100,
		Traits: []domain.TraitSpec{{Name: "fish", Params: map[string]int{
			"price_unit_cents": 100,
			"rarity_points":    5,
			"foil_points":      3,
		}}},
	}
	const npcCount = 3
	fx := newExchangeFixtureWithRegistry(t, npcCount, spec, domain.ExchangeParams{}, domain.DefaultTraits(), 1000)

	rng := rand.New(rand.NewPCG(42, 42))
	var all []domain.CardKey
	for _, cat := range crowdSets() {
		for id, card := range cat.Cards {
			if !card.IsBasicLand {
				all = append(all, domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
			}
		}
	}

	// Seed the player's collection and NPC shelves.
	for range 60 {
		fx.give(t, all[rng.IntN(len(all))], 1+rng.IntN(3))
	}
	for id := range npcCount {
		for range 25 {
			fx.shelve(id, all[rng.IntN(len(all))], 1+rng.IntN(2))
		}
	}
	fx.fund(t, 5000)

	var accepted int

	for step := range 400 {
		fx.exchange.StartDay(step + 1)

		// Check NPC-initiated offers: NPCGives value must be ≤ PlayerGives value (margin=0).
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("step %d: ProposeFromNPCs: %v", step, err)
		}
		for _, offer := range fx.exchange.Offers() {
			if offer.Origin != domain.OriginNPC {
				continue
			}
			npcGivesValue := tradeLinesCents(fx, offer.NPCGives)
			playerGivesValue := tradeLinesCents(fx, offer.PlayerGives)
			if npcGivesValue > playerGivesValue {
				t.Errorf("step %d: NPC-initiated offer #%d: NPCGives %d cents > PlayerGives %d cents (margin=0 invariant violated)",
					step, offer.ID, npcGivesValue, playerGivesValue)
			}
		}

		// Accept all open NPC offers (none require a top-up since margin=0, so they should accept).
		for _, offer := range fx.exchange.Offers() {
			if offer.Origin != domain.OriginNPC {
				continue
			}
			before := fx.playerValue()
			if _, err := fx.exchange.Accept(offer.ID); err != nil {
				if !errors.Is(err, domain.ErrInsufficientBalance) {
					t.Fatalf("step %d: Accept offer #%d: %v", step, offer.ID, err)
				}
				continue
			}
			accepted++
			if after := fx.playerValue(); after > before {
				t.Fatalf("step %d: Accept offer #%d: player value rose from %d to %d",
					step, offer.ID, before, after)
			}
		}

		// Random player-initiated Propose calls.
		owned := ownedLines(fx)
		if len(owned) == 0 {
			continue
		}
		npcID := rng.IntN(npcCount)
		before := fx.playerValue()
		d, err := fx.exchange.Propose(npcID,
			randomLines(rng, owned, 3, 2),
			randomLines(rng, fx.npcs[npcID].Shelf(), 3, 2),
			domain.Coins(rng.IntN(60)))
		if err != nil {
			t.Fatalf("step %d: Propose: %v", step, err)
		}
		if d.Outcome == domain.OutcomeCountered && rng.IntN(2) == 0 {
			if _, err := fx.exchange.Accept(d.Offer.ID); err != nil && !errors.Is(err, domain.ErrInsufficientBalance) {
				t.Fatalf("step %d: Accept counter: %v", step, err)
			}
		}
		if d.Outcome == domain.OutcomeAccepted {
			accepted++
		}
		if after := fx.playerValue(); after > before {
			t.Fatalf("step %d: player value rose from %d to %d through a trade", step, before, after)
		}
		if sum := fx.journal.SumAllBalances(); sum != 0 {
			t.Fatalf("step %d: journal sums to %d, want 0", step, sum)
		}
	}

	if accepted == 0 {
		t.Errorf("no trade was accepted in 400 steps, the test proves nothing")
	}
}
