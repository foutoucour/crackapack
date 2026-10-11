package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/foutoucour/crackapack/internal/sim"
)

const simulateUsage = `usage: crackapack simulate [flags]

Run one solo-loop simulation and write results to the output directory.

Flags:
  --config <file>
        YAML config file (optional; omit to use all defaults)
  --catalogue-dir <dir>
        directory containing <CODE>.json MTGJSON set files (required)
  --sets <CODE,...>
        comma-separated set codes to open, e.g. RAV,CLS (required)
        each code names a <CODE>.json file in --catalogue-dir
  --supplementary <CODE,...>
        comma-separated card-pool-only codes, e.g. TSB (optional)
        each code names a <CODE>.json file in --catalogue-dir
        the file need not have a booster section
  --prices <file>
        JSON price snapshot file (required)
        format: {"<uuid>": <cents>, ...}
  --player <name>
        player policy name (default: collector)
  --out <dir>
        output directory for result.json and days.csv (required)

Example:
  crackapack simulate \
    --catalogue-dir ./catalogues \
    --sets RAV,CLS \
    --prices ./prices.json \
    --out ./out
`

// cmdSimulate handles the simulate subcommand. It parses flags, wires real OS
// dependencies, creates the output directory, and delegates to runSimulate.
// All errors are returned with a "crackapack simulate: " prefix.
func cmdSimulate(args []string, stdout, stderr io.Writer) error {
	fset := flag.NewFlagSet("simulate", flag.ContinueOnError)
	fset.SetOutput(stderr)
	fset.Usage = func() { fmt.Fprint(stderr, simulateUsage) }

	var (
		configPath string
		catDir     string
		setsRaw    string
		suppRaw    string
		pricesPath string
		player     string
		outDir     string
	)

	fset.StringVar(&configPath, "config", "", "YAML config file (optional; omit for defaults)")
	fset.StringVar(&catDir, "catalogue-dir", "", "catalogue directory containing <CODE>.json files (required)")
	fset.StringVar(&setsRaw, "sets", "", "comma-separated openable set codes, e.g. RAV,CLS (required)")
	fset.StringVar(&suppRaw, "supplementary", "", "comma-separated card-pool-only codes, e.g. TSB (optional)")
	fset.StringVar(&pricesPath, "prices", "", "JSON price snapshot file (required)")
	fset.StringVar(&player, "player", "collector", `player policy name (default "collector")`)
	fset.StringVar(&outDir, "out", "", "output directory for result.json and days.csv (required)")

	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // -h or --help: usage already printed, exit 0
		}
		return fmt.Errorf("crackapack simulate: %w", err)
	}

	// Validate required flags.
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
		return fmt.Errorf("crackapack simulate: required flags not set: %s", strings.Join(missing, ", "))
	}

	// Open the config file if provided. Defer close here; runSimulate reads it
	// synchronously before this function returns.
	var configReader io.Reader
	if configPath != "" {
		f, err := os.Open(configPath)
		if err != nil {
			return fmt.Errorf("crackapack simulate: config %q: %w", configPath, err)
		}
		defer func() { _ = f.Close() }() // reading only; close error is not actionable
		configReader = f
	}

	// Build FS abstractions from the OS paths.
	catFS := os.DirFS(catDir)
	pricesFS := os.DirFS(filepath.Dir(pricesPath))
	pricesName := filepath.Base(pricesPath)

	// Create the output directory before building the output creator.
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("crackapack simulate: --out %q: %w", outDir, err)
	}
	newOutput := func(name string) (io.WriteCloser, error) {
		return os.Create(filepath.Join(outDir, name))
	}

	a := simulateArgs{
		configReader: configReader,
		catFS:        catFS,
		setCodes:     splitCodes(setsRaw),
		suppCodes:    splitCodes(suppRaw),
		pricesFS:     pricesFS,
		pricesName:   pricesName,
		player:       player,
		policies:     buildPolicies(),
		newOutput:    newOutput,
		stdout:       stdout,
	}

	if err := runSimulate(a); err != nil {
		return fmt.Errorf("crackapack simulate: %w", err)
	}
	return nil
}

// runSimulate is the testable core of the simulate subcommand. It loads all
// inputs, constructs the simulation engine, executes the run, and writes outputs.
// All returned errors include the relevant file or flag name.
func runSimulate(a simulateArgs) error {
	in, err := loadInputs(a)
	if err != nil {
		return err
	}

	// 6. Build RNG from the config seed, as documented in sim.NewEngine.
	rng := rand.New(rand.NewPCG(in.cfg.Seed, in.cfg.Seed))

	// 7. Build and run the engine.
	engine, err := sim.NewEngine(in.cfg, in.catalogues, in.supplementary, in.prices, in.policyFn(), rng)
	if err != nil {
		return fmt.Errorf("engine: %w", err)
	}
	result, err := engine.Run()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	// 8. Write result.json.
	jsonW, err := a.newOutput("result.json")
	if err != nil {
		return fmt.Errorf("result.json: %w", err)
	}
	if writeErr := sim.WriteJSON(jsonW, result); writeErr != nil {
		_ = jsonW.Close() // write error takes precedence; close error discarded
		return fmt.Errorf("result.json: %w", writeErr)
	}
	if closeErr := jsonW.Close(); closeErr != nil {
		return fmt.Errorf("result.json: close: %w", closeErr)
	}

	// 9. Write days.csv.
	csvW, err := a.newOutput("days.csv")
	if err != nil {
		return fmt.Errorf("days.csv: %w", err)
	}
	if writeErr := sim.WriteCSV(csvW, result); writeErr != nil {
		_ = csvW.Close() // write error takes precedence; close error discarded
		return fmt.Errorf("days.csv: %w", writeErr)
	}
	if closeErr := csvW.Close(); closeErr != nil {
		return fmt.Errorf("days.csv: close: %w", closeErr)
	}

	// 10. Print summary to stdout.
	if err := sim.WriteSummary(a.stdout, result.Summary); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	return nil
}
