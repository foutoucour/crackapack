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
	// DailyReward is the coins the player is paid for each day caught up, see
	// docs/adr/0067-the-daily-reward-is-paid-for-each-day-the-game-catches-up.md. Zero pays none.
	DailyReward domain.Coins
	// Treasury is the coins a Counter with a treasury starts with, and DailyFloat what it gets each
	// day caught up.
	Treasury, DailyFloat domain.Coins
	// Supply is the boosters the Counter opens to stock itself. The zero value opens none.
	Supply SupplyRules
	// Crowd is the NPCs who sell the Counter the copies above what they keep, and who trade on the
	// market when there is one. Size 0 is none.
	Crowd domain.PopulationParams
	// Market is how the NPCs trade with the player, nil in a game without a market. It needs a
	// crowd.
	Market *MarketRules
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

// MarketRules is how the market of a game trades, see docs/core-features.md#trades-with-npcs.
type MarketRules struct {
	// SurplusHoldDays is how long an NPC keeps the copies above what it keeps on its shelf, where
	// the market can trade them, before it sells them to the Counter. It is at least 1.
	SurplusHoldDays int
	Params          domain.MarketParams
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
	// market is nil in a game without one.
	market *domain.Market
	// cold is true while the NPCs of a market are as generated, and are to be warmed up, see warmUp.
	cold bool
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
	e, err := g.newEconomy(rules, supply, crowd, g.collection)
	if err != nil {
		return err
	}
	g.openCrowdSets(e)
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

// newEconomy builds the Counter of the game's sets and pools, with its supply, its crowd and its
// market on the streams given, as they stand at the start of a game: the crowd draws who its NPCs
// are from crowd. The market trades with the player's collection. The ledger is empty. g.mu must be
// held.
func (g *Game) newEconomy(rules CounterRules, supplyStream, crowdStream *Stream, collection *domain.Collection) (*economy, error) {
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
		var opts []domain.CrowdOption
		if rules.Market != nil {
			opts = append(opts, domain.WithSurplusHold(rules.Market.SurplusHoldDays))
		}
		if e.crowd, err = domain.NewCrowd(npcs, g.sets, newClassicOpener(rng), counter, rng, opts...); err != nil {
			return nil, fmt.Errorf("play: %w", err)
		}
	}
	if rules.Market != nil {
		if e.crowd == nil {
			return nil, errors.New("play: a market needs NPCs to trade with")
		}
		if e.market, err = e.newMarket(collection); err != nil {
			return nil, err
		}
		e.cold = true
	}
	return e, nil
}

// newMarket is an empty market of e's NPCs that trades with collection.
func (e *economy) newMarket(collection *domain.Collection) (*domain.Market, error) {
	valuer := domain.NewValuer(e.rules.Prices, e.rules.Params.FallbackPrice)
	m, err := domain.NewMarket(e.rules.Market.Params, e.crowd, collection, e.journal, valuer)
	if err != nil {
		return nil, fmt.Errorf("play: %w", err)
	}
	return m, nil
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

// openCrowdSets makes the NPCs of e open only the sets the player can open, see
// docs/adr/0063-the-npcs-of-the-game-open-the-sets-the-player-can-open.md. g.mu must be held.
func (g *Game) openCrowdSets(e *economy) {
	if e != nil && e.crowd != nil {
		e.crowd.OpenOnly(func(code string) bool { return !g.locked(code) })
	}
}

// warmUp plays the NPCs of a market that are as generated, a new game's or those that replace a
// save's, for the days of their hold but one, a day at least, so that their shelves hold cards to
// trade from the first minute. A copy leaves a shelf once it has waited the whole hold, so the
// Counter gets none of their surplus before the game starts. It does nothing to NPCs that a save
// gave, so a game that loads one does not warm up NPCs it then throws away.
func (e *economy) warmUp() error {
	if !e.cold {
		return nil
	}
	e.cold = false
	days := max(1, e.rules.Market.SurplusHoldDays-1)
	for range days * domain.TicksPerDay {
		if _, err := e.crowd.Tick(); err != nil {
			return fmt.Errorf("play: warm the NPCs up: %w", err)
		}
	}
	return nil
}

// instant is the clock of the Counter: the tick being replayed during a catch-up, now otherwise.
func (e *economy) instant() time.Time {
	if !e.at.IsZero() {
		return e.at
	}
	return e.now()
}

// catchUp plays the whole days gone by since syncedAt, as the simulation steps a day: the player's
// daily reward, the float of a treasury, then each tick the Counter's shelf, its supply and the
// NPCs, in that order. Only the
// most recent CatchUpDays are played, and the days before them are skipped. A part of a day is
// left for later. An error is a catalogue that does not hold a card its boosters draw.
//
// The market's answers due on a day are delivered before the day is played, so that they see the
// NPCs as the day found them, as they would have live, and those due since the last whole day at
// the end.
func (e *economy) catchUp() error {
	if err := e.warmUp(); err != nil {
		return err
	}
	days := int(e.now().Sub(e.syncedAt) / gameDay)
	if days <= 0 {
		return e.advanceMarket(e.now())
	}
	played := min(days, e.rules.CatchUpDays)
	first := e.syncedAt.Add(time.Duration(days-played) * gameDay)
	defer func() { e.at = time.Time{} }()
	for d := range played {
		start := first.Add(time.Duration(d) * gameDay)
		// The last instant of the day: a day is played once its end has come.
		if err := e.advanceMarket(start.Add(gameDay - time.Nanosecond)); err != nil {
			return err
		}
		if err := e.payDailyReward(); err != nil {
			return fmt.Errorf("play: catch up: %w", err)
		}
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
	return e.advanceMarket(e.now())
}

// payDailyReward pays the player the reward of one day, if the rules give one.
func (e *economy) payDailyReward() error {
	if e.rules.DailyReward <= 0 {
		return nil
	}
	return e.journal.Post(domain.Entry{From: domain.AccountDailyReward, To: domain.AccountPlayer, Amount: e.rules.DailyReward, Memo: "daily reward"})
}

// advanceMarket delivers the market's answers due by at, in a game with a market.
func (e *economy) advanceMarket(at time.Time) error {
	if e.market == nil {
		return nil
	}
	if err := e.market.Advance(at); err != nil {
		return fmt.Errorf("play: market: %w", err)
	}
	return nil
}
