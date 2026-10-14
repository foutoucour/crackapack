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
	crowd         *domain.Crowd         // nil in a run without NPCs
	supply        *domain.CounterSupply // nil unless the Counter opens boosters
	exchange      *domain.Exchange      // nil unless trades are enabled
	opener        *domain.Opener
	policy        PlayerPolicy
	boosterCosts  map[string]int // setCode -> energy cost; openable sets only
	maxWait       float64        // max_booster_wait_minutes of the run
	ran           bool           // guards against calling Run more than once

	// per-day session counters; reset at the start of each day
	dayBoostersOpened int
	dayCardsSold      int
	dayCardsBought    int
	dayTradesRefused  int
	dayNPCBoosters    int
	dayNPCSold        int
	daySupplyBoosters int
	daySupplyCards    int
}

type options struct {
	reference []domain.SetCatalogue
}

// Option tunes NewEngine.
type Option func(*options)

// WithReferenceCatalogues sets the catalogues whose median booster value is the reference of the
// energy cost tiers. It must include the openable catalogues of the run, so that a set costs the
// same whichever sets are opened alongside it. Without it, or when empty, the reference is the
// openable catalogues themselves and a set run alone always lands in the x1 tier.
func WithReferenceCatalogues(reference []domain.SetCatalogue) Option {
	return func(o *options) { o.reference = reference }
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
	opts ...Option,
) (*Engine, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
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

	// A reference set no price covers would be valued at fallback_price and collapse the median,
	// moving every energy cost a tier without a sign.
	if err := checkReferencePriced(o.reference, catalogues, prices); err != nil {
		return nil, err
	}

	// Build energy.
	energyParams := domain.EnergyParams{
		RechargePerTick:      cfg.Energy.RechargePerTick,
		ReserveCap:           cfg.Energy.ReserveCap,
		SavingsIntervalTicks: cfg.Energy.SavingsIntervalTicks,
		SavingsCap:           cfg.Energy.SavingsCap,
	}
	energy, err := domain.NewEnergy(energyParams)
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
		ShelfLife:     shelfLife(cfg.CounterShelfDays),
	})
	if err != nil {
		return nil, fmt.Errorf("sim: counter: %w", err)
	}

	valuer := domain.NewValuer(prices, cfg.FallbackPrice)
	crowd, err := newCrowd(cfg, catalogues, counter, valuer)
	if err != nil {
		return nil, err
	}
	collection := domain.NewCollection()
	exchange, err := newExchange(cfg, crowd, collection, journal, valuer)
	if err != nil {
		return nil, err
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
	avgValues, err := boosterAverageValues(catalogues, prices, cfg.FallbackPrice)
	if err != nil {
		return nil, err
	}
	// 2. median across the reference catalogues, by default the openable ones
	referenceValues := avgValues
	if len(o.reference) > 0 {
		referenceValues, err = boosterAverageValues(o.reference, prices, cfg.FallbackPrice)
		if err != nil {
			return nil, err
		}
	}
	median, err := domain.MedianValue(referenceValues)
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
	supply, err := newCounterSupply(cfg, catalogues, counter, boosterCosts)
	if err != nil {
		return nil, err
	}
	maxWait, err := maxBoosterWaitMinutes(energyParams, boosterCosts)
	if err != nil {
		return nil, fmt.Errorf("sim: %w", err)
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
		collection:    collection,
		counter:       counter,
		crowd:         crowd,
		supply:        supply,
		exchange:      exchange,
		opener:        opener,
		policy:        policy,
		boosterCosts:  boosterCosts,
		maxWait:       maxWait,
	}, nil
}

