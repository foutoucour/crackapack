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
	supplementary []domain.SetCatalogue // supplementary pools: the Counter knows their cards
	binderCats    []domain.SetCatalogue // catalogues plus the pool cards their boosters draw: the binders
	prices        domain.PriceSnapshot
	journal       *domain.Journal
	energy        *domain.Energy
	collection    *domain.Collection
	counter       *domain.Counter
	crowd         *domain.Crowd         // nil in a run without NPCs
	supply        *domain.CounterSupply // nil unless the Counter opens boosters
	counterOn     bool                  // the run reports the Counter's own metrics
	exchange      *domain.Exchange      // nil unless trades are enabled
	opener        *domain.Opener
	policy        PlayerPolicy
	boosterCosts  map[string]int          // setCode -> energy cost; openable sets only
	boosterFees   map[string]domain.Coins // setCode -> coin fee the player pays; only sets that have one
	feeOn         bool                    // the run prices energy and so reports the player's fees
	maxWait       float64                 // max_booster_wait_minutes of the run
	ran           bool                    // guards against calling Run more than once

	// coin top-up state (coins substituted for missing energy when opening a booster)
	coinTopupOn        bool         // energy.coin_topup.enabled and coins_per_energy > 0
	coinTopupVariant   string       // "min_energy", "escalating", or "both"
	coinTopupMinEnergy int          // minimum real energy required (all variants)
	coinTopupStep      domain.Coins // additional coins per successive unit within a day (escalating and both)
	coinTopupDayUsed   int          // energy units bought with coins so far today (reset each day)

	// per-day session counters; reset at the start of each day
	dayBoostersOpened int
	dayCardsSold      int
	dayCardsBought    int
	dayTradesRefused  int
	dayNPCBoosters    int
	dayNPCSold        int
	daySupplyBoosters int
	daySupplyCards    int
	daySupplyFees     domain.Coins
	dayBrokeRefusals  int
	dayBoosterFees    domain.Coins
	dayCoinTopupUnits int
	dayCoinTopupCoins domain.Coins
}

