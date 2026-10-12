// Package main is the single binary entry point for the crackapack application.
package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// newRootCmd builds the cobra root command with all subcommands attached.
// stdout receives command output; stderr receives errors, usage, and help text.
// No globals, no init(): all state flows through constructor parameters.
func newRootCmd(stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "crackapack",
		Short:         "simulate and calibrate the solo pack-opening economy",
		SilenceUsage:  true,
		SilenceErrors: true,
		// RunE is called when crackapack is invoked with no subcommand.
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(stderr, cmd.UsageString())
			return fmt.Errorf("no subcommand given")
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(
		newSimulateCmd(stdout, stderr),
		newSweepCmd(stdout, stderr),
		newCompareCmd(stdout, stderr),
		newDemoCmd(stdout, stderr),
	)
	return root
}
