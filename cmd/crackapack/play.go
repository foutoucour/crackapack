package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/images"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/web"
	producttune "github.com/foutoucour/crackapack/tune"
)

const (
	defaultPlayAddr = "127.0.0.1:8080"
	// demoSeed makes the demo open the same boosters on every run.
	demoSeed = 42
	// readHeaderTimeout bounds how long a client may take to send its headers.
	readHeaderTimeout = 10 * time.Second
	// shutdownTimeout bounds how long a stop waits for requests in flight.
	shutdownTimeout = 5 * time.Second

	scryfallAPI = "https://api.scryfall.com"
	// scryfallIconHost serves the SVG icon of each set.
	scryfallIconHost = "https://svgs.scryfall.io"
	// scryfallUserAgent identifies the app, as the Scryfall API requires.
	scryfallUserAgent = "crackapack/0 (unofficial fan project; https://github.com/foutoucour/crackapack)"
	// cardBackURL is the picture of the back of the cards, downloaded once into the image cache, see
	// docs/adr/0036-the-card-back-is-downloaded-with-an-in-house-fallback.md.
	cardBackURL = "https://tolkiengateway.net/w/images/1/18/Magic_the_Gathering_Card_Back.jpg"
	// imageFetchTimeout bounds one picture download.
	imageFetchTimeout = 30 * time.Second
	// imageCacheSubdir is where the pictures live under the OS user cache directory.
	imageCacheSubdir = "crackapack/images"
	// iconCacheSubdir is where the set icons live, under the image cache directory.
	iconCacheSubdir = "set-icons"
	// saveSubdir is where the save lives under the OS user config directory.
	saveSubdir = "crackapack"
	// saveFile and demoSaveFile are the names of the save. The demo has one of its own, so that
	// playing it never touches a real game.
	saveFile     = "save.json"
	demoSaveFile = "save-demo.json"
	// unlockSeedSalt, supplySeedSalt and crowdSeedSalt set the seeds of the unlocks, of the
	// Counter's boosters and of the NPCs apart from the seed of the player's boosters, so that no two
	// draw the same numbers.
	unlockSeedSalt = 0x9e3779b97f4a7c15
	supplySeedSalt = 0xbf58476d1ce4e5b9
	crowdSeedSalt  = 0x94d049bb133111eb
)

// playArgs holds all dependencies for runPlay, decoupled from the OS so that tests can serve on a
// free port and read the sets from memory.
type playArgs struct {
	// catFS is the catalogue directory; set files are loaded as "<CODE>.json".
	catFS     fs.FS
	setCodes  []string // openable set codes (must have a booster section)
	suppCodes []string // card pools the boosters draw from, such as TSB for TSP
	// scope plays every set in scope of catFS, with the pools they draw from, in stages that open up
	// as cfg.Unlock says, instead of setCodes and suppCodes, see [loadScope].
	scope bool

	// seed seeds the boosters of a new game, so that it is reproducible from it. A game loaded from
	// a save keeps the seed it was saved with.
	seed uint64
	// store keeps the game between runs, and savePath is where, for the line that tells the player.
	store    play.Store
	savePath string

	// pricesFS and pricesName locate the price snapshot that sets what a booster costs in energy:
	// a booster worth more than the median of the catalogue costs more. Without a name, no set is
	// worth more than another and every booster costs the middle tier.
	pricesFS   fs.FS
	pricesName string
	// cfg gives the energy the player recharges, the tiers of what a booster costs and how the stages
	// open up.
	cfg config.Config
	// now tells the time that the energy recharges by.
	now play.Clock

	// images gives the picture of a card by its Scryfall ID.
	images images.Source
	// cardBack gives the picture of the back of a card, under images.CardBackID.
	cardBack images.Source
	// icons gives the icon of a set by its icon code.
	icons images.Source

	// openBrowser shows the game at a URL in the player's browser once the server listens. Nil
	// opens nothing, as in the tests; a failure is logged and does not stop the game.
	openBrowser func(url string) error

	listener net.Listener
	stdout   io.Writer
	stderr   io.Writer
}