// Run executes the simulation for cfg.RunDays days and returns the Result.
//
// Each day posts the daily reward, then runs 144 ticks. Sessions are spread
// evenly: session i fires at tick floor(i * TicksPerDay / SessionsPerDay).
// Energy ticks every tick and the NPC crowd acts every tick, before the player; the player
// acts only in session ticks.
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
		e.dayNPCBoosters = 0
		e.dayNPCSold = 0
		e.daySupplyBoosters = 0
		e.daySupplyCards = 0

		if e.exchange != nil {
			e.exchange.StartDay(day)
		}

		// Tick loop.
		for tick := range TicksPerDay {
			if err := e.energy.Tick(1); err != nil {
				return Result{}, fmt.Errorf("sim: day %d tick %d: energy tick: %w", day, tick, err)
			}
			e.counter.Tick()
			if err := e.tickSupply(); err != nil {
				return Result{}, fmt.Errorf("sim: day %d tick %d: %w", day, tick, err)
			}
			if err := e.tickCrowd(); err != nil {
				return Result{}, fmt.Errorf("sim: day %d tick %d: %w", day, tick, err)
			}
			if err := e.tickTrades(); err != nil {
				return Result{}, fmt.Errorf("sim: day %d tick %d: %w", day, tick, err)
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
	summary["max_booster_wait_minutes"] = e.maxWait
	if e.crowd != nil {
		addNPCSummary(summary, days)
	}
	if e.exchange != nil {
		addTradeSummary(summary, days)
	}
	if e.supply != nil {
		addCounterSummary(summary, days)
	}

	return Result{Days: days, Summary: summary}, nil
}

// shelfLife maps the counter_shelf_days setting to the Counter's ShelfLife: a negative value keeps
// cards forever.
func shelfLife(days int) domain.ShelfLife {
	if days < 0 {
		return domain.ForeverOnShelf()
	}
	return domain.VanishAfterDays(days)
}

// tickCrowd advances the NPC crowd by one tick and books what it did into the day counters.
// It does nothing in a run without NPCs.
func (e *Engine) tickCrowd() error {
	if e.crowd == nil {
		return nil
	}
	tick, err := e.crowd.Tick()
	if err != nil {
		return err
	}
	e.dayNPCBoosters += tick.BoostersOpened
	e.dayNPCSold += tick.CardsSold
	return nil
}

// tickSupply advances the Counter's supply by one tick and books what it opened into the day
// counters. It does nothing when the Counter opens no boosters.
func (e *Engine) tickSupply() error {
	if e.supply == nil {
		return nil
	}
	tick, err := e.supply.Tick()
	if err != nil {
		return err
	}
	e.daySupplyBoosters += tick.BoostersOpened
	e.daySupplyCards += tick.CardsOpened
	return nil
}

// counterMetrics returns what the Counter opened on the day just ended, or nil when it opens
// nothing.
func (e *Engine) counterMetrics() *CounterDayMetrics {
	if e.supply == nil {
		return nil
	}
	return &CounterDayMetrics{BoostersOpened: e.daySupplyBoosters, CardsOpened: e.daySupplyCards}
}

// npcMetrics returns the crowd's metrics for the day just ended, or nil without a crowd.
func (e *Engine) npcMetrics() *NPCDayMetrics {
	if e.crowd == nil {
		return nil
	}
	return &NPCDayMetrics{BoostersOpened: e.dayNPCBoosters, CardsSold: e.dayNPCSold, CardsHeld: e.crowd.CardsHeld()}
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
	if e.exchange != nil {
		accounts = append(accounts, domain.AccountTradeTopUp)
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
		NPC:             e.npcMetrics(),
		Counter:         e.counterMetrics(),
		Trade:           e.tradeMetrics(),
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

// checkReferencePriced returns an error naming the first reference catalogue, other than the
// openable ones, whose non-basic cards are all absent from the price snapshot. Partial coverage
// is not checked.
func checkReferencePriced(reference, openable []domain.SetCatalogue, prices domain.PriceSnapshot) error {
	isOpenable := make(map[string]bool, len(openable))
	for _, cat := range openable {
		isOpenable[cat.Code] = true
	}
	for _, cat := range reference {
		if isOpenable[cat.Code] {
			continue
		}
		var nonBasic, priced int
		for id, card := range cat.Cards {
			if card.IsBasicLand {
				continue
			}
			nonBasic++
			if _, ok := prices[id]; ok {
				priced++
			}
		}
		if nonBasic > 0 && priced == 0 {
			return fmt.Errorf("sim: price snapshot has no price for any card of the reference set %q: fetch the prices of every set of the catalogue directory, for example with one tools/fetchdata call for all of them", cat.Code)
		}
	}
	return nil
}

// boosterAverageValues returns the average booster value of each catalogue, in order.
func boosterAverageValues(cats []domain.SetCatalogue, prices domain.PriceSnapshot, fallback domain.USDCents) ([]domain.USDCents, error) {
	values := make([]domain.USDCents, 0, len(cats))
	for _, cat := range cats {
		avg, err := domain.BoosterAverageValue(cat, prices, fallback)
		if err != nil {
			return nil, fmt.Errorf("sim: booster average value for %q: %w", cat.Code, err)
		}
		values = append(values, avg)
	}
	return values, nil
}
