# 0005 – cobra for the CLI

## Status

Accepted — 2026-10-11

## Context

The CLI had four subcommands (simulate, sweep, compare, demo), each implemented
with a hand-rolled `flag.FlagSet`. This produced duplicated boilerplate in four
places:

- Usage strings written by hand, with no guarantee of consistency.
- Required-flag validation duplicated per subcommand (four separate loops with
  their own `--flag-name` error messages).
- `flag.FlagSet` stops at the first positional argument, so `compare a.json
  b.json --days-csv delta.csv` was rejected. An explicit check detected this and
  returned an error telling the user to put flags first.
- No generated help structure, no shell completion.

The dispatch table in `main.go` (`map[string]func() error`) was a hand-written
reimplementation of what a command framework provides.

## Decision

The CLI is built on [cobra](https://github.com/spf13/cobra) (v1.10.2). Each
subcommand is a `*cobra.Command` constructed in its own file. The root command is
built in `newRootCmd(stdout, stderr io.Writer)`, injecting writers via
`cmd.SetOut` / `cmd.SetErr` and constructor parameters. `SilenceUsage` and
`SilenceErrors` are set on root so that error printing happens once, in `main`.

cobra uses pflag internally. pflag parses interspersed flags by default, so
`compare a.json b.json --days-csv delta.csv` now works without any special-case
code.

## Alternatives considered

| Alternative | Why not chosen |
|---|---|
| Keep `flag` package | Stops at first positional argument; no generated help or completion; required-flag validation and usage must be hand-written in every subcommand. |
| `urfave/cli v3` | Active project but less widely adopted than cobra; would gain the same features at the cost of a less familiar API; the Go module ecosystem heavily favours cobra. |
| `kong` | Struct-tag based configuration is concise for simple CLIs but makes dependency injection (injecting `io.Writer` into subcommands) awkward; adds a less common dependency. |

## Consequences

- **New dependencies** in `go.mod`: `github.com/spf13/cobra v1.10.2`,
  `github.com/spf13/pflag v1.0.9`, and `github.com/inconshreveable/mousetrap
  v1.1.0` (Windows-only cobra dependency).
- `crackapack help <command>` and `crackapack <command> --help` work without any
  additional code.
- `crackapack completion zsh` (and bash, fish, PowerShell) works out of the box.
- Flags may appear before or after positional arguments for all subcommands.
- The hand-written dispatch map in `main.go` is replaced by cobra's command tree.
- The testable entry point `run(args, stdout, stderr) error` is preserved; tests
  continue to call it or the `cmdX` shim functions without modification.