// newPlayCmd returns the cobra command for the play subcommand.
// stdout receives the address to open; stderr receives errors, usage, help, and the server log.
func newPlayCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		demoData   bool
		noBrowser  bool
		catDir     string
		setsRaw    string
		suppRaw    string
		pricesPath string
		addr       string
		seed       uint64
		savePath   string
	)

	cmd := &cobra.Command{
		Use:   "play",
		Short: "serve the web client on localhost to open boosters",
		Long: `Serve the web client on localhost, and open it in your browser unless --no-browser says
otherwise. You start with ten boosters to open.

Pass --demo to play on the built-in synthetic sets, with a fixed seed, so no files or network are
needed. Otherwise pass --catalogue-dir; --seed 0 picks a random seed and prints it.

Without --sets, or with --sets all, the game has every set of the catalogue directory from Mirage
on whose booster is worth under $60, and the card pools their boosters draw from. The sets open up
in stages, from the quickest binder to fill to the slowest: the first stage is open, and a binder
filled to unlock.binder_threshold of its cards unlocks unlock.sets_per_unlock sets of the next stage.
--sets RAV,TSP plays those sets only, all open, with the pools of --supplementary. The demo plays in
stages too.

The game is saved after every action and picked up where it was when you start again, so the
energy keeps recharging while it is closed. The save is a JSON file in the user config directory,
or at --save; the demo has a save of its own. A game that is loaded keeps the seed it was saved
with, and --seed only seeds a new one.

Opening a booster costs energy, once the ten welcome boosters, which are free, are gone. What a
booster costs follows its value against the median of the catalogue, so pass --prices, the price
snapshot of fetchdata, to tell the sets apart; without it every booster costs the same.

You start with 200 coins. The Counter buys any card at 60% of its price and sells at 120%, and it
stocks itself each day, also while the game is closed, up to the last catch_up_days. The prices come from
--prices too: without it every card is worth the fallback price of one cent.

The game's NPCs open the sets you can open and trade with you on the market: ask around for a card and
the NPCs that have it ask a price, or list a card and the NPCs that want it make offers in coins or
cards. Each answers at the pace of its personality, some at once and some the next day, also while
the game is closed. Ask around from an empty slot of a binder, list a card from a held one, and
answer on the Market page, which the header links to with the answers waiting.

Card pictures are downloaded from Scryfall the first time a card shows, one at a time, and kept
in the user cache directory. A card whose picture cannot be downloaded shows as text. The back of
the cards is downloaded once too, also with --demo, and an in-house drawing stands in for it
while it is missing.`,
		Example: `  crackapack play --demo
  crackapack play --catalogue-dir ./catalogues --prices prices.json
  crackapack play --catalogue-dir ./catalogues --sets RAV,TSP --supplementary TSB
  crackapack play --demo --save ./my-save.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ds demo.Dataset
			if demoData {
				ds = demo.Build()
			}
			cfg, err := playConfig(demoData, ds)
			if err != nil {
				return fmt.Errorf("crackapack play: configuration: %w", err)
			}
			// The one place the real clock is read: the rest of the game is given it.
			a := playArgs{seed: seed, stdout: stdout, stderr: stderr, cfg: cfg, now: time.Now} //nolint:forbidigo // composition root
			if !noBrowser {
				a.openBrowser = systemBrowser
			}
			cacheRoot, err := os.UserCacheDir()
			if err != nil {
				return fmt.Errorf("crackapack play: image cache directory: %w", err)
			}
			imageDir := filepath.Join(cacheRoot, imageCacheSubdir)
			a.cardBack = images.NewCached(newCardBackSource(), imageDir)
			// The demo sets are made up: no site draws their icons, so the demo shows the stand-in.
			a.icons = images.Chain()
			if savePath == "" {
				configRoot, err := os.UserConfigDir()
				if err != nil {
					return fmt.Errorf("crackapack play: save directory, pass --save: %w", err)
				}
				savePath = defaultSavePath(configRoot, demoData)
			}
			a.savePath, a.store = savePath, play.NewFileStore(savePath)
			if demoData {
				a.catFS, a.scope, a.images = ds.Files, true, ds.Images
				a.pricesFS, a.pricesName = ds.Files, "prices.json"
				if a.seed == 0 {
					a.seed = demoSeed
				}
			} else {
				if catDir == "" {
					if err := cmd.Usage(); err != nil {
						return err
					}
					return errors.New("crackapack play: required flag not set: --catalogue-dir (or pass --demo)")
				}
				a.catFS = os.DirFS(catDir)
				if setsRaw == allSets {
					a.scope = true
				} else {
					a.setCodes, a.suppCodes = splitCodes(setsRaw), splitCodes(suppRaw)
				}
				if pricesPath != "" {
					a.pricesFS, a.pricesName = os.DirFS(filepath.Dir(pricesPath)), filepath.Base(pricesPath)
				}
				a.images = images.NewCached(newScryfallSource(), imageDir)
				a.icons = images.NewCachedIcons(newScryfallIconSource(), filepath.Join(imageDir, iconCacheSubdir))
				if a.seed == 0 {
					a.seed = rand.Uint64()
				}
			}

			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("crackapack play: listen on %s: %w", addr, err)
			}
			a.listener = ln

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			if err := runPlay(ctx, a); err != nil {
				return fmt.Errorf("crackapack play: %w", err)
			}
			return nil
		},
	}
	// cobra help and usage go to stderr to match existing test assertions.
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	f := cmd.Flags()
	f.BoolVar(&demoData, "demo", false, "play on the built-in synthetic sets, with a fixed seed")
	f.StringVar(&catDir, "catalogue-dir", "", "catalogue directory of <CODE>.json files (required without --demo)")
	f.StringVar(&setsRaw, "sets", allSets, "comma-separated openable set codes, e.g. RAV,TSP, or all for every set in scope, in stages")
	f.StringVar(&suppRaw, "supplementary", "", "comma-separated card-pool-only codes, e.g. TSB (optional, ignored with --sets all)")
	f.StringVar(&pricesPath, "prices", "", "JSON price snapshot file; sets what a booster costs in energy (optional, ignored with --demo)")
	f.BoolVar(&noBrowser, "no-browser", false, "do not open the game in your browser when the server starts")
	f.StringVar(&addr, "addr", defaultPlayAddr, "address to listen on; keep it on localhost")
	f.Uint64Var(&seed, "seed", 0, "seed of the boosters of a new game; 0 is the demo seed with --demo and a random one otherwise")
	f.StringVar(&savePath, "save", "", "file the game is saved in (default: crackapack/save.json in the user config directory, save-demo.json with --demo)")

	return cmd
}

// runPlay is the testable core of the play subcommand. It loads the sets, starts a game and serves
// the web client on a.listener until ctx is cancelled.
func runPlay(ctx context.Context, a playArgs) error {
	log := slog.New(slog.NewTextHandler(a.stderr, nil))
	game, stream, err := startGame(a, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           web.NewHandler(game, images.Chain(a.cardBack, a.images), a.icons, log),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(a.listener) }()
	url := "http://" + a.listener.Addr().String()
	fmt.Fprintf(a.stdout, "crackapack: open %s in your browser (seed %d), Ctrl-C to stop\n", url, stream.Seed())
	if a.openBrowser != nil {
		if err := a.openBrowser(url); err != nil {
			log.Warn("open the browser", "url", url, "error", err)
		}
	}

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	// ctx is done, so the shutdown deadline cannot derive from it.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// playConfig is the configuration of the game: the product's base config, the world its goals are
// judged in, with the play overlay, which makes the Counter scarce, see
// docs/adr/0057-the-game-plays-the-products-base-config.md. The demo has a configuration of its
// own, ds's, which cuts its few sets into stages small enough to unlock one soon.
func playConfig(demoData bool, ds demo.Dataset) (config.Config, error) {
	if demoData {
		return config.Load(strings.NewReader(ds.PlayConfigYAML))
	}
	base, err := config.Load(strings.NewReader(producttune.BaseYAML))
	if err != nil {
		return config.Config{}, fmt.Errorf("tune/base.yaml: %w", err)
	}
	overlay, err := config.LoadOverrides(strings.NewReader(producttune.PlayYAML))
	if err != nil {
		return config.Config{}, fmt.Errorf("tune/play.yaml: %w", err)
	}
	cfg, err := config.ApplyOverrides(base, overlay)
	if err != nil {
		return config.Config{}, fmt.Errorf("tune/play.yaml: %w", err)
	}
	return cfg, nil
}

// startGame loads the sets and starts the game of a: its stages, its Counter, and what its save
// holds. It returns the stream of the player's boosters, whose seed a loaded game takes from the
// save.
func startGame(a playArgs, log *slog.Logger) (*play.Game, *play.Stream, error) {
	if a.store == nil {
		return nil, nil, errors.New("no store to save the game in")
	}
	prices := domain.PriceSnapshot{}
	if a.pricesName != "" {
		var err error
		if prices, err = domain.LoadPriceSnapshot(a.pricesFS, a.pricesName); err != nil {
			return nil, nil, fmt.Errorf("prices %q: %w", a.pricesName, err)
		}
	}
	sets, pools, reference, err := loadPlaySets(a, prices)
	if err != nil {
		return nil, nil, err
	}
	stream := play.NewStream(a.seed)
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{
			domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
		},
		rand.New(stream),
	)
	costs, err := domain.BoosterEnergyCosts(sets, reference, prices, a.cfg.FallbackPrice, a.cfg.Energy.CostTiers())
	if err != nil {
		return nil, nil, err
	}
	rules, err := playRules(a.cfg, sets, costs, prices)
	if err != nil {
		return nil, nil, err
	}
	game, err := play.NewGame(sets, pools, opener, rules, a.now)
	if err != nil {
		return nil, nil, err
	}
	if a.scope {
		u := a.cfg.Unlock
		stages := play.StageRules{Threshold: u.BinderThreshold, SetsPerUnlock: u.SetsPerUnlock, Stages: u.Stages, StartSets: u.StartSets}
		if err := game.Stage(stages, play.NewStream(a.seed^unlockSeedSalt)); err != nil {
			return nil, nil, err
		}
	}
	counter, err := counterRules(a.cfg, prices)
	if err != nil {
		return nil, nil, err
	}
	if err := game.OpenCounter(counter, play.NewStream(a.seed^supplySeedSalt), play.NewStream(a.seed^crowdSeedSalt)); err != nil {
		return nil, nil, err
	}
	// Said first, so that an error about the save has said which file it is.
	fmt.Fprintf(a.stdout, "crackapack: save file %s\n", a.savePath)
	saves := play.Saves{Store: a.store, Stream: stream, Failed: func(err error) { log.Error("save game", "path", a.savePath, "error", err) }}
	if err := game.Persist(saves); err != nil {
		return nil, nil, fmt.Errorf("save file %q: %w", a.savePath, err)
	}
	return game, stream, nil
}

// playRules is what opening a booster costs in the game of cfg, as the simulation prices it: the
// energy of each set, the coin fee of the sets worth more than the energy cap, and the coin top-up.
// The game tops up at one price for every point, the `min_energy` variant: a price that rises with
// the day's use is not played.
func playRules(cfg config.Config, sets []domain.SetCatalogue, costs map[string]int, prices domain.PriceSnapshot) (play.Rules, error) {
	rules := play.Rules{Energy: cfg.Energy.Params(), Costs: costs}
	values := make([]domain.USDCents, len(sets))
	for i, cat := range sets {
		var err error
		if values[i], err = domain.BoosterAverageValue(cat, prices, cfg.FallbackPrice); err != nil {
			return play.Rules{}, fmt.Errorf("booster value of %q: %w", cat.Code, err)
		}
	}
	var err error
	rules.Fees, err = domain.BoosterCoinFees(cfg.Energy.CoinsPerEnergy, domain.EnergyCap(cfg.Energy.CostTiers()), cfg.CoinRate, cfg.CounterSellPct, sets, values)
	if err != nil {
		return play.Rules{}, err
	}
	if topUp := cfg.Energy.CoinTopup; topUp.Enabled {
		if topUp.Variant != config.CoinTopupVariantMinEnergy {
			return play.Rules{}, fmt.Errorf("energy.coin_topup.variant: %q is not played yet, the game tops up at one price: use %q", topUp.Variant, config.CoinTopupVariantMinEnergy)
		}
		rules.TopUp = &play.TopUpRules{Price: cfg.Energy.CoinsPerEnergy, MinEnergy: topUp.MinEnergyUnits}
	}
	return rules, nil
}

// counterRules is the Counter of cfg, valuing the cards by prices, as the simulation builds it: the
// rates, the stock and the shelf, the treasury, the supply and the NPCs. The game's NPCs always trade
// on its market, whatever trades.enabled, the simulation's switch, says. The Counter's coin fee on
// dear boosters is not played.
func counterRules(cfg config.Config, prices domain.PriceSnapshot) (play.CounterRules, error) {
	if cfg.CounterSupply.CoinsPerEnergy > 0 {
		return play.CounterRules{}, errors.New("counter_supply.coins_per_energy: the Counter's coin fee on dear boosters is not played yet")
	}
	rules := play.CounterRules{
		Params:       cfg.CounterParams(),
		Prices:       prices,
		WelcomeBonus: cfg.WelcomeBonus,
		DailyReward:  cfg.DailyReward,
		CatchUpDays:  cfg.CatchUpDays,
	}
	if cfg.CounterBudget.Enabled {
		rules.Treasury, rules.DailyFloat = cfg.CounterBudget.Initial, cfg.CounterBudget.DailyFloat
	}
	switch cfg.CounterSupply.Mode {
	case config.CounterSupplyFixed:
		rules.Supply.BoostersPerDay = cfg.CounterSupply.BoostersPerDay
	case config.CounterSupplyEnergy:
		energy := cfg.CounterSupply.Energy.Params()
		rules.Supply.Energy = &energy
	}
	if cfg.NPCs.Population > 0 {
		if cfg.NPCs.SurplusHoldDays < 1 {
			return play.CounterRules{}, fmt.Errorf("npcs.surplus_hold_days is %d: the market needs NPCs that hold their surplus a day at least", cfg.NPCs.SurplusHoldDays)
		}
		rules.Crowd = domain.PopulationParams{
			Size:         cfg.NPCs.Population,
			Archetypes:   cfg.NPCs.ArchetypeSpecs(true),
			Valuer:       domain.NewValuer(prices, cfg.FallbackPrice),
			MaxMarginPct: cfg.NPCs.MaxMarginPct,
		}
		rules.Market = &play.MarketRules{
			SurplusHoldDays: cfg.NPCs.SurplusHoldDays,
			Params: domain.MarketParams{
				Expiry:          time.Duration(cfg.Trades.OfferExpiryDays) * 24 * time.Hour,
				MaxCardsPerSide: cfg.Trades.MaxCardsPerSide,
				CoinRate:        cfg.CoinRate,
			},
		}
	}
	return rules, nil
}

// defaultSavePath is where the save of a game goes under configDir, the OS user config directory.
func defaultSavePath(configDir string, demo bool) string {
	name := saveFile
	if demo {
		name = demoSaveFile
	}
	return filepath.Join(configDir, saveSubdir, name)
}

// loadPlaySets loads the sets of the session and the card pools their boosters draw from: every
// set in scope with a.scope, see [loadScope], and otherwise the sets and pools a names. reference
// is the common bundle of the catalogue directory, which the energy cost of a booster follows: its
// value against their median, as in the simulation, so it does not depend on which sets are played.
// A requested set of the expensive bundle fails the session, see [leaveOutExpensiveSets].
func loadPlaySets(a playArgs, prices domain.PriceSnapshot) (sets, pools, reference []domain.SetCatalogue, err error) {
	if a.scope {
		return loadScope(a.catFS, prices, a.cfg)
	}
	if sets, pools, err = loadPlayCatalogues(a.catFS, a.setCodes, a.suppCodes); err != nil {
		return nil, nil, nil, err
	}
	if reference, err = loadReferenceCatalogues(a.catFS, a.setCodes, sets); err != nil {
		return nil, nil, nil, err
	}
	if reference, err = leaveOutExpensiveSets(sets, reference, prices, a.cfg); err != nil {
		return nil, nil, nil, err
	}
	return sets, pools, reference, nil
}

// loadPlayCatalogues loads the openable sets, which need a booster definition, and the card pools
// their boosters draw from.
func loadPlayCatalogues(catFS fs.FS, setCodes, suppCodes []string) (sets, pools []domain.SetCatalogue, err error) {
	for _, code := range setCodes {
		name := code + ".json"
		cat, err := domain.LoadSetCatalogue(catFS, name)
		if err != nil {
			return nil, nil, fileError("catalogue", name, err)
		}
		sets = append(sets, cat)
	}
	for _, code := range suppCodes {
		name := code + ".json"
		pool, err := domain.LoadCardPool(catFS, name)
		if err != nil {
			return nil, nil, fileError("supplementary", name, err)
		}
		pools = append(pools, pool)
	}
	return sets, pools, nil
}

// newCardBackSource builds the source of the picture of the back of the cards.
func newCardBackSource() *images.Fixed {
	return images.NewFixed(images.FixedConfig{
		Client:    &http.Client{Timeout: imageFetchTimeout},
		ID:        images.CardBackID,
		URL:       cardBackURL,
		UserAgent: scryfallUserAgent,
	})
}

// newScryfallIconSource builds the source of the set icons.
func newScryfallIconSource() *images.ScryfallIcons {
	return images.NewScryfallIcons(images.ScryfallConfig{
		Client:    &http.Client{Timeout: imageFetchTimeout},
		BaseURL:   scryfallIconHost,
		UserAgent: scryfallUserAgent,
		Interval:  images.DefaultInterval,
		Sleep:     time.Sleep,
	})
}

// newScryfallSource builds the source of the real pictures.
func newScryfallSource() *images.Scryfall {
	return images.NewScryfall(images.ScryfallConfig{
		Client:    &http.Client{Timeout: imageFetchTimeout},
		BaseURL:   scryfallAPI,
		UserAgent: scryfallUserAgent,
		Interval:  images.DefaultInterval,
		Sleep:     time.Sleep,
	})
}
