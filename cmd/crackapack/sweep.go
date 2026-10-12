package main

import (
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// cmdSweep is a shim kept for backward-compatible test invocation.
func cmdSweep(args []string, stdout, stderr io.Writer) error {
	return run(append([]string{"sweep"}, args...), stdout, stderr)
}

// newSweepCmd returns the cobra command for the sweep subcommand.
// stdout receives the sweep table; stderr receives errors, usage, and help.
func newSweepCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		configPath string
		catDir     string
		setsRaw    string
		suppRaw    string
		pricesPath string
		player     string
		outDir     string
		ratesRaw   string
	)

	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "sweep the coin-per-USD rate over a grid and report calibration results",
		Long: `Run the same simulation for each coin-per-USD rate in the sweep grid,
evaluate calibration targets against each run, and report which rates pass.

The sweep grid is taken from sweep_rates in the config file. Pass --rates N,...
to override it on the command line.`,
		Example: `  crackapack sweep \
    --catalogue-dir ./catalogues \
    --sets RAV,CLS \
    --prices ./prices.json \
    --out ./sweep-out`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var missing []string
			if catDir == "" {
				missing = append(missing, "--catalogue-dir")
			}
			if setsRaw == "" {
				missing = append(missing, "--sets")
			}
			if pricesPath == "" {
				missing = append(missing, "--prices")
			}
			if outDir == "" {
				missing = append(missing, "--out")
			}
			if len(missing) > 0 {
				if err := cmd.Usage(); err != nil {
					return err
				}
				return fmt.Errorf("crackapack sweep: required flags not set: %s",
					strings.Join(missing, ", "))
			}

			var rates []domain.CoinRate
			if ratesRaw != "" {
				parsed, err := parseRates(ratesRaw)
				if err != nil {
					return fmt.Errorf("crackapack sweep: --rates: %w", err)
				}
				rates = parsed
			}

			var configReader io.Reader
			if configPath != "" {
				f, err := os.Open(configPath)
				if err != nil {
					return fmt.Errorf("crackapack sweep: config %q: %w", configPath, err)
				}
				defer func() { _ = f.Close() }()
				configReader = f
			}

			catFS := os.DirFS(catDir)
			pricesFS := os.DirFS(filepath.Dir(pricesPath))
			pricesName := filepath.Base(pricesPath)

			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("crackapack sweep: --out %q: %w", outDir, err)
			}
			newOutput := func(name string) (io.WriteCloser, error) {
				full := filepath.Join(outDir, name)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					return nil, err
				}
				return os.Create(full)
			}

			a := sweepArgs{
				configReader: configReader,
				catFS:        catFS,
				setCodes:     splitCodes(setsRaw),
				suppCodes:    splitCodes(suppRaw),
				pricesFS:     pricesFS,
				pricesName:   pricesName,
				player:       player,
				policies:     buildPolicies(),
				rates:        rates,
				newOutput:    newOutput,
				stdout:       stdout,
			}

			if err := runSweep(a); err != nil {
				return fmt.Errorf("crackapack sweep: %w", err)
			}
			return nil
		},
	}
	// cobra help and usage go to stderr to match existing test assertions.
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	f := cmd.Flags()
	f.StringVar(&configPath, "config", "", "YAML config file (optional; omit for defaults)")
	f.StringVar(&catDir, "catalogue-dir", "", "catalogue directory of <CODE>.json files; every openable set in it is the energy cost reference (required)")
	f.StringVar(&setsRaw, "sets", "", "comma-separated openable set codes, e.g. RAV,CLS (required)")
	f.StringVar(&suppRaw, "supplementary", "", "comma-separated card-pool-only codes, e.g. TSB (optional)")
	f.StringVar(&pricesPath, "prices", "", "JSON price snapshot file (required)")
	f.StringVar(&player, "player", "collector", "player policy name")
	f.StringVar(&outDir, "out", "", "output root directory (required)")
	f.StringVar(&ratesRaw, "rates", "", "comma-separated coin-per-USD rates (overrides sweep_rates in config)")

	return cmd
}

