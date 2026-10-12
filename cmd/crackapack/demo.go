package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/demo"
)

// newDemoCmd returns the cobra command for the demo subcommand.
// stdout receives the sweep table; stderr receives errors, usage, and help.
func newDemoCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		outDir   string
		ratesRaw string
	)

	cmd := &cobra.Command{
		Use:   "demo",
		Short: "run a sweep on the built-in synthetic demo dataset",
		Long: `Run a coin-rate sweep on the built-in synthetic demo dataset.
No files or network required; the dataset is generated from a fixed seed.

The demo uses a fixed seed (42) so it is byte-identical on every run.
Pass --out to write the same per-rate reports as sweep.
Pass --rates to override the default grid ([50, 100, 200, 400]).`,
		Example: `  crackapack demo
  crackapack demo --out ./demo-out
  crackapack demo --rates 50,100,200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ds := demo.Build()

			a := sweepArgs{
				configReader: strings.NewReader(ds.ConfigYAML),
				catFS:        ds.Files,
				setCodes:     ds.SetCodes,
				suppCodes:    ds.SupplementaryCodes,
				pricesFS:     ds.Files,
				pricesName:   "prices.json",
				player:       "collector",
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

	return cmd
}

// discardOutput is a newOutput function that silently discards all writes.
// Used when --out is not provided.
func discardOutput(string) (io.WriteCloser, error) {
	return nopWriteCloser{io.Discard}, nil
}

// nopWriteCloser wraps an io.Writer with a no-op Close method.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
