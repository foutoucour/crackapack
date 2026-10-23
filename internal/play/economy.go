package play

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// gameDay is the length of one day of the Counter's supply and of the NPCs: a day of real time,
// TicksPerDay ticks of tickLength.
const gameDay = domain.TicksPerDay * tickLength

// CounterRules is how the Counter of a game trades and stocks itself, from its configuration, see
// docs/core-features.md#the-counter.
type CounterRules struct {
	// Params are the Counter's rates, initial stock and shelf life, and whether it has a treasury.
	Params domain.CounterParams
	// Prices values the cards; a card it lacks is worth Params.FallbackPrice.
	Prices domain.PriceSnapshot
	// WelcomeBonus is the coins a new game starts with.
	WelcomeBonus domain.Coins
	// Treasury is the coins a Counter with a treasury starts with, and DailyFloat what it gets each
	// day caught up.
	Treasury, DailyFloat domain.Coins
	// Supply is the boosters the Counter opens to stock itself. The zero value opens none.
	Supply SupplyRules
	// Crowd is the NPCs who sell the Counter the copies above what they keep. Size 0 is none. Its
	// archetypes must not trade: the state of a crowd that trades is not saved yet.
	Crowd domain.PopulationParams
	// CatchUpDays is the most days caught up when the game is played again: the most recent ones,
	// the days before them being skipped, so that a long time away cannot hold up the game.
	CatchUpDays int
}

// SupplyRules is how the Counter opens boosters, as a special player, see
// docs/simulation.md#the-counter-as-a-special-player: a fixed number a day, or from an energy of
// its own that pays the energy cost of each set, never both.
type SupplyRules struct {
	// BoostersPerDay is the boosters opened each day, with no energy, when above zero.
	BoostersPerDay int
	// Energy is the Counter's own energy, when it opens a booster whenever it can pay for one.
	Energy *domain.EnergyParams
}

// economy is the coins of a game and the Counter's side of it: the ledger, the Counter, and the
// supply and the crowd that stock it, each on a random stream of its own that the save keeps.
// Every method needs the lock of its Game.
type economy struct {
	rules   CounterRules
	journal *domain.Journal
	counter *domain.Counter
	// supply and crowd are nil when the Counter opens no booster and when there are no NPCs.
	supply       *domain.CounterSupply
	supplyStream *Stream
	crowd        *domain.Crowd
	crowdStream  *Stream
	// syncedAt is the start of the first day not caught up yet.
	syncedAt time.Time
	// at is the instant of the tick a catch-up is replaying, which the Counter remembers a card
	// by, and the zero time outside a catch-up, when the Counter reads now.
	at  time.Time
	now Clock
}

// OpenCounter gives the game coins and a Counter under rules: the welcome bonus, an initial stock,
// and a supply and a crowd that stock it, on the random streams supply and crowd. It is called once,
// after NewGame and Stage and before Persist. A game without OpenCounter has no coins.
func (g *Game) OpenCounter(rules CounterRules, supply, crowd *Stream) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if rules.CatchUpDays < 1 {
		return fmt.Errorf("play: the Counter catches up on %d days, want at least 1", rules.CatchUpDays)
	}
	e, err := g.newEconomy(rules, supply, crowd)
	if err != nil {
		return err
	}
	if rules.WelcomeBonus > 0 {
		if err := e.journal.Post(domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: rules.WelcomeBonus, Memo: "welcome bonus"}); err != nil {
			return fmt.Errorf("play: welcome bonus: %w", err)
		}
	}
	if err := domain.FundTreasury(e.journal, rules.Treasury, "initial treasury"); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	g.economy = e
	return nil
}

