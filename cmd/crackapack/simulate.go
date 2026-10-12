package main

import (
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/internal/sim"
)

// cmdSimulate is a shim kept for backward-compatible test invocation.
func cmdSimulate(args []string, stdout, stderr io.Writer) error {
	return run(append([]string{"simulate"}, args...), stdout, stderr)
}

// newSimulateCmd returns the cobra command for the simulate subcommand.
// stdout receives simulation output; stderr receives errors, usage, and help.
func newSimulateCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		configPath string
		catDir     string
		setsRaw    string
		suppRaw    string
		pricesPath string
		player     string
		outDir     string
	)

	cmd := &cobra.Command{
		Use:   "simulate",
		Short: "run one solo-loop simulation and write results to the output directory",
		Long: `Run one solo-loop simulation and write result.json and days.csv to --out.

Omit --config to use all defaults. Only the keys you set are overridden;
every other key keeps its default value.`,
		Example: `  crackapack simulate \
    --catalogue-dir ./catalogues \
    --sets RAV,CLS \
    --prices ./prices.json \
    --out ./out`,
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
				return fmt.Errorf("crackapack simulate: required flags not set: %s",
					strings.Join(missing, ", "))
			}

			var configReader io.Reader
			if configPath != "" {
				f, err := os.Open(configPath)
				if err != nil {
					return fmt.Errorf("crackapack simulate: config %q: %w", configPath, err)
				}
				defer func() { _ = f.Close() }()
				configReader = f
			}

			catFS := os.DirFS(catDir)
			pricesFS := os.DirFS(filepath.Dir(pricesPath))
			pricesName := filepath.Base(pricesPath)

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
		},
	}
	// cobra help and usage go to stderr to match existing test assertions.
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	f := cmd.Flags()
	f.StringVar(&configPath, "config", "", "YAML config file (optional; omit for defaults)")
	f.StringVar(&catDir, "catalogue-dir", "", "catalogue directory containing <CODE>.json files (required)")
	f.StringVar(&setsRaw, "sets", "", "comma-separated openable set codes, e.g. RAV,CLS (required)")
	f.StringVar(&suppRaw, "supplementary", "", "comma-separated card-pool-only codes, e.g. TSB (optional)")
	f.StringVar(&pricesPath, "prices", "", "JSON price snapshot file (required)")
	f.StringVar(&player, "player", "collector", "player policy name")
	f.StringVar(&outDir, "out", "", "output directory for result.json and days.csv (required)")

	return cmd
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
