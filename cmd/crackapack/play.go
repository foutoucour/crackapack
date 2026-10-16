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
	// scryfallUserAgent identifies the app, as the Scryfall API requires.
	scryfallUserAgent = "crackapack/0 (unofficial fan project; https://github.com/foutoucour/crackapack)"
	// cardBackURL is the picture of the back of the cards, downloaded once into the image cache, see
	// docs/adr/0036-the-card-back-is-downloaded-with-an-in-house-fallback.md.
	cardBackURL = "https://tolkiengateway.net/w/images/1/18/Magic_the_Gathering_Card_Back.jpg"
	// imageFetchTimeout bounds one picture download.
	imageFetchTimeout = 30 * time.Second
	// imageCacheSubdir is where the pictures live under the OS user cache directory.
	imageCacheSubdir = "crackapack/images"
)

// playArgs holds all dependencies for runPlay, decoupled from the OS so that tests can serve on a
// free port and read the sets from memory.
type playArgs struct {
	// catFS is the catalogue directory; set files are loaded as "<CODE>.json".
	catFS     fs.FS
	setCodes  []string // openable set codes (must have a booster section)
	suppCodes []string // card pools the boosters draw from, such as TSB for TSP

	// seed seeds the boosters, so a run is reproducible from it.
	seed uint64

	// pricesFS and pricesName locate the price snapshot that sets what a booster costs in energy:
	// a booster worth more than the median of the catalogue costs more. Without a name, no set is
	// worth more than another and every booster costs the middle tier.
	pricesFS   fs.FS
	pricesName string
	// cfg gives the energy the player recharges and the tiers of what a booster costs.
	cfg config.Config
	// now tells the time that the energy recharges by.
	now play.Clock

	// images gives the picture of a card by its Scryfall ID.
	images images.Source
	// cardBack gives the picture of the back of a card, under images.CardBackID.
	cardBack images.Source

	listener net.Listener
	stdout   io.Writer
	stderr   io.Writer
}

