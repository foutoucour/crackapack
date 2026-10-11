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
	"sort"
	"strings"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// simulateArgs holds all dependencies for runSimulate, decoupled from the OS so
// that tests can substitute in-memory implementations (fs.FS, Writers, etc.).
type simulateArgs struct {
	// configReader is an open reader for the YAML config file.
	// nil means "use all defaults" (equivalent to an empty YAML document).
	configReader io.Reader

	// catFS is the catalogue directory; set files are loaded as "<CODE>.json".
	catFS     fs.FS
	setCodes  []string // openable set codes (must have a booster.draft section)
	suppCodes []string // supplementary card-pool codes (no booster section required)

	// pricesFS is the FS rooted at the directory that contains the price file.
	// pricesName is the relative file name within that FS.
	pricesFS   fs.FS
	pricesName string

	player   string
	policies map[string]func() sim.PlayerPolicy

	// newOutput creates (or truncates) an output file by name. The caller is
	// responsible for calling Close and checking its error.
	newOutput func(string) (io.WriteCloser, error)

	stdout io.Writer
}

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

// buildPolicies returns the map of registered player policies. T-011 adds its
// cash-out policy by inserting an entry here.
func buildPolicies() map[string]func() sim.PlayerPolicy {
	return map[string]func() sim.PlayerPolicy{
		"collector": func() sim.PlayerPolicy { return sim.NewCollectorPolicy() },
	}
}

// splitCodes splits a comma-separated string of set codes, trims whitespace,
// and removes empty entries.
func splitCodes(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// runSimulate is the testable core of the simulate subcommand. It loads all
// inputs, constructs the simulation engine, executes the run, and writes outputs.
// All returned errors include the relevant file or flag name.
func runSimulate(a simulateArgs) error {
	// 1. Load config. nil reader means "all defaults".
	reader := a.configReader
	if reader == nil {
		reader = strings.NewReader("")
	}
	cfg, err := config.Load(reader)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// 2. Load openable catalogues (booster.draft section required).
	catalogues := make([]domain.SetCatalogue, 0, len(a.setCodes))
	for _, code := range a.setCodes {
		name := code + ".json"
		cat, err := domain.LoadSetCatalogue(a.catFS, name)
		if err != nil {
			return fmt.Errorf("catalogue %s: %w", name, err)
		}
		catalogues = append(catalogues, cat)
	}

	// 3. Load supplementary card pools (no booster section required).
	supplementary := make([]domain.SetCatalogue, 0, len(a.suppCodes))
	for _, code := range a.suppCodes {
		name := code + ".json"
		pool, err := domain.LoadCardPool(a.catFS, name)
		if err != nil {
			return fmt.Errorf("supplementary %s: %w", name, err)
		}
		supplementary = append(supplementary, pool)
	}

	// 4. Load price snapshot.
	prices, err := domain.LoadPriceSnapshot(a.pricesFS, a.pricesName)
	if err != nil {
		return fmt.Errorf("prices %q: %w", a.pricesName, err)
	}

	// 5. Pick player policy.
	policyFn, ok := a.policies[a.player]
	if !ok {
		names := make([]string, 0, len(a.policies))
		for k := range a.policies {
			names = append(names, k)
		}
		sort.Strings(names)
		return fmt.Errorf("unknown player %q; valid names: %s",
			a.player, strings.Join(names, ", "))
	}

	// 6. Build RNG from the config seed, as documented in sim.NewEngine.
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))

	// 7. Build and run the engine.
	engine, err := sim.NewEngine(cfg, catalogues, supplementary, prices, policyFn(), rng)
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
