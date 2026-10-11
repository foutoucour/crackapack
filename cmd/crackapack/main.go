// Command crackapack is the single binary entry point for the crackapack application.
// It dispatches CLI subcommands to their respective handlers.
package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: crackapack <subcommand>

Subcommands:
  simulate  run the NPC economy simulation
  sweep     sweep all card sets
  compare   compare two collections
  demo      run the app on the demo dataset
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run dispatches CLI subcommands and writes output to stdout and stderr.
// It is extracted from main so dispatch logic can be tested without process exit.
func run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("no subcommand given")
	}

	// dispatch maps each subcommand name to its handler.
	// Handlers currently return a not-implemented error until their story lands.
	dispatch := map[string]func() error{
		"simulate": func() error { return cmdSimulate(args[1:], stdout, stderr) },
		"sweep":    func() error { return cmdSweep(args[1:], stdout, stderr) },
		"compare":  func() error { return cmdCompare(args[1:], stdout, stderr) },
		"demo":     func() error { return cmdDemo(args[1:], stdout, stderr) },
	}

	fn, ok := dispatch[args[0]]
	if !ok {
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
	return fn()
}

// cmdSimulate will run the NPC economy simulation (not yet implemented).
func cmdSimulate(_ []string, _ io.Writer, _ io.Writer) error {
	return fmt.Errorf("simulate: not implemented")
}

// cmdSweep will sweep all card sets (not yet implemented).
func cmdSweep(_ []string, _ io.Writer, _ io.Writer) error {
	return fmt.Errorf("sweep: not implemented")
}

// cmdCompare will compare two collections (not yet implemented).
func cmdCompare(_ []string, _ io.Writer, _ io.Writer) error {
	return fmt.Errorf("compare: not implemented")
}

// cmdDemo will run the app on the demo dataset (not yet implemented).
func cmdDemo(_ []string, _ io.Writer, _ io.Writer) error {
	return fmt.Errorf("demo: not implemented")
}
