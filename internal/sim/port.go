package sim

import (
	"errors"
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// buildWorldView constructs a fresh WorldView from the current engine state.
// It is called by port.View() on every Session.View() call, so policies
// always see state that is current after their most recent action.
func (e *Engine) buildWorldView(day int) WorldView {
	setProgress := make(map[string]domain.Progress, len(e.catalogues))
	for _, cat := range e.catalogues {
		setProgress[cat.Code] = e.collection.BinderProgress(cat)
	}
	fees := make(map[string]domain.Coins, len(e.boosterFees))
	for k, v := range e.boosterFees {
		fees[k] = v
	}
	// Copy boosterCosts so the policy cannot mutate the engine's map.
	costs := make(map[string]int, len(e.boosterCosts))
	for k, v := range e.boosterCosts {
		costs[k] = v
	}
	topupMinEnergy := 0
	if e.coinTopupOn {
		topupMinEnergy = e.coinTopupMinEnergy
	}
	topupStep := domain.Coins(0)
	if e.coinTopupOn {
		topupStep = e.coinTopupStep
	}
	return WorldView{
		Day:                     day,
		Balance:                 e.journal.Balance(domain.AccountPlayer),
		DailyReward:             e.cfg.DailyReward,
		EnergyTotal:             e.energy.Reserve() + e.energy.Savings(),
		SetProgress:             setProgress,
		BoosterCosts:            costs,
		BoosterFees:             fees,
		Catalogues:              e.catalogues,
		Prices:                  e.prices,
		OwnedCards:              e.collection.Snapshot(),
		CoinRate:                e.cfg.CoinRate,
		CounterBuyPct:           e.cfg.CounterBuyPct,
		CounterSellPct:          e.cfg.CounterSellPct,
		CounterFoilPct:          e.cfg.CounterFoilPct,
		FallbackPrice:           e.cfg.FallbackPrice,
		CoinTopupEnabled:        e.coinTopupOn,
		CoinTopupMinEnergyUnits: topupMinEnergy,
		CoinTopupRateNow:        e.coinTopupRateNow(),
		CoinTopupEscalationStep: topupStep,
	}
}

// port implements Session. One port is created per session tick.
// fatalErr captures any non-ErrSubCoin domain error; Run checks it after
// the policy returns and aborts the simulation on failure.
type port struct {
	e        *Engine
	day      int
	fatalErr error
}

func (p *port) View() WorldView {
	return p.e.buildWorldView(p.day)
}

// OpenBooster spends energy and opens one booster from setCode, adding its
// cards to the player's collection. Energy-insufficient errors are returned
// as normal (non-fatal) guards; catalogue or opener errors are fatal.
func (p *port) OpenBooster(setCode string) error {
	var cat *domain.SetCatalogue
	for i := range p.e.catalogues {
		if p.e.catalogues[i].Code == setCode {
			cat = &p.e.catalogues[i]
			break
		}
	}
	if cat == nil {
		err := fmt.Errorf("open booster: unknown set %q", setCode)
		p.fatalErr = err
		return err
	}

	cost, ok := p.e.boosterCosts[setCode]
	if !ok {
		err := fmt.Errorf("open booster: no cost for set %q", setCode)
		p.fatalErr = err
		return err
	}

	// The balance is checked first so that a refusal for coins never costs energy, and the fee is
	// posted last so that a refusal for energy never costs coins.
	fee := p.e.boosterFees[setCode]
	if balance := p.e.journal.Balance(domain.AccountPlayer); balance < fee {
		// A player who cannot pay the fee is a normal policy guard, like one short of energy.
		return fmt.Errorf("open booster %q: fee of %d coins, balance %d: %w", setCode, fee, balance, domain.ErrInsufficientBalance)
	}
	if err := p.e.energy.Spend(cost); err != nil {
		// Insufficient energy is a normal policy guard, not a fatal error.
		return err
	}
	if fee > 0 {
		if err := p.e.journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountBoosterFee, Amount: fee, Memo: "booster fee: set " + setCode}); err != nil {
			p.fatalErr = fmt.Errorf("open booster %q: post fee: %w", setCode, err)
			return p.fatalErr
		}
		p.e.dayBoosterFees += fee
	}

	booster, err := p.e.opener.Open(domain.BoosterLayoutClassic, *cat)
	if err != nil {
		// Opener failure means a corrupt catalogue; treat as fatal.
		// Energy has no refund method; the spent amount is accepted as a loss.
		refundErr := fmt.Errorf("open booster for %q: %w", setCode, err)
		p.fatalErr = refundErr
		return refundErr
	}

	p.e.collection.AddBooster(booster)
	p.e.dayBoostersOpened++
	return nil
}