// runSweep is the testable core of the sweep subcommand. It loads inputs, runs
// the sweep, writes per-rate reports and the aggregate sweep.json, and prints
// the text table to stdout.
func runSweep(a sweepArgs) error {
	// Reuse the shared input-loading logic from simulate.
	sa := simulateArgs{
		configReader: a.configReader,
		catFS:        a.catFS,
		setCodes:     a.setCodes,
		suppCodes:    a.suppCodes,
		pricesFS:     a.pricesFS,
		pricesName:   a.pricesName,
		player:       a.player,
		policies:     a.policies,
	}
	in, err := loadInputs(sa)
	if err != nil {
		return err
	}

	// Determine the rate grid: --rates flag overrides cfg.SweepRates.
	grid := a.rates
	if grid == nil {
		grid = in.cfg.SweepRates
	}
	if len(grid) == 0 {
		return fmt.Errorf("sweep grid is empty: set sweep_rates in config or use --rates")
	}

	// Build the run closure: for each rate, build a fresh engine and rng.
	run := func(rate domain.CoinRate) (sim.Result, error) {
		rateCfg := in.cfg
		rateCfg.CoinRate = rate

		rng := rand.New(rand.NewPCG(in.cfg.Seed, in.cfg.Seed))

		engine, err := in.newEngine(rateCfg, rng)
		if err != nil {
			return sim.Result{}, fmt.Errorf("engine: %w", err)
		}
		result, err := engine.Run()
		if err != nil {
			return sim.Result{}, fmt.Errorf("run: %w", err)
		}
		return result, nil
	}

	// Run the sweep.
	sr, err := sim.Sweep(grid, run, in.cfg.Targets)
	if err != nil {
		return err
	}

	// Write per-rate reports.
	for _, rr := range sr.Rates {
		rateDir := fmt.Sprintf("rate-%d", rr.Rate)

		jsonW, err := a.newOutput(rateDir + "/result.json")
		if err != nil {
			return fmt.Errorf("rate-%d: result.json: %w", rr.Rate, err)
		}
		if writeErr := sim.WriteJSON(jsonW, rr.Result); writeErr != nil {
			_ = jsonW.Close()
			return fmt.Errorf("rate-%d: result.json: %w", rr.Rate, writeErr)
		}
		if closeErr := jsonW.Close(); closeErr != nil {
			return fmt.Errorf("rate-%d: result.json: close: %w", rr.Rate, closeErr)
		}

		csvW, err := a.newOutput(rateDir + "/days.csv")
		if err != nil {
			return fmt.Errorf("rate-%d: days.csv: %w", rr.Rate, err)
		}
		if writeErr := sim.WriteCSV(csvW, rr.Result); writeErr != nil {
			_ = csvW.Close()
			return fmt.Errorf("rate-%d: days.csv: %w", rr.Rate, writeErr)
		}
		if closeErr := csvW.Close(); closeErr != nil {
			return fmt.Errorf("rate-%d: days.csv: close: %w", rr.Rate, closeErr)
		}
	}

	// Write sweep.json.
	sweepJSONW, err := a.newOutput("sweep.json")
	if err != nil {
		return fmt.Errorf("sweep.json: %w", err)
	}
	if writeErr := sim.WriteSweepJSON(sweepJSONW, sr); writeErr != nil {
		_ = sweepJSONW.Close()
		return fmt.Errorf("sweep.json: %w", writeErr)
	}
	if closeErr := sweepJSONW.Close(); closeErr != nil {
		return fmt.Errorf("sweep.json: close: %w", closeErr)
	}

	// Print the text table to stdout.
	if err := sim.WriteSweepText(a.stdout, sr, in.cfg.Targets); err != nil {
		return fmt.Errorf("stdout: %w", err)
	}
	return nil
}

// parseRates splits a comma-separated list of integers into a []domain.CoinRate
// and applies the same validation rules as config.ValidateSweepRates (non-empty,
// every rate > 0, strictly ascending). Returns an error naming --rates and the
// offending value.
func parseRates(raw string) ([]domain.CoinRate, error) {
	parts := strings.Split(raw, ",")
	rates := make([]domain.CoinRate, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid token %q: must be a positive integer", p)
		}
		rates = append(rates, domain.CoinRate(n))
	}
	if err := config.ValidateSweepRates(rates); err != nil {
		return nil, err
	}
	return rates, nil
}
