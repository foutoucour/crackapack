package sim

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// Engine runs the headless simulation. Construct with NewEngine; the zero value
// is not ready to use.
//
// Run may be called at most once: the engine mutates its domain objects
// (journal, energy, collection, counter) and there is no reset path.
type Engine struct {
	cfg           config.Config
	catalogues    []domain.SetCatalogue // openable sets only
	supplementary []domain.SetCatalogue // supplementary pools (Counter-only, no binder progress)
	prices        domain.PriceSnapshot
	journal       *domain.Journal
	energy        *domain.Energy
	collection    *domain.Collection
	counter       *domain.Counter
	opener        *domain.Opener
	policy        PlayerPolicy
	boosterCosts  map[string]int // setCode -> energy cost; openable sets only
	ran           bool           // guards against calling Run more than once

	// per-day session counters; reset at the start of each day
	dayBoostersOpened int
	dayCardsSold      int
	dayCardsBought    int
	dayTradesRefused  int
}

// NewEngine builds and validates a simulation engine.
//
// catalogues are the openable sets: their cards are tracked in progress, metrics,
// target-set selection and energy cost computation. supplementary holds extra
// catalogues whose cards must be known to the Counter (e.g. TSB timeshifted
// cards referenced by TSP booster sheets) but do not contribute to metrics.
//
// prices must include all cards in both catalogues and supplementary; absent
// cards fall back to cfg.FallbackPrice.
//
// rng must be a freshly seeded generator built by the caller:
//
//	rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
func NewEngine(
	cfg config.Config,
	catalogues []domain.SetCatalogue,
	supplementary []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	policy PlayerPolicy,
	rng *rand.Rand,
) (*Engine, error) {
	if len(catalogues) == 0 {
		return nil, fmt.Errorf("sim: at least one openable catalogue is required")
	}
	if policy == nil {
		return nil, fmt.Errorf("sim: policy must not be nil")
	}
	if rng == nil {
		return nil, fmt.Errorf("sim: rng must not be nil")
	}

	// Validate that the price snapshot matches at least one catalogue card.
	// A snapshot that matches no card at all is almost certainly keyed with the
	// wrong identifiers; fail fast rather than silently using fallback_price for
	// every card.
	{
		var totalNonBasic, priced int
		allForCheck := make([]domain.SetCatalogue, 0, len(catalogues)+len(supplementary))
		allForCheck = append(allForCheck, catalogues...)
		allForCheck = append(allForCheck, supplementary...)
		for _, cat := range allForCheck {
			for id, card := range cat.Cards {
				if !card.IsBasicLand {
					totalNonBasic++
					if _, ok := prices[id]; ok {
						priced++
					}
				}
			}
		}
		if totalNonBasic > 0 && priced == 0 {
			return nil, fmt.Errorf("sim: price snapshot matches none of the %d catalogue cards: check that it is keyed by MTGJSON uuid", totalNonBasic)
		}
	}

	// Build energy.
	energy, err := domain.NewEnergy(domain.EnergyParams{
		RechargePerTick:      cfg.Energy.RechargePerTick,
		ReserveCap:           cfg.Energy.ReserveCap,
		SavingsIntervalTicks: cfg.Energy.SavingsIntervalTicks,
		SavingsCap:           cfg.Energy.SavingsCap,
	})
	if err != nil {
		return nil, fmt.Errorf("sim: energy: %w", err)
	}

	// Build journal.
	journal := domain.NewJournal()

	// Build counter: index both openable and supplementary catalogues.
	allCats := make([]domain.SetCatalogue, 0, len(catalogues)+len(supplementary))
	allCats = append(allCats, catalogues...)
	allCats = append(allCats, supplementary...)

	counter, err := domain.NewCounter(allCats, prices, journal, domain.CounterParams{
		SellPct:       cfg.CounterSellPct,
		BuyPct:        cfg.CounterBuyPct,
		CoinRate:      cfg.CoinRate,
		FallbackPrice: cfg.FallbackPrice,
		InitialStock:  cfg.InitialStock,
	})
	if err != nil {
		return nil, fmt.Errorf("sim: counter: %w", err)
	}

	// Build opener with the classic layout.
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{
			domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
		},
		rng,
	)

	// Compute booster energy costs for each openable catalogue.
	// 1. average value per catalogue
	avgValues := make([]domain.USDCents, 0, len(catalogues))
	for _, cat := range catalogues {
		avg, err := domain.BoosterAverageValue(cat, prices, cfg.FallbackPrice)
		if err != nil {
			return nil, fmt.Errorf("sim: booster average value for %q: %w", cat.Code, err)
		}
		avgValues = append(avgValues, avg)
	}
	// 2. median across catalogues
	median, err := domain.MedianValue(avgValues)
	if err != nil {
		return nil, fmt.Errorf("sim: median booster value: %w", err)
	}
	// 3. cost tier per catalogue
	tiers := make([]domain.CostTier, len(cfg.Energy.CostTable))
	for i, e := range cfg.Energy.CostTable {
		tiers[i] = domain.CostTier{Threshold: e.Threshold, Cost: e.Cost}
	}
	boosterCosts := make(map[string]int, len(catalogues))
	for i, cat := range catalogues {
		cost, err := domain.EnergyCost(avgValues[i], median, tiers)
		if err != nil {
			return nil, fmt.Errorf("sim: energy cost for %q: %w", cat.Code, err)
		}
		boosterCosts[cat.Code] = cost
	}

	// Post welcome bonus before the simulation begins.
	if cfg.WelcomeBonus > 0 {
		if err := journal.Post(domain.Entry{
			From:   domain.AccountWelcomeBonus,
			To:     domain.AccountPlayer,
			Amount: cfg.WelcomeBonus,
			Memo:   "welcome bonus",
		}); err != nil {
			return nil, fmt.Errorf("sim: post welcome bonus: %w", err)
		}
	}

	return &Engine{
		cfg:           cfg,
		catalogues:    catalogues,
		supplementary: supplementary,
		prices:        prices,
		journal:       journal,
		energy:        energy,
		collection:    domain.NewCollection(),
		counter:       counter,
		opener:        opener,
		policy:        policy,
		boosterCosts:  boosterCosts,
	}, nil
}

