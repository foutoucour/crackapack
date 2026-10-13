# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- The default coin rate is settled at 100 coins per dollar after a sweep on real data, see
  [ADR-0006](docs/adr/0006-coin-rate-is-100-per-dollar.md).
- An openable set keeps only the cards a booster sheet contains. Cards no booster holds, such as
  the ★ promos of Guildpact, no longer count towards the set's totals, so its binder milestone can
  be reached, see [ADR-0007](docs/adr/0007-catalogues-count-only-obtainable-cards.md).
- The CLI is built on [cobra](https://github.com/spf13/cobra), replacing the hand-written
  `flag.FlagSet` dispatch. Every subcommand now has generated `--help` output and shell
  completion is available via `crackapack completion <shell>`. Flags may appear before or
  after positional arguments (the `compare` subcommand no longer requires flags first).

### Added

- `go run ./tools/fetchdata --sets RAV,TSP` downloads MTGJSON set files and a USD price snapshot
  for those sets, ready for `simulate` and `sweep`.
- Summary metrics `days_to_100pct.<setcode>` (a set's whole binder), `coins_after_full_binder` and
  `max_booster_wait_minutes`, to check the pace of a set, the coins left idle once the binder is
  done, and the longest wait for one booster.
- `crackapack simulate` — run one solo-loop simulation and write `result.json` and `days.csv`.
- `crackapack sweep` — sweep the coin-per-USD rate over a configurable grid, evaluate calibration
  targets, and report passing rates in a text table.
- `crackapack compare` — compare two simulation result files and report metric deltas.
- `crackapack demo` — run a sweep on the built-in synthetic demo dataset; no files or network
  required, deterministic from a fixed seed.
