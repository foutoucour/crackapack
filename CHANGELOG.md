# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `energy.coin_topup`, an optional coin top-up when opening a booster. When the player has fewer
  energy units than the booster costs, they may pay coins for the shortfall at rate
  `energy.coins_per_energy` (p). Off by default; needs `coins_per_energy > 0`. The player must
  always supply at least `min_energy_units` real energy — opening with coins only is never allowed.
  Three variants control pricing:
  - `min_energy` (default): each shortfall unit costs exactly `p` coins.
  - `escalating`: the k-th unit (0-indexed within the day) costs `p + k × escalation_step` coins,
    resetting at day start. Always costs at least `p` per unit, so net worth never increases.
  - `both`: the minimum real-energy floor and the escalating price both apply simultaneously.
  Composes with the mandatory booster fee (ADR-0019): the player pays top-up coins AND the fee in
  one atomic open. A run with top-up adds `coin_topup_units` and `coin_topup_coins` to the summary
  and `days.csv`. `compare` refuses a top-up run against one without. See
  [Energy coin top-up](docs/simulation.md#energy-coin-top-up).
- `converting-collector` player policy (`--player converting-collector`). Extends the `collector`
  with coin top-up in Phase 1: when short on energy for the next booster, it pays coins for the
  shortfall (no-op when `energy.coin_topup.enabled` is false). It also raises the coin floor when
  buying cards by the minimum booster fee, so the fee buffer is never spent on card purchases. This
  fixes the stall where the plain collector spends its balance on cheap cards and never accumulates
  enough for the mandatory fee.
- `energy.coins_per_energy`, a price in coins for one energy point. A booster worth more than the
  energy cap then costs the cap's energy and a coin fee for the rest, which leaves the economy into
  the `booster_fee` account. It is off by default, and a run without it is unchanged. A run with it
  adds `booster_fee_coins` to the summary and `days.csv`, and `compare` refuses it against a run
  without. See [ADR-0019](docs/adr/0019-the-player-pays-a-coin-fee-on-boosters-above-the-energy-cap.md).

### Changed

- A player that sells duplicates to a Counter with a treasury it has emptied offers each card once
  per session, not each copy, since the copies would be refused at the same price. Balances,
  treasury and sales are unchanged; `counter_broke_refusals` and `total_refused_trades` now count
  cards, not copies, so they fall by a large factor (86% in one 60-day run). Recorded in
  [ADR-0018](docs/adr/0018-cards-sold-to-the-counter-can-vanish.md) with the rule for cards that
  vanish.
- Config validation is stricter, after a stress test of the new Counter settings. Numeric keys have
  upper bounds, see [ADR-0014](docs/adr/0014-numeric-config-keys-have-upper-bounds.md). The Counter's
  energy reserve must hold the dearest tier, see
  [ADR-0015](docs/adr/0015-the-counters-reserve-must-hold-its-dearest-tier.md). A null item in a
  sequence, a float in an integer field and an unknown rarity under `initial_stock` are errors,
  where they were skipped, truncated and ignored, see
  [ADR-0017](docs/adr/0017-config-input-is-strict.md). A config that relied on any of them no longer
  loads.
- A run fails at the start, naming the set and the `--supplementary` flag, when a booster sheet holds
  a card that no loaded catalogue has, see
  [ADR-0016](docs/adr/0016-a-run-fails-when-a-sheet-card-has-no-catalogue.md).
- A config error is no longer prefixed with `config:` twice.

### Fixed

- The Counter's energy mode no longer panics, with `invalid argument to IntN`, when the energy costs
  of the sets are so varied that the weights of its pick overflow. It returns an error naming the
  costs.
- A large `coins_per_energy`, and the other money keys, can no longer overflow the ledger or the
  fee: they have upper bounds.

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

- `counter_budget`, a feature flag that gives the Counter a finite treasury instead of minted coins.
  A sale the treasury cannot pay is refused, and `daily_float` is the only new coin. With
  `counter_supply.coins_per_energy` the Counter also pays a coin fee from the treasury for boosters
  worth more than the energy cap. A run with a treasury adds `counter_fee_coins`,
  `counter_broke_refusals` and `counter_treasury`, and the `counter_treasury`, `counter_float` and
  `counter_fee` accounts. The demo config turns on the supply and a treasury large enough to leave
  its sweep unchanged. See
  [The Counter as a special player](docs/simulation.md#the-counter-as-a-special-player).
- `counter_supply`, a feature flag that makes the Counter open boosters to stock itself. The mode
  `fixed` opens a set number a day, with no energy, and the mode `energy` opens a booster whenever
  the Counter's own energy pays the cost of the set. The default `none` opens nothing, so existing
  runs are unchanged. A run that opens boosters adds the `counter_boosters_opened` and
  `counter_cards_opened` metrics. See
  [The Counter as a special player](docs/simulation.md#the-counter-as-a-special-player).
- `counter_shelf_days`, a feature flag for how long a card sold to the Counter stays in its stock,
  whether it comes from the player or from NPCs. `-1`, the default, keeps cards forever so existing
  runs are unchanged, `0` makes the Counter a sink and `N` makes a card vanish after N days. See
  [The Counter's shelf life](docs/simulation.md#the-counters-shelf-life).
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
