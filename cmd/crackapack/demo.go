package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/tune"
)

// newDemoCmd returns the cobra command for the demo subcommand.
// stdout receives the sweep table; stderr receives errors, usage, and help.
func newDemoCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		outDir   string
		ratesRaw string
		tune     bool
	)

	cmd := &cobra.Command{
		Use:   "demo",
		Short: "run a sweep on the built-in synthetic demo dataset",
		Long: `Run a coin-rate sweep on the built-in synthetic demo dataset.
No files or network required; the dataset is generated from a fixed seed.

The demo uses a fixed seed (42) so it is byte-identical on every run.
The demo player is the trader, so the report shows trades with NPCs.
Pass --out to write the same per-rate reports as sweep.
Pass --rates to override the default grid ([50, 100, 200, 400]).
Pass --tune to run a small config search on the demo dataset instead of the sweep; --out then
receives candidates.json, best.yaml and report.txt, and --rates is not used.`,
		Example: `  crackapack demo
  crackapack demo --out ./demo-out
  crackapack demo --rates 50,100,200
  crackapack demo --tune`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ds := demo.Build()

			if tune {
				return runDemoTune(cmd, ds, outDir, stdout, stderr)
			}

			a := sweepArgs{
				configReader: strings.NewReader(ds.ConfigYAML),
				catFS:        ds.Files,
				setCodes:     ds.SetCodes,
				suppCodes:    ds.SupplementaryCodes,
				pricesFS:     ds.Files,
				pricesName:   "prices.json",
				player:       "trader",
				policies:     buildPolicies(),
				newOutput:    discardOutput,
				stdout:       stdout,
			}

			if ratesRaw != "" {
				parsed, err := parseRates(ratesRaw)
				if err != nil {
					return fmt.Errorf("crackapack demo: --rates: %w", err)
				}
				a.rates = parsed
			}

			if outDir != "" {
				if err := os.MkdirAll(outDir, 0o755); err != nil {
					return fmt.Errorf("crackapack demo: --out %q: %w", outDir, err)
				}
				a.newOutput = func(name string) (io.WriteCloser, error) {
					full := filepath.Join(outDir, name)
					if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
						return nil, err
					}
					return os.Create(full)
				}
			}

			fmt.Fprintln(stdout, "crackapack demo: synthetic dataset (DEM + DSP/DST), seed 42")

			if err := runSweep(a); err != nil {
				return fmt.Errorf("crackapack demo: %w", err)
			}
			return nil
		},
	}
	// cobra help and usage go to stderr to match existing test assertions.
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	f := cmd.Flags()
	f.StringVar(&outDir, "out", "", "output root directory (optional; writes per-rate reports and sweep.json)")
	f.StringVar(&ratesRaw, "rates", "", "comma-separated coin-per-USD rates (optional; overrides the demo grid)")
	f.BoolVar(&tune, "tune", false, "run a small config search on the demo dataset instead of the sweep")

	return cmd
}

// demoTuneOptions returns the size of the demo search: small enough to finish in seconds.
func demoTuneOptions() tune.Options {
	return tune.Options{Seed: 1, Candidates: 24, KeepFraction: 0.25, Seeds: 2, ValidationSeeds: 2, Top: 2, Workers: 2}
}

// runDemoTune runs the demo search and, with outDir set, writes its outputs there.
func runDemoTune(cmd *cobra.Command, ds demo.Dataset, outDir string, stdout, stderr io.Writer) error {
	a := tuneArgs{
		configReader:  strings.NewReader(ds.TuneConfigYAML),
		spaceReader:   strings.NewReader(ds.TuneSpaceYAML),
		problemReader: strings.NewReader(ds.TuneGoalsYAML),
		catFS:         ds.Files,
		pricesFS:      ds.Files,
		pricesName:    "prices.json",
		policies:      buildPolicies(),
		opts:          demoTuneOptions(),
		newOutput:     discardOutput,
		stdout:        stdout,
	}
	a.opts.Progress = progressTo(stderr)
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("crackapack demo: --out %q: %w", outDir, err)
		}
		a.newOutput = func(name string) (io.WriteCloser, error) { return os.Create(filepath.Join(outDir, name)) }
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	fmt.Fprintln(stdout, "crackapack demo: config search on the synthetic dataset (DEM + DSP/DST)")
	if err := runTune(ctx, a); err != nil {
		return fmt.Errorf("crackapack demo: %w", err)
	}
	return nil
}

// discardOutput is a newOutput function that silently discards all writes.
// Used when --out is not provided.
func discardOutput(string) (io.WriteCloser, error) {
	return nopWriteCloser{io.Discard}, nil
}

// nopWriteCloser wraps an io.Writer with a no-op Close method.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
