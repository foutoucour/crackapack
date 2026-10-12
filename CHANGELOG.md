# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- The CLI is built on [cobra](https://github.com/spf13/cobra), replacing the hand-written
  `flag.FlagSet` dispatch. Every subcommand now has generated `--help` output and shell
  completion is available via `crackapack completion <shell>`. Flags may appear before or
  after positional arguments (the `compare` subcommand no longer requires flags first).

### Added

- `crackapack simulate` — run one solo-loop simulation and write `result.json` and `days.csv`.
- `crackapack sweep` — sweep the coin-per-USD rate over a configurable grid, evaluate calibration
  targets, and report passing rates in a text table.
- `crackapack compare` — compare two simulation result files and report metric deltas.
- `crackapack demo` — run a sweep on the built-in synthetic demo dataset; no files or network
  required, deterministic from a fixed seed.