// SellDuplicates sells all copies above keep to the Counter, one copy at a time.
// Each sell is atomic: remove from collection then sell; on ErrSubCoin or ErrCounterBroke
// refusal the copy is restored and the refusal is counted. After an ErrCounterBroke the
// remaining copies of that card are not offered, so a broke Counter is asked once per card
// instead of once per copy. Any other error from the Counter is fatal.
func (p *port) SellDuplicates(keep int) (sold, refused int) {
	surplus, err := p.e.collection.DuplicatesAbove(keep)
	if err != nil {
		p.fatalErr = fmt.Errorf("sell duplicates: %w", err)
		return 0, 0
	}

	for _, s := range surplus {
		for range s.Quantity {
			if err := p.e.collection.Remove(s.Key, 1); err != nil {
				p.fatalErr = fmt.Errorf("sell duplicates: remove: %w", err)
				return sold, refused
			}
			if err := p.e.counter.BuyFromPlayer(s.Key); err != nil {
				if errors.Is(err, domain.ErrSubCoin) || errors.Is(err, domain.ErrCounterBroke) {
					// Restore and count as refused. A Counter that is out of coins refuses like one
					// that finds the card worth less than a coin.
					if addErr := p.e.collection.Add(s.Key, 1); addErr != nil {
						p.fatalErr = fmt.Errorf("sell duplicates: restore after refusal: %w", addErr)
						return sold, refused
					}
					refused++
					p.e.dayTradesRefused++
					if errors.Is(err, domain.ErrCounterBroke) {
						p.e.dayBrokeRefusals++
						// The other copies of this card carry the same price and would be refused too:
						// leave them with the player and move on to the next card, which a cheaper
						// price may still let the Counter pay for.
						break
					}
				} else {
					p.fatalErr = fmt.Errorf("sell duplicates: counter buy: %w", err)
					return sold, refused
				}
			} else {
				sold++
				p.e.dayCardsSold++
			}
		}
	}
	return sold, refused
}

// BuyCard purchases one copy of key from the Counter and adds it to the
// collection. ErrOutOfStock, ErrInsufficientBalance, and ErrSubCoin are
// non-fatal (caller skips); all other errors are fatal.
func (p *port) BuyCard(key domain.CardKey) error {
	if err := p.e.counter.SellToPlayer(key); err != nil {
		if errors.Is(err, domain.ErrOutOfStock) ||
			errors.Is(err, domain.ErrInsufficientBalance) ||
			errors.Is(err, domain.ErrSubCoin) {
			return err
		}
		p.fatalErr = fmt.Errorf("buy card: %w", err)
		return err
	}
	if err := p.e.collection.Add(key, 1); err != nil {
		p.fatalErr = fmt.Errorf("buy card: add to collection: %w", err)
		return err
	}
	p.e.dayCardsBought++
	return nil
}

