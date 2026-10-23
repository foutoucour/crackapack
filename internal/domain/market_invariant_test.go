package domain_test

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// Whatever the player does on the market, no trade raises what the player owns, coins plus the
// reference value of the cards, the ledger sums to zero, and every card is still somewhere: a trade
// creates neither value, nor coins out of nothing, nor cards. Prices vary, so the NPCs' margins and
// coin rounding are tried on cards worth from one cent to dozens of dollars.
func TestMarket_aTradeNeverCreatesValue(t *testing.T) {
	t.Parallel()
	stingy := domain.TraitSpec{Name: "stingy", Params: map[string]int{"common_pct": 5, "uncommon_pct": 10, "rare_pct": 20, "special_pct": 20, "mythic_pct": 40}}
	fx := newMarketFixture(t, stingy, domain.TraitSpec{Name: "gambler"})
	rng := rand.New(rand.NewPCG(7, 7))
	for _, key := range fx.keys {
		fx.prices[key.ID] = domain.USDCents(1 + rng.IntN(5000))
	}
	for range 40 {
		fx.give(t, fx.keys[rng.IntN(len(fx.keys))], 1+rng.IntN(2))
	}
	for npc := range fx.npcs {
		for range 30 {
			fx.crowd.ShelveForTest(npc, fx.keys[rng.IntN(len(fx.keys))], 1+rng.IntN(2))
		}
		var wishes []domain.Wish
		for range 10 {
			wishes = append(wishes, domain.Wish{ID: fx.keys[rng.IntN(len(fx.keys))].ID, Want: 1})
		}
		fx.crowd.SetWishesForTest(npc, dedupeWishes(wishes))
	}
	fx.fund(t, 50_000)

	cards := fx.cardsInPlay()
	now := marketStart()
	traded := 0
	for step := range 3000 {
		before, held := fx.playerValue(), fx.player.Snapshot()
		now = now.Add(time.Duration(rng.IntN(120)) * time.Minute)
		if err := randomMarketMove(fx, rng, now); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if err := fx.market.Advance(now); err != nil {
			t.Fatalf("step %d: Advance: %v", step, err)
		}
		after := fx.playerValue()
		if after > before {
			t.Fatalf("step %d: the player's value rose from %d to %d", step, before, after)
		}
		if !reflect.DeepEqual(held, fx.player.Snapshot()) {
			traded++
		}
		if sum := fx.journal.SumAllBalances(); sum != 0 {
			t.Fatalf("step %d: the journal sums to %d", step, sum)
		}
		if got := fx.cardsInPlay(); !reflect.DeepEqual(got, cards) {
			t.Fatalf("step %d: the cards in play changed", step)
		}
		for _, key := range fx.keys {
			if fx.market.Reserved(key) > fx.player.Quantity(key) {
				t.Fatalf("step %d: %v is reserved %d times and held %d", step, key, fx.market.Reserved(key), fx.player.Quantity(key))
			}
		}
	}
	if traded < 20 {
		t.Errorf("only %d steps traded, so the test proves little", traded)
	}
}

func dedupeWishes(wishes []domain.Wish) []domain.Wish {
	seen := map[domain.CardID]bool{}
	var out []domain.Wish
	for _, w := range wishes {
		if !seen[w.ID] {
			seen[w.ID] = true
			out = append(out, w)
		}
	}
	return out
}

// randomMarketMove makes one move a player could make. A refusal the market explains is a move
// too; any other error is returned.
func randomMarketMove(fx *marketFixture, rng *rand.Rand, now time.Time) error {
	var err error
	answers := fx.market.Answers()
	switch move := rng.IntN(6); {
	case move == 0:
		_, err = fx.market.AskAround(fx.keys[rng.IntN(len(fx.keys))], now)
	case move == 1:
		_, err = fx.market.List(fx.keys[rng.IntN(len(fx.keys))], domain.Coins(1+rng.IntN(6000)), now)
	case len(answers) == 0:
		return nil
	case move == 2:
		_, err = fx.market.Accept(answers[rng.IntN(len(answers))].ID, now)
	case move == 3:
		err = fx.market.Decline(answers[rng.IntN(len(answers))].ID)
	default:
		a := answers[rng.IntN(len(answers))]
		var give []domain.TradeLine
		if a.Kind == domain.AnswerAsk && rng.IntN(2) == 0 {
			give = randomLines(rng, ownedMarketLines(fx), 2, 1)
		}
		err = fx.market.Counter(a.ID, domain.Coins(rng.IntN(6000)), give, now)
	}
	for _, refusal := range []error{
		domain.ErrNoHolder, domain.ErrAlreadyAsked, domain.ErrAlreadyListed, domain.ErrPlayerLacksCard,
		domain.ErrReserved, domain.ErrInsufficientBalance, domain.ErrNPCLacksCard, domain.ErrAwaitingReply,
		domain.ErrNothingOffered, domain.ErrInvalidPrice, domain.ErrCoinsOnly, domain.ErrTooManyCards,
	} {
		if errors.Is(err, refusal) {
			return nil
		}
	}
	return err
}

func ownedMarketLines(fx *marketFixture) []domain.TradeLine {
	var lines []domain.TradeLine
	for _, key := range fx.keys {
		if n := fx.player.Quantity(key) - fx.market.Reserved(key); n > 0 {
			lines = append(lines, domain.TradeLine{Key: key, Qty: n})
		}
	}
	return lines
}
