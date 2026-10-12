package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/internal/sim"
)

// compareArgs holds all dependencies for runCompare, decoupled from the OS so
// that tests can substitute in-memory implementations.
type compareArgs struct {
	// fsA is the FS containing file nameA (the baseline result).
	fsA   fs.FS
	nameA string

	// fsB is the FS containing file nameB (the second result).
	fsB   fs.FS
	nameB string

	// newOutput creates the days-CSV output file. nil when --days-csv is absent.
	newOutput func(string) (io.WriteCloser, error)

	// daysCsvName is the file name passed to newOutput. "" when absent.
	daysCsvName string

	// stdout receives the summary diff table.
	stdout io.Writer
}

// cmdCompare is a shim kept for backward-compatible test invocation.
func cmdCompare(args []string, stdout, stderr io.Writer) error {
	return run(append([]string{"compare"}, args...), stdout, stderr)
}

// newCompareCmd returns the cobra command for the compare subcommand.
// stdout receives the diff table; stderr receives errors, usage, and help.
// cobra/pflag parses interspersed flags, so --days-csv may appear before or
// after the positional file paths.
func newCompareCmd(stdout, stderr io.Writer) *cobra.Command {
	var daysCsvPath string

	cmd := &cobra.Command{
		Use:   "compare [--days-csv <file>] <a.json> <b.json>",
		Short: "compare two simulation results and report metric deltas",
		Long: `Compare two simulation result files (produced by simulate or sweep) and
report which summary metrics changed. Optionally write the per-day delta as CSV.

Flags may appear before or after the positional file paths.`,
		Example: `  crackapack compare baseline/result.json changed/result.json
  crackapack compare --days-csv delta.csv baseline/result.json changed/result.json
  crackapack compare baseline/result.json changed/result.json --days-csv delta.csv`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("crackapack compare: expected 2 result files, got %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			pathA, pathB := args[0], args[1]

			var newOutput func(string) (io.WriteCloser, error)
			var daysCsvName string
			if daysCsvPath != "" {
				daysCsvName = filepath.Base(daysCsvPath)
				csvDir := filepath.Dir(daysCsvPath)
				if err := os.MkdirAll(csvDir, 0o755); err != nil {
					return fmt.Errorf("crackapack compare: %q: %w", daysCsvPath, err)
				}
				newOutput = func(name string) (io.WriteCloser, error) {
					return os.Create(filepath.Join(csvDir, name))
				}
			}

			a := compareArgs{
				fsA:         os.DirFS(filepath.Dir(pathA)),
				nameA:       filepath.Base(pathA),
				fsB:         os.DirFS(filepath.Dir(pathB)),
				nameB:       filepath.Base(pathB),
				newOutput:   newOutput,
				daysCsvName: daysCsvName,
				stdout:      stdout,
			}

			if err := runCompare(a); err != nil {
				return fmt.Errorf("crackapack compare: %w", err)
			}
			return nil
		},
	}
	// cobra help and usage go to stderr to match existing test assertions.
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	cmd.Flags().StringVar(&daysCsvPath, "days-csv", "", "write per-day delta CSV to this file (optional)")

	return cmd
}

// runCompare is the testable core of the compare subcommand. It reads both
// result files, computes the comparison, and writes outputs.
// All returned errors include the relevant file name.
func runCompare(a compareArgs) error {
	// 1. Read file A.
	fA, err := a.fsA.Open(a.nameA)
	if err != nil {
		return fmt.Errorf("%s: %w", a.nameA, err)
	}
	defer func() { _ = fA.Close() }()

	resultA, err := sim.ReadResult(fA)
	if err != nil {
		return fmt.Errorf("%s: %w", a.nameA, err)
	}

	// 2. Read file B.
	fB, err := a.fsB.Open(a.nameB)
	if err != nil {
		return fmt.Errorf("%s: %w", a.nameB, err)
	}
	defer func() { _ = fB.Close() }()

	resultB, err := sim.ReadResult(fB)
	if err != nil {
		return fmt.Errorf("%s: %w", a.nameB, err)
	}

	// 3. Compare.
	cmp, err := sim.Compare(resultA, resultB)
	if err != nil {
		return err
	}

	// 4. Print summary diff table to stdout.
	if err := sim.WriteComparisonSummary(a.stdout, cmp); err != nil {
		return fmt.Errorf("summary: %w", err)
	}

	// 5. Write per-day delta CSV if --days-csv was given.
	if a.daysCsvName != "" && a.newOutput != nil {
		csvW, err := a.newOutput(a.daysCsvName)
		if err != nil {
			return fmt.Errorf("%s: %w", a.daysCsvName, err)
		}
		if writeErr := sim.WriteComparisonCSV(csvW, cmp); writeErr != nil {
			_ = csvW.Close() // write error takes precedence; close error discarded
			return fmt.Errorf("%s: %w", a.daysCsvName, writeErr)
		}
		if closeErr := csvW.Close(); closeErr != nil {
			return fmt.Errorf("%s: close: %w", a.daysCsvName, closeErr)
		}
	}

	return nil
}
