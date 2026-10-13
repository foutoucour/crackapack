package domain_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// playerValue is what the player owns, in hundredths of a coin at rate 100: coins × 100 plus the
// reference value of the cards in cents × the rate. A trade must never raise it.
func (fx *exchangeFixture) playerValue() int64 {
	const rate = 100
	coins := int64(fx.journal.Balance(domain.AccountPlayer)) * 100
	cards := int64(fx.player.Value(fx.prices, exchangeFallbackPrice)) * rate
	return coins + cards
}

func randomLines(rng *rand.Rand, pool []domain.TradeLine, maxLines, maxQty int) []domain.TradeLine {
	if len(pool) == 0 {
		return nil
	}
	var lines []domain.TradeLine
	for range 1 + rng.IntN(maxLines) {
		src := pool[rng.IntN(len(pool))]
		lines = append(lines, domain.TradeLine{Key: src.Key, Qty: 1 + rng.IntN(min(maxQty, src.Qty))})
	}
	return lines
}

func ownedLines(fx *exchangeFixture) []domain.TradeLine {
	var lines []domain.TradeLine
	for key, qty := range fx.player.Snapshot() {
		lines = append(lines, domain.TradeLine{Key: key, Qty: qty})
	}
	slices.SortFunc(lines, func(a, b domain.TradeLine) int {
		switch {
		case a.Key.ID < b.Key.ID:
			return -1
		case a.Key.ID > b.Key.ID:
			return 1
		}
		return int(a.Key.Finish[0]) - int(b.Key.Finish[0])
	})
	return lines
}

func TestExchangeNeverLetsThePlayerGainValue(t *testing.T) {
	t.Parallel()
	for _, margin := range []domain.Percent{0, 5, 25} {
		t.Run(fmt.Sprintf("margin %d%%", margin), func(t *testing.T) {
			t.Parallel()
			const npcCount = 5
			fx := newExchangeFixture(t, npcCount, tradingSpec(margin), domain.ExchangeParams{})
			rng := rand.New(rand.NewPCG(99, 99))
			var all []domain.CardKey
			for _, cat := range crowdSets() {
				for id, card := range cat.Cards {
					if !card.IsBasicLand {
						all = append(all, domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
					}
				}
			}
			slices.SortFunc(all, func(a, b domain.CardKey) int {
				switch {
				case a.ID < b.ID:
					return -1
				case a.ID > b.ID:
					return 1
				}
				return 0
			})
			for range 60 {
				fx.give(t, all[rng.IntN(len(all))], 1+rng.IntN(3))
			}
			for id := range npcCount {
				for range 25 {
					fx.shelve(id, all[rng.IntN(len(all))], 1+rng.IntN(2))
				}
			}
			fx.fund(t, 5000)

			for step := range 2000 {
				before := fx.playerValue()
				npcID := rng.IntN(npcCount)
				d, err := fx.exchange.Propose(npcID,
					randomLines(rng, ownedLines(fx), 3, 2),
					randomLines(rng, fx.npcs[npcID].Shelf(), 3, 2),
					domain.Coins(rng.IntN(60)))
				if err != nil {
					t.Fatalf("step %d: Propose: %v", step, err)
				}
				if d.Outcome == domain.OutcomeCountered && rng.IntN(2) == 0 {
					if _, err := fx.exchange.Accept(d.Offer.ID); err != nil && !errors.Is(err, domain.ErrInsufficientBalance) {
						t.Fatalf("step %d: Accept: %v", step, err)
					}
				}
				if after := fx.playerValue(); after > before {
					t.Fatalf("step %d: the player's value rose from %d to %d through a trade", step, before, after)
				}
				if sum := fx.journal.SumAllBalances(); sum != 0 {
					t.Fatalf("step %d: the journal sums to %d, want 0", step, sum)
				}
			}
			if stats := fx.exchange.TakeDayStats(); stats.Accepted == 0 {
				t.Errorf("no trade was accepted in 2000 attempts, the test proves nothing: %+v", stats)
			}
		})
	}
}