// Run executes the simulation for cfg.RunDays days and returns the Result.
//
// Each day posts the daily reward, then runs 144 ticks. Sessions are spread
// evenly: session i fires at tick floor(i * TicksPerDay / SessionsPerDay).
// Energy ticks every tick; the player acts only in session ticks.
//
// Any domain error other than ErrSubCoin aborts Run and is returned with context.
//
// Run may be called at most once. A second call returns an error immediately,
// because the engine's domain objects are mutated and cannot be reset.
func (e *Engine) Run() (Result, error) {
	if e.ran {
		return Result{}, fmt.Errorf("sim: Engine.Run called more than once; construct a new Engine for each run")
	}
	e.ran = true

	days := make([]DayMetrics, 0, e.cfg.RunDays)

	// Compute session tick offsets once (same every day).
	sessionTicks := make([]int, e.cfg.SessionsPerDay)
	for i := range e.cfg.SessionsPerDay {
		sessionTicks[i] = i * TicksPerDay / e.cfg.SessionsPerDay
	}
	sessionSet := make(map[int]struct{}, len(sessionTicks))
	for _, t := range sessionTicks {
		sessionSet[t] = struct{}{}
	}

	for day := 1; day <= e.cfg.RunDays; day++ {
		// Daily reward.
		if e.cfg.DailyReward > 0 {
			if err := e.journal.Post(domain.Entry{
				From:   domain.AccountDailyReward,
				To:     domain.AccountPlayer,
				Amount: e.cfg.DailyReward,
				Memo:   fmt.Sprintf("daily reward day %d", day),
			}); err != nil {
				return Result{}, fmt.Errorf("sim: day %d: post daily reward: %w", day, err)
			}
		}

		// Reset day counters.
		e.dayBoostersOpened = 0
		e.dayCardsSold = 0
		e.dayCardsBought = 0
		e.dayTradesRefused = 0

		// Tick loop.
		for tick := range TicksPerDay {
			if err := e.energy.Tick(1); err != nil {
				return Result{}, fmt.Errorf("sim: day %d tick %d: energy tick: %w", day, tick, err)
			}
			if _, isSession := sessionSet[tick]; isSession {
				port := &port{e: e, day: day}
				e.policy.RunSession(port)
				if port.fatalErr != nil {
					return Result{}, fmt.Errorf("sim: day %d tick %d: session: %w", day, tick, port.fatalErr)
				}
			}
		}

		// Snapshot metrics.
		dm, err := e.snapshotDay(day)
		if err != nil {
			return Result{}, fmt.Errorf("sim: day %d: snapshot: %w", day, err)
		}
		days = append(days, dm)
	}

	summary := computeSummary(
		days,
		e.cfg.RunDays,
		e.catalogues,
		e.supplementary,
		e.prices,
		e.cfg.FallbackPrice,
		e.cfg.CoinRate,
		e.cfg.CounterSellPct,
		e.cfg.CounterBuyPct,
		e.cfg.DailyReward,
		e.collection,
	)

	return Result{Days: days, Summary: summary}, nil
}

// snapshotDay captures the end-of-day metrics.
func (e *Engine) snapshotDay(day int) (DayMetrics, error) {
	// Money supply: snapshot all known accounts.
	accounts := []domain.Account{
		domain.AccountPlayer,
		domain.AccountWelcomeBonus,
		domain.AccountDailyReward,
		domain.AccountCounterSale,
		domain.AccountCounterPurchase,
	}
	supply := make(map[domain.Account]domain.Coins, len(accounts))
	for _, a := range accounts {
		supply[a] = e.journal.Balance(a)
	}

	// Counter stock by rarity.
	stock := e.counter.StockByRarity()

	// Collection value.
	collUSD := e.collection.Value(e.prices, e.cfg.FallbackPrice)
	collCoins, err := e.collection.CoinValue(e.prices, e.cfg.FallbackPrice, e.cfg.CoinRate)
	if err != nil {
		if errors.Is(err, domain.ErrSubCoin) {
			collCoins = 0
		} else {
			return DayMetrics{}, err
		}
	}

	// Per-set binder progress (openable sets only).
	setProgress := make(map[string]SetDayProgress, len(e.catalogues))
	for _, cat := range e.catalogues {
		p := e.collection.BinderProgress(cat)
		cuOwned := p.ByRarity[domain.RarityCommon].Owned + p.ByRarity[domain.RarityUncommon].Owned
		cuTotal := p.ByRarity[domain.RarityCommon].Total + p.ByRarity[domain.RarityUncommon].Total
		setProgress[cat.Code] = SetDayProgress{
			Owned:    p.Owned,
			Total:    p.Total,
			CUOwned:  cuOwned,
			CUTotal:  cuTotal,
			Playsets: p.Playsets,
		}
	}

	return DayMetrics{
		Day:             day,
		MoneySupply:     supply,
		StockByRarity:   stock,
		CollectionUSD:   collUSD,
		CollectionCoins: collCoins,
		SetProgress:     setProgress,
		BoostersOpened:  e.dayBoostersOpened,
		CardsSold:       e.dayCardsSold,
		CardsBought:     e.dayCardsBought,
		TradesRefused:   e.dayTradesRefused,
	}, nil
}
