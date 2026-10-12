// Command crackapack is the single binary entry point for the crackapack application.
// It dispatches CLI subcommands through a cobra command tree.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run dispatches CLI subcommands and writes output to stdout and stderr.
// It is extracted from main so dispatch logic can be tested without process exit.
func run(args []string, stdout, stderr io.Writer) error {
	root := newRootCmd(stdout, stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		// cobra reports an unknown subcommand as: unknown command "X" for "crackapack".
		// Reformat to our convention and print usage so the user sees what is available.
		if strings.HasPrefix(err.Error(), "unknown command ") && len(args) > 0 {
			fmt.Fprint(stderr, root.UsageString())
			return fmt.Errorf("unknown subcommand %q", args[0])
		}
		return err
	}
	return nil
}
