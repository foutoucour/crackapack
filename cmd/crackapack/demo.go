package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/foutoucour/crackapack/demo"
)

const demoUsage = `usage: crackapack demo [flags]

Run a coin-rate sweep on the built-in synthetic demo dataset.
No files or network required; the dataset is generated from a fixed seed.

Flags:
  --out <dir>
        output root directory (optional; when given, writes per-rate reports
        and sweep.json alongside the sweep table printed to stdout)
  --rates <N,...>
        comma-separated coin-per-USD rates (optional; overrides the demo grid)

Example:
  crackapack demo
  crackapack demo --out ./demo-out
  crackapack demo --rates 50,100,200
`

// cmdDemo handles the demo subcommand. It builds the demo dataset in memory and
// delegates to runSweep. Only this function touches the OS (creating --out).
// Errors are returned with a "crackapack demo: " prefix.
func cmdDemo(args []string, stdout, stderr io.Writer) error {
	fset := flag.NewFlagSet("demo", flag.ContinueOnError)
	fset.SetOutput(stderr)
	fset.Usage = func() { fmt.Fprint(stderr, demoUsage) }

	var (
		outDir   string
		ratesRaw string
	)
	fset.StringVar(&outDir, "out", "", "output root directory (optional)")
	fset.StringVar(&ratesRaw, "rates", "", "comma-separated rates (overrides demo defaults)")

	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("crackapack demo: %w", err)
	}

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
}

// discardOutput is a newOutput function that silently discards all writes.
// Used when --out is not provided.
func discardOutput(string) (io.WriteCloser, error) {
	return nopWriteCloser{io.Discard}, nil
}

// nopWriteCloser wraps an io.Writer with a no-op Close method.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
