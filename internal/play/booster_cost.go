package play

import (
	"fmt"
	"maps"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// bill is what opening one booster takes from the player: energy, and the coins of the set's fee and
// of a top-up.
type bill struct {
	energy int
	fee    domain.Coins
	topUp  domain.Coins
}

// billFor prices a booster of setCode that is not a welcome one, for a player who tops up its energy
// with coins when topUp is true. It returns a *NotEnoughEnergyError when the energy falls short of
// what the opening asks, and a *NotEnoughCoinsError when the coins do. g.mu must be held, and now
// must be the instant sync returned.
func (g *Game) billFor(setCode string, topUp bool, now time.Time) (bill, error) {
	cost := g.rules.Costs[setCode]
	have := g.energy.Reserve() + g.energy.Savings()
	b := bill{energy: cost, fee: g.rules.Fees[setCode]}
	if have < cost {
		if !topUp {
			return bill{}, &NotEnoughEnergyError{Need: cost, Have: have, Wait: g.waitFor(cost, now)}
		}
		least := g.rules.TopUp.MinEnergy
		if have < least {
			return bill{}, &NotEnoughEnergyError{Need: least, Have: have, Wait: g.waitFor(least, now), TopUp: true}
		}
		b.energy = have
		b.topUp = domain.Coins(cost-have) * g.rules.TopUp.Price
	}
	if need := b.fee + b.topUp; need > 0 {
		coins, err := g.coins()
		if err != nil {
			return bill{}, err
		}
		if coins < need {
			return bill{}, &NotEnoughCoinsError{Need: need, Have: coins}
		}
	}
	return b, nil
}

// pay takes b from the player: the energy, and the coins into the sinks of the fee and of the
// top-up. billFor checked that the player has them. g.mu must be held.
func (g *Game) pay(setCode string, b bill) error {
	if b.fee > 0 {
		if err := g.economy.journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountBoosterFee, Amount: b.fee, Memo: "booster fee: set " + setCode}); err != nil {
			return fmt.Errorf("pay the fee: %w", err)
		}
	}
	if b.topUp > 0 {
		if err := g.economy.journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountCoinTopup, Amount: b.topUp, Memo: "coin top-up: set " + setCode}); err != nil {
			return fmt.Errorf("pay the top-up: %w", err)
		}
	}
	if err := g.energy.Spend(b.energy); err != nil {
		return fmt.Errorf("spend the energy: %w", err)
	}
	return nil
}

// coins is the player's coins, with the daily rewards of the days gone by, and zero in a game
// without a Counter. g.mu must be held.
func (g *Game) coins() (domain.Coins, error) {
	if g.economy == nil {
		return 0, nil
	}
	if err := g.syncCounter(); err != nil {
		return 0, err
	}
	return g.economy.journal.Balance(domain.AccountPlayer), nil
}

// checkFees returns a copy of fees if every fee is above zero and names a set of costs, and an
// error otherwise.
func checkFees(fees map[string]domain.Coins, costs map[string]int) (map[string]domain.Coins, error) {
	for code, fee := range fees {
		if _, ok := costs[code]; !ok {
			return nil, fmt.Errorf("play: a fee for set %q, which the game does not play", code)
		}
		if fee <= 0 {
			return nil, fmt.Errorf("play: set %q has a fee of %d coins, want one above zero", code, fee)
		}
	}
	return maps.Clone(fees), nil
}