type options struct {
	reference     []domain.SetCatalogue
	traitRegistry domain.TraitRegistry
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

// WithTraitRegistry sets the trait registry used by the NPC crowd. Without it the
// domain.DefaultTraits registry is used. Tests use this to inject test traits.
func WithTraitRegistry(registry domain.TraitRegistry) Option {
	return func(o *options) { o.traitRegistry = registry }
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

	// A sheet card no catalogue holds would make a run with a supply or a crowd fail late.
	if err := checkSheetCards(catalogues, supplementary); err != nil {
		return nil, err
	}

	// A reference set no price covers would be valued at fallback_price and collapse the median,
	// moving every energy cost a tier without a sign.
	if err := checkReferencePriced(o.reference, catalogues, prices); err != nil {
		return nil, err
	}

	// Build energy.
	energyParams := cfg.Energy.Params()
	energy, err := domain.NewEnergy(energyParams)
	if err != nil {
		return nil, fmt.Errorf("sim: energy: %w", err)
	}

	// Build journal. A Counter with a treasury must not be able to pay out more than it holds.
	journal := domain.NewJournal()
	if cfg.CounterBudget.Enabled {
		journal = domain.NewJournal(domain.AccountPlayer, domain.AccountCounterTreasury)
	}

	// Build counter: index both openable and supplementary catalogues.
	allCats := make([]domain.SetCatalogue, 0, len(catalogues)+len(supplementary))
	allCats = append(allCats, catalogues...)
	allCats = append(allCats, supplementary...)

	counter, err := domain.NewCounter(allCats, prices, journal, domain.CounterParams{
		SellPct:       cfg.CounterSellPct,
		BuyPct:        cfg.CounterBuyPct,
		FoilPct:       cfg.CounterFoilPct,
		CoinRate:      cfg.CoinRate,
		FallbackPrice: cfg.FallbackPrice,
		InitialStock:  cfg.InitialStock,
		ShelfLife:     shelfLife(cfg.CounterShelfDays),
		Treasury:      cfg.CounterBudget.Enabled,
	})
	if err != nil {
		return nil, fmt.Errorf("sim: counter: %w", err)
	}
	if err := fundTreasury(journal, cfg.CounterBudget.Initial, "initial treasury"); err != nil {
		return nil, err
	}

	valuer := domain.NewValuer(prices, cfg.FallbackPrice)
	crowd, err := newCrowd(cfg, catalogues, counter, valuer, o.traitRegistry)
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
	// 2. cost tier per catalogue, against the median of the reference catalogues, by default the
	// openable ones
	boosterCosts, err := domain.BoosterEnergyCosts(catalogues, o.reference, prices, cfg.FallbackPrice, cfg.Energy.CostTiers())
	if err != nil {
		return nil, fmt.Errorf("sim: %w", err)
	}
	fees, err := counterCoinFees(cfg, catalogues, avgValues)
	if err != nil {
		return nil, err
	}
	boosterFees, err := coinFees(cfg.Energy.CoinsPerEnergy, energyCap(cfg.Energy.CostTable), cfg.CoinRate, cfg.CounterSellPct, catalogues, avgValues)
	if err != nil {
		return nil, err
	}
	supply, err := newCounterSupply(cfg, catalogues, counter, boosterCosts, journal, fees)
	if err != nil {
		return nil, err
	}

	// Determine top-up settings early so the booster-wait check can use the real energy floor.
	topupOn := cfg.Energy.CoinTopup.Enabled && cfg.Energy.CoinsPerEnergy > 0
	topupMinEnergy := 1
	if cfg.Energy.CoinTopup.MinEnergyUnits > 0 {
		topupMinEnergy = cfg.Energy.CoinTopup.MinEnergyUnits
	}

	// When coin top-up is enabled the player only needs min_energy_units real energy (coins cover
	// the shortfall), so the booster-wait check must use that lower floor, not the full cost.
	waitCosts := boosterCosts
	if topupOn {
		waitCosts = make(map[string]int, len(boosterCosts))
		for k, cost := range boosterCosts {
			effective := cost
			if effective > topupMinEnergy {
				effective = topupMinEnergy
			}
			waitCosts[k] = effective
		}
	}
	maxWait, err := maxBoosterWaitMinutes(energyParams, waitCosts)
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
		cfg:                cfg,
		catalogues:         catalogues,
		binderCats:         binderCatalogues(catalogues, supplementary),
		supplementary:      supplementary,
		prices:             prices,
		journal:            journal,
		energy:             energy,
		collection:         collection,
		counter:            counter,
		crowd:              crowd,
		supply:             supply,
		counterOn:          supply != nil || cfg.CounterBudget.Enabled,
		exchange:           exchange,
		opener:             opener,
		policy:             policy,
		boosterCosts:       boosterCosts,
		boosterFees:        boosterFees,
		feeOn:              cfg.Energy.CoinsPerEnergy > 0,
		maxWait:            maxWait,
		coinTopupOn:        topupOn,
		coinTopupVariant:   cfg.Energy.CoinTopup.Variant,
		coinTopupMinEnergy: topupMinEnergy,
		coinTopupStep:      cfg.Energy.CoinTopup.EscalationStep,
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
		e.daySupplyFees = 0
		e.dayBrokeRefusals = 0
		e.dayBoosterFees = 0
		e.dayCoinTopupUnits = 0
		e.dayCoinTopupCoins = 0
		e.coinTopupDayUsed = 0

		if err := fundTreasury(e.journal, e.cfg.CounterBudget.DailyFloat, fmt.Sprintf("daily float day %d", day)); err != nil {
			return Result{}, fmt.Errorf("sim: day %d: %w", day, err)
		}

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
	if e.counterOn {
		addCounterSummary(summary, days)
	}
	if e.feeOn {
		addBoosterFeeSummary(summary, days)
	}
	if e.coinTopupOn {
		addCoinTopupSummary(summary, days)
	}

	return Result{Days: days, Summary: summary}, nil
}

// fundTreasury moves coins from the Counter's float into its treasury. It does nothing for a
// non-positive amount, which is also what a run without a budget has.
func fundTreasury(journal *domain.Journal, amount domain.Coins, memo string) error {
	if amount <= 0 {
		return nil
	}
	if err := journal.Post(domain.Entry{From: domain.AccountCounterFloat, To: domain.AccountCounterTreasury, Amount: amount, Memo: memo}); err != nil {
		return fmt.Errorf("sim: fund the Counter's treasury (%s): %w", memo, err)
	}
	return nil
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
	e.daySupplyFees += tick.FeeCoins
	return nil
}

// counterMetrics returns what the Counter did on its own on the day just ended, or nil when it
// neither opens boosters nor has a treasury.
func (e *Engine) counterMetrics() *CounterDayMetrics {
	if !e.counterOn {
		return nil
	}
	return &CounterDayMetrics{
		BoostersOpened: e.daySupplyBoosters,
		CardsOpened:    e.daySupplyCards,
		FeeCoins:       e.daySupplyFees,
		Treasury:       e.journal.Balance(domain.AccountCounterTreasury),
		BrokeRefusals:  e.dayBrokeRefusals,
	}
}

// boosterFeeMetrics returns the fees the player paid on the day just ended, or nil in a run with no
// fee price.
func (e *Engine) boosterFeeMetrics() *BoosterFeeDayMetrics {
	if !e.feeOn {
		return nil
	}
	return &BoosterFeeDayMetrics{Coins: e.dayBoosterFees}
}

// coinTopupMetrics returns the coin top-up metrics for the day just ended, or nil when
// top-up is not enabled.
func (e *Engine) coinTopupMetrics() *CoinTopupDayMetrics {
	if !e.coinTopupOn {
		return nil
	}
	return &CoinTopupDayMetrics{Units: e.dayCoinTopupUnits, Coins: e.dayCoinTopupCoins}
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
	if e.cfg.CounterBudget.Enabled {
		accounts = append(accounts, domain.AccountCounterTreasury, domain.AccountCounterFloat, domain.AccountCounterFee)
	}
	if e.feeOn {
		accounts = append(accounts, domain.AccountBoosterFee)
	}
	if e.coinTopupOn {
		accounts = append(accounts, domain.AccountCoinTopup)
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
	for _, cat := range e.binderCats {
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
		BoosterFee:      e.boosterFeeMetrics(),
		CoinTopup:       e.coinTopupMetrics(),
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

// coinTopupRateNow returns the coins-per-energy-unit price for the next top-up unit.
// For the escalating variant the rate rises with each unit bought within the day.
// Returns 0 when top-up is not enabled.
func (e *Engine) coinTopupRateNow() domain.Coins {
	if !e.coinTopupOn {
		return 0
	}
	p := e.cfg.Energy.CoinsPerEnergy
	if e.coinTopupVariant == config.CoinTopupVariantEscalating || e.coinTopupVariant == config.CoinTopupVariantBoth {
		return p + domain.Coins(e.coinTopupDayUsed)*e.coinTopupStep
	}
	return p
}

// computeTopUpCost returns the total coins needed to buy units energy units via coin top-up,
// given the current day state (coinTopupDayUsed). For the escalating variant the k-th unit
// (0-indexed from coinTopupDayUsed) costs p + (coinTopupDayUsed+k)*step.
func (e *Engine) computeTopUpCost(units int) domain.Coins {
	if units <= 0 || !e.coinTopupOn {
		return 0
	}
	p := e.cfg.Energy.CoinsPerEnergy
	switch e.coinTopupVariant {
	case config.CoinTopupVariantEscalating, config.CoinTopupVariantBoth:
		var total domain.Coins
		for k := range units {
			total += p + domain.Coins(e.coinTopupDayUsed+k)*e.coinTopupStep
		}
		return total
	default: // min_energy: constant rate p
		return domain.Coins(units) * p
	}
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
