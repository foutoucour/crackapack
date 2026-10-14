# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- `tools/fetchdata` merges the prices of a call into the existing `prices.json` instead of
  overwriting it, so the sets of a catalogue directory can be downloaded in several calls. A
  `prices.json` it cannot decode stops the run and is left untouched.
- `days.csv` has a `stock_mythic` column after `stock_special`. `result.json` is unchanged for
  runs on sets without mythics.
- [ADR-0009](docs/adr/0009-npc-archetypes-are-data.md) is superseded by ADR-0010: an archetype now
  names a trade behaviour, and its supply parameters stay data.
- A booster's energy cost is relative to the median booster value of every openable set in the
  catalogue directory, not of the sets in `--sets`. A set costs the same energy alone or with
  others, and `max_booster_wait_minutes` no longer changes with `--sets`. A set file in the
  directory that is malformed now stops a run even when it is not requested, see
  [ADR-0008](docs/adr/0008-energy-cost-reference-is-the-catalogue-directory.md).
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

- The `mythic` rarity, so the core sets Magic 2010 to Magic 2015 load. The Counter stocks it
  (`initial_stock.mythic`, default 2), the `whale` wishes for it and the `targeted_collector` ranks
  it above a rare, see [ADR-0013](docs/adr/0013-mythic-is-a-rarity-of-its-own.md). The demo set DEM
  holds three mythics.
- Trades with NPCs in the simulation, behind `trades.enabled` (default off, so existing runs are
  unchanged). NPCs hold their surplus on a shelf, wish for cards, propose swaps for the player's
  duplicates and counter a short offer once. Every setting is a config key, and two player
  policies trade: `trader` and `exploiter`. See [Trades](docs/simulation.md#trades),
  [ADR-0010](docs/adr/0010-npc-trade-behaviour-is-a-strategy.md),
  [ADR-0011](docs/adr/0011-npcs-hold-surplus-on-a-shelf.md) and
  [ADR-0012](docs/adr/0012-an-npc-counters-once.md).
- An NPC crowd in the simulation. `npcs.population` sets how many NPCs open boosters each tick and
  sell their surplus to the Counter, according to five archetypes held in `npcs.archetypes`, see
  [ADR-0009](docs/adr/0009-npc-archetypes-are-data.md). The default population is 0, so existing
  runs are unchanged. A run with a crowd adds the `npc_*` summary metrics and per-day NPC columns.
  `crackapack demo` now runs 60 NPCs.
- The `idle` player policy (`--player idle`), to measure what the crowd supplies on its own.
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