// OpenBoosterTopUp opens one booster from setCode, paying coins for any energy shortfall.
// The player spends all available real energy up to the booster's cost; any remaining cost is
// paid in coins at the current top-up rate. The player must have at least coinTopupMinEnergy
// real energy, so opening with coins only is never allowed.
//
// When top-up is disabled (coinTopupOn == false), this delegates to OpenBooster.
func (p *port) OpenBoosterTopUp(setCode string) error {
	if !p.e.coinTopupOn {
		return p.OpenBooster(setCode)
	}

	var cat *domain.SetCatalogue
	for i := range p.e.catalogues {
		if p.e.catalogues[i].Code == setCode {
			cat = &p.e.catalogues[i]
			break
		}
	}
	if cat == nil {
		err := fmt.Errorf("open booster with top-up: unknown set %q", setCode)
		p.fatalErr = err
		return err
	}

	cost, ok := p.e.boosterCosts[setCode]
	if !ok {
		err := fmt.Errorf("open booster with top-up: no cost for set %q", setCode)
		p.fatalErr = err
		return err
	}

	// Real energy the player can spend toward this booster's cost.
	available := p.e.energy.Reserve() + p.e.energy.Savings()
	realEnergy := available
	if realEnergy > cost {
		realEnergy = cost
	}

	// Minimum real energy check: cannot open with coins only.
	if realEnergy < p.e.coinTopupMinEnergy {
		return fmt.Errorf("open booster %q with top-up: have %d energy, need at least %d real energy (min_energy_units)",
			setCode, available, p.e.coinTopupMinEnergy)
	}

	// If no shortfall, open normally.
	missingUnits := cost - realEnergy
	if missingUnits == 0 {
		return p.OpenBooster(setCode)
	}

	// Compute coin cost for the shortfall.
	topUpCoins := p.e.computeTopUpCost(missingUnits)

	// Mandatory fee (ADR-0019), if any.
	fee := p.e.boosterFees[setCode]

	// Balance check: must cover top-up AND mandatory fee.
	if balance := p.e.journal.Balance(domain.AccountPlayer); balance < topUpCoins+fee {
		return fmt.Errorf("open booster %q with top-up: need %d coins (top-up %d + fee %d), balance %d: %w",
			setCode, topUpCoins+fee, topUpCoins, fee, balance, domain.ErrInsufficientBalance)
	}

	// Spend real energy first (we've verified sufficient energy above).
	if err := p.e.energy.Spend(realEnergy); err != nil {
		p.fatalErr = fmt.Errorf("open booster %q with top-up: spend energy: %w", setCode, err)
		return p.fatalErr
	}

	// Pay top-up coins into the sink.
	if err := p.e.journal.Post(domain.Entry{
		From:   domain.AccountPlayer,
		To:     domain.AccountCoinTopup,
		Amount: topUpCoins,
		Memo:   fmt.Sprintf("coin top-up: %d units for set %s", missingUnits, setCode),
	}); err != nil {
		p.fatalErr = fmt.Errorf("open booster %q with top-up: post top-up: %w", setCode, err)
		return p.fatalErr
	}
	p.e.dayCoinTopupUnits += missingUnits
	p.e.dayCoinTopupCoins += topUpCoins
	p.e.coinTopupDayUsed += missingUnits

	// Pay mandatory fee if any.
	if fee > 0 {
		if err := p.e.journal.Post(domain.Entry{
			From:   domain.AccountPlayer,
			To:     domain.AccountBoosterFee,
			Amount: fee,
			Memo:   "booster fee: set " + setCode,
		}); err != nil {
			p.fatalErr = fmt.Errorf("open booster %q with top-up: post fee: %w", setCode, err)
			return p.fatalErr
		}
		p.e.dayBoosterFees += fee
	}

	// Open the booster.
	booster, err := p.e.opener.Open(domain.BoosterLayoutClassic, *cat)
	if err != nil {
		p.fatalErr = fmt.Errorf("open booster with top-up for %q: %w", setCode, err)
		return p.fatalErr
	}

	p.e.collection.AddBooster(booster)
	p.e.dayBoostersOpened++
	return nil
}
