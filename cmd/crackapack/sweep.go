package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// sweepArgs holds all dependencies for runSweep, decoupled from the OS so that
// tests can substitute in-memory implementations.
type sweepArgs struct {
	// configReader is an open reader for the YAML config file.
	// nil means "use all defaults".
	configReader io.Reader

	// catFS is the catalogue directory.
	catFS     fs.FS
	setCodes  []string
	suppCodes []string

	// pricesFS is the FS rooted at the prices file's directory.
	pricesFS   fs.FS
	pricesName string

	player   string
	policies map[string]func() sim.PlayerPolicy

	// rates overrides cfg.SweepRates when non-nil.
	rates []domain.CoinRate

	// newOutput creates (or truncates) an output file by relative path.
	// The path may contain a subdirectory, e.g. "rate-10/result.json".
	// The OS wrapper creates parent directories; the in-memory fake stores by path.
	newOutput func(string) (io.WriteCloser, error)

	stdout io.Writer
}

const sweepUsage = `usage: crackapack sweep [flags]

Run the same simulation for each coin-per-USD rate in the sweep grid,
evaluate calibration targets, and report which rates pass.

Flags:
  --config <file>
        YAML config file (optional; omit to use all defaults)
  --catalogue-dir <dir>
        directory containing <CODE>.json MTGJSON set files (required)
  --sets <CODE,...>
        comma-separated set codes to open, e.g. RAV,CLS (required)
  --supplementary <CODE,...>
        comma-separated card-pool-only codes, e.g. TSB (optional)
  --prices <file>
        JSON price snapshot file (required)
  --player <name>
        player policy name (default: collector)
  --out <dir>
        output root directory (required)
        writes <out>/rate-<N>/result.json, <out>/rate-<N>/days.csv, <out>/sweep.json
  --rates <N,...>
        comma-separated coin-per-USD rates (overrides sweep_rates in config)

Example:
  crackapack sweep \
    --catalogue-dir ./catalogues \
    --sets RAV,CLS \
    --prices ./prices.json \
    --out ./sweep-out
`

// cmdSweep handles the sweep subcommand. It parses flags, wires OS dependencies,
// and delegates to runSweep. Errors are returned with a "crackapack sweep: " prefix.
func cmdSweep(args []string, stdout, stderr io.Writer) error {
	fset := flag.NewFlagSet("sweep", flag.ContinueOnError)
	fset.SetOutput(stderr)
	fset.Usage = func() { fmt.Fprint(stderr, sweepUsage) }

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

	fset.StringVar(&configPath, "config", "", "YAML config file (optional; omit for defaults)")
	fset.StringVar(&catDir, "catalogue-dir", "", "catalogue directory (required)")
	fset.StringVar(&setsRaw, "sets", "", "comma-separated openable set codes (required)")
	fset.StringVar(&suppRaw, "supplementary", "", "comma-separated card-pool-only codes (optional)")
	fset.StringVar(&pricesPath, "prices", "", "JSON price snapshot file (required)")
	fset.StringVar(&player, "player", "collector", `player policy name (default "collector")`)
	fset.StringVar(&outDir, "out", "", "output root directory (required)")
	fset.StringVar(&ratesRaw, "rates", "", "comma-separated coin-per-USD rates (overrides config sweep_rates)")

	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("crackapack sweep: %w", err)
	}

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
		fset.Usage()
		return fmt.Errorf("crackapack sweep: required flags not set: %s", strings.Join(missing, ", "))
	}

	// Parse --rates flag when provided.
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

		engine, err := sim.NewEngine(rateCfg, in.catalogues, in.supplementary, in.prices, in.policyFn(), rng)
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

	// Write per-rate reports. Abort all writes on the first unknown-metric error
	// (already caught by Sweep above), so by the time we write, sr is valid.
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

// parseRates splits a comma-separated list of integers into a []domain.CoinRate.
// Returns an error naming the flag and the offending token.
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
	if len(rates) == 0 {
		return nil, fmt.Errorf("no valid rates found in %q", raw)
	}
	return rates, nil
}