// newPlayCmd returns the cobra command for the play subcommand.
// stdout receives the address to open; stderr receives errors, usage, help, and the server log.
func newPlayCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		demoData   bool
		catDir     string
		setsRaw    string
		suppRaw    string
		pricesPath string
		addr       string
		seed       uint64
	)

	cmd := &cobra.Command{
		Use:   "play",
		Short: "serve the web client on localhost to open boosters",
		Long: `Serve the web client on localhost. You start with ten boosters to open.

Pass --demo to play on the built-in synthetic sets, with a fixed seed, so no files or network are
needed. Otherwise pass --catalogue-dir and --sets, like simulate; --seed 0 picks a random seed and
prints it.

Opening a booster costs energy, once the ten welcome boosters, which are free, are gone. What a
booster costs follows its value against the median of the catalogue, so pass --prices, the price
snapshot of fetchdata, to tell the sets apart; without it every booster costs the same.

Card pictures are downloaded from Scryfall the first time a card shows, one at a time, and kept
in the user cache directory. A card whose picture cannot be downloaded shows as text. The back of
the cards is downloaded once too, also with --demo, and an in-house drawing stands in for it
while it is missing.`,
		Example: `  crackapack play --demo
  crackapack play --catalogue-dir ./catalogues --sets RAV,TSP --supplementary TSB`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(strings.NewReader(""))
			if err != nil {
				return fmt.Errorf("crackapack play: default configuration: %w", err)
			}
			// The one place the real clock is read: the rest of the game is given it.
			a := playArgs{seed: seed, stdout: stdout, stderr: stderr, cfg: cfg, now: time.Now} //nolint:forbidigo // composition root
			cacheRoot, err := os.UserCacheDir()
			if err != nil {
				return fmt.Errorf("crackapack play: image cache directory: %w", err)
			}
			imageDir := filepath.Join(cacheRoot, imageCacheSubdir)
			a.cardBack = images.NewCached(newCardBackSource(), imageDir)
			if demoData {
				ds := demo.Build()
				a.catFS, a.setCodes, a.suppCodes, a.images = ds.Files, ds.SetCodes, ds.SupplementaryCodes, ds.Images
				a.pricesFS, a.pricesName = ds.Files, "prices.json"
				if a.seed == 0 {
					a.seed = demoSeed
				}
			} else {
				var missing []string
				if catDir == "" {
					missing = append(missing, "--catalogue-dir")
				}
				if setsRaw == "" {
					missing = append(missing, "--sets")
				}
				if len(missing) > 0 {
					if err := cmd.Usage(); err != nil {
						return err
					}
					return fmt.Errorf("crackapack play: required flags not set: %s (or pass --demo)",
						strings.Join(missing, ", "))
				}
				a.catFS, a.setCodes, a.suppCodes = os.DirFS(catDir), splitCodes(setsRaw), splitCodes(suppRaw)
				if pricesPath != "" {
					a.pricesFS, a.pricesName = os.DirFS(filepath.Dir(pricesPath)), filepath.Base(pricesPath)
				}
				a.images = images.NewCached(newScryfallSource(), imageDir)
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
	f.StringVar(&setsRaw, "sets", "", "comma-separated openable set codes, e.g. RAV,TSP (required without --demo)")
	f.StringVar(&suppRaw, "supplementary", "", "comma-separated card-pool-only codes, e.g. TSB (optional)")
	f.StringVar(&pricesPath, "prices", "", "JSON price snapshot file; sets what a booster costs in energy (optional, ignored with --demo)")
	f.StringVar(&addr, "addr", defaultPlayAddr, "address to listen on; keep it on localhost")
	f.Uint64Var(&seed, "seed", 0, "seed of the boosters; 0 is the demo seed with --demo and a random one otherwise")

	return cmd
}

// runPlay is the testable core of the play subcommand. It loads the sets, starts a game and serves
// the web client on a.listener until ctx is cancelled.
func runPlay(ctx context.Context, a playArgs) error {
	sets, pools, err := loadPlayCatalogues(a.catFS, a.setCodes, a.suppCodes)
	if err != nil {
		return err
	}
	opener := domain.NewOpener(
		map[domain.BoosterLayout]domain.BoosterGenerator{
			domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
		},
		rand.New(rand.NewPCG(a.seed, a.seed)),
	)
	rules, err := playRules(a, sets)
	if err != nil {
		return err
	}
	game, err := play.NewGame(sets, pools, opener, rules, a.now)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           web.NewHandler(game, images.Chain(a.cardBack, a.images), slog.New(slog.NewTextHandler(a.stderr, nil))),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(a.listener) }()
	fmt.Fprintf(a.stdout, "crackapack: open http://%s in your browser (seed %d), Ctrl-C to stop\n", a.listener.Addr(), a.seed)

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

// playRules builds the rules of the session: the recharge of the energy, and what a booster of each
// set costs. The cost follows the booster's value against the median of every set in the catalogue
// directory, as in the simulation, so it does not depend on which sets are played.
func playRules(a playArgs, sets []domain.SetCatalogue) (play.Rules, error) {
	reference, err := loadReferenceCatalogues(a.catFS, a.setCodes, sets)
	if err != nil {
		return play.Rules{}, err
	}
	prices := domain.PriceSnapshot{}
	if a.pricesName != "" {
		if prices, err = domain.LoadPriceSnapshot(a.pricesFS, a.pricesName); err != nil {
			return play.Rules{}, fmt.Errorf("prices %q: %w", a.pricesName, err)
		}
	}
	costs, err := domain.BoosterEnergyCosts(sets, reference, prices, a.cfg.FallbackPrice, a.cfg.Energy.CostTiers())
	if err != nil {
		return play.Rules{}, err
	}
	return play.Rules{Energy: a.cfg.Energy.Params(), Costs: costs}, nil
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
