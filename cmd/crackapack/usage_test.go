package main

import (
	"io"
	"strings"
	"testing"
)

// TestUsage_showsNoArchetypeFlagOrDirectory asserts that no subcommand (nor the root) exposes
// an archetype flag or mentions archetypes in its usage text. Archetypes are embedded and
// read-only; the player cannot change them, see ADR-0023.
//
// UsageString() includes all flag names, so it covers both the short/long descriptions and
// every flag the command registers.
func TestUsage_showsNoArchetypeFlagOrDirectory(t *testing.T) {
	t.Parallel()
	root := newRootCmd(io.Discard, io.Discard)
	cmds := append(root.Commands(), root)
	for _, cmd := range cmds {
		cmd := cmd
		t.Run(cmd.Name(), func(t *testing.T) {
			t.Parallel()
			for _, text := range []string{
				cmd.UsageString(),
				cmd.Short,
				cmd.Long,
			} {
				if strings.Contains(strings.ToLower(text), "archetype") {
					t.Errorf("command %q: text contains 'archetype':\n%s", cmd.Name(), text)
				}
			}
		})
	}
}