// newEconomy builds the Counter of the game's sets and pools, with its supply and crowd on the
// streams given, as they stand at the start of a game: the crowd draws who its NPCs are from crowd.
// The ledger is empty. g.mu must be held.
func (g *Game) newEconomy(rules CounterRules, supplyStream, crowdStream *Stream) (*economy, error) {
	journal := domain.NewJournal()
	if rules.Params.Treasury {
		journal = domain.NewJournal(domain.AccountPlayer, domain.AccountCounterTreasury)
	}
	e := &economy{rules: rules, journal: journal, supplyStream: supplyStream, crowdStream: crowdStream, syncedAt: g.now(), now: g.now}
	cats := append(append([]domain.SetCatalogue(nil), g.sets...), g.pools...)
	counter, err := domain.NewCounter(cats, rules.Prices, journal, rules.Params, domain.WithLastInStock(e.instant))
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	e.counter = counter
	if e.supply, err = g.newSupply(rules.Supply, counter, supplyStream); err != nil {
		return nil, err
	}
	if rules.Crowd.Size > 0 {
		rng := rand.New(crowdStream)
		npcs, err := domain.GeneratePopulation(rules.Crowd, g.sets, rng)
		if err != nil {
			return nil, fmt.Errorf("play: %w", err)
		}
		if e.crowd, err = domain.NewCrowd(npcs, g.sets, newClassicOpener(rng), counter, rng); err != nil {
			return nil, fmt.Errorf("play: %w", err)
		}
		if _, err := e.crowd.State(); err != nil {
			return nil, fmt.Errorf("play: %w", err)
		}
	}
	return e, nil
}

// newSupply builds the supply rules describe, or nil when the Counter opens no booster. The energy
// supply pays the player's energy costs. g.mu must be held.
func (g *Game) newSupply(rules SupplyRules, counter *domain.Counter, stream *Stream) (*domain.CounterSupply, error) {
	if rules.BoostersPerDay > 0 && rules.Energy != nil {
		return nil, errors.New("play: the Counter opens a fixed number of boosters or from its energy, not both")
	}
	rng := rand.New(stream)
	var (
		supply *domain.CounterSupply
		err    error
	)
	switch {
	case rules.BoostersPerDay > 0:
		supply, err = domain.NewFixedCounterSupply(counter, g.sets, newClassicOpener(rng), rng, rules.BoostersPerDay)
	case rules.Energy != nil:
		var energy *domain.Energy
		if energy, err = domain.NewEnergy(*rules.Energy); err == nil {
			supply, err = domain.NewEnergyCounterSupply(counter, g.sets, newClassicOpener(rng), rng, energy, g.rules.Costs)
		}
	default:
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	return supply, nil
}

func newClassicOpener(rng *rand.Rand) *domain.Opener {
	return domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
}

// instant is the clock of the Counter: the tick being replayed during a catch-up, now otherwise.
func (e *economy) instant() time.Time {
	if !e.at.IsZero() {
		return e.at
	}
	return e.now()
}

// catchUp plays the whole days gone by since syncedAt, as the simulation steps a day: the float of
// a treasury, then each tick the Counter's shelf, its supply and the NPCs, in that order. Only the
// most recent CatchUpDays are played, and the days before them are skipped. A part of a day is
// left for later. An error is a catalogue that does not hold a card its boosters draw.
func (e *economy) catchUp() error {
	days := int(e.now().Sub(e.syncedAt) / gameDay)
	if days <= 0 {
		return nil
	}
	played := min(days, e.rules.CatchUpDays)
	first := e.syncedAt.Add(time.Duration(days-played) * gameDay)
	defer func() { e.at = time.Time{} }()
	for d := range played {
		start := first.Add(time.Duration(d) * gameDay)
		if err := domain.FundTreasury(e.journal, e.rules.DailyFloat, "daily float"); err != nil {
			return fmt.Errorf("play: catch up: %w", err)
		}
		for tick := range domain.TicksPerDay {
			e.at = start.Add(time.Duration(tick+1) * tickLength)
			e.counter.Tick()
			if e.supply != nil {
				if _, err := e.supply.Tick(); err != nil {
					return fmt.Errorf("play: catch up: %w", err)
				}
			}
			if e.crowd != nil {
				if _, err := e.crowd.Tick(); err != nil {
					return fmt.Errorf("play: catch up: %w", err)
				}
			}
		}
	}
	e.syncedAt = e.syncedAt.Add(time.Duration(days) * gameDay)
	return nil
}
