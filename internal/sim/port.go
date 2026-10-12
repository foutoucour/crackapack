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
	// Copy boosterCosts so the policy cannot mutate the engine's map.
	costs := make(map[string]int, len(e.boosterCosts))
	for k, v := range e.boosterCosts {
		costs[k] = v
	}
	return WorldView{
		Day:            day,
		Balance:        e.journal.Balance(domain.AccountPlayer),
		DailyReward:    e.cfg.DailyReward,
		EnergyTotal:    e.energy.Reserve() + e.energy.Savings(),
		SetProgress:    setProgress,
		BoosterCosts:   costs,
		Catalogues:     e.catalogues,
		Prices:         e.prices,
		OwnedCards:     e.collection.Snapshot(),
		CoinRate:       e.cfg.CoinRate,
		CounterBuyPct:  e.cfg.CounterBuyPct,
		CounterSellPct: e.cfg.CounterSellPct,
		FallbackPrice:  e.cfg.FallbackPrice,
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

	if err := p.e.energy.Spend(cost); err != nil {
		// Insufficient energy is a normal policy guard, not a fatal error.
		return err
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
// Each sell is atomic: remove from collection then sell; on ErrSubCoin refusal
// the copy is restored and the refusal is counted. Any other error from the
// Counter is fatal.
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
				if errors.Is(err, domain.ErrSubCoin) {
					// Restore and count as refused.
					if addErr := p.e.collection.Add(s.Key, 1); addErr != nil {
						p.fatalErr = fmt.Errorf("sell duplicates: restore after refusal: %w", addErr)
						return sold, refused
					}
					refused++
					p.e.dayTradesRefused++
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
