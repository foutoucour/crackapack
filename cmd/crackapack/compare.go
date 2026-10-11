package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/foutoucour/crackapack/internal/sim"
)

const compareUsage = `usage: crackapack compare [--days-csv <file>] <a.json> <b.json>

Compare two simulation result files and report the difference of each metric.
Flags must appear before the result file paths.

Flags:
  --days-csv <file>
        write per-day delta CSV to this file (optional)

Arguments:
  <a.json>  path to the baseline result file
  <b.json>  path to the second result file

Example:
  crackapack compare baseline/result.json changed/result.json
  crackapack compare --days-csv delta.csv baseline/result.json changed/result.json
`

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

// cmdCompare handles the compare subcommand. It parses flags, wires real OS
// dependencies, and delegates to runCompare.
// All errors are returned with a "crackapack compare: " prefix.
func cmdCompare(args []string, stdout, stderr io.Writer) error {
	fset := flag.NewFlagSet("compare", flag.ContinueOnError)
	fset.SetOutput(stderr)
	fset.Usage = func() { fmt.Fprint(stderr, compareUsage) }

	var daysCsvPath string
	fset.StringVar(&daysCsvPath, "days-csv", "", "write per-day delta CSV to this file (optional)")

	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // -h / --help: usage already printed, exit 0
		}
		return fmt.Errorf("crackapack compare: %w", err)
	}

	rest := fset.Args()

	// Go's flag package stops at the first positional argument. Detect any
	// flag-like token after the positional files and return a clear error.
	for _, a := range rest {
		if strings.HasPrefix(a, "-") {
			return fmt.Errorf(
				"crackapack compare: flags must come before the result files; got %q after positional arguments",
				a,
			)
		}
	}

	if len(rest) != 2 {
		fset.Usage()
		return fmt.Errorf("crackapack compare: expected 2 result files, got %d", len(rest))
	}

	pathA, pathB := rest[0], rest[1]

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
