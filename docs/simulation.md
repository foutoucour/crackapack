# Simulation guide

crackapack includes a headless simulation of the solo economy. It runs the real
domain logic — boosters, energy, the Counter, the coin ledger — without a UI or
real time. Time advances by ticks; a seeded generator makes every run reproducible.

## What it is for

The simulation lets maintainers tune the economy before releasing a change. Some
effects only show after weeks of play: the Counter's stock running dry, or a coin
rate that inflates the player's balance too fast. Playing through that is too slow;
the simulation runs it in seconds and compares the outcome to calibration targets.

The design is described in [Simulating the economy](specs-review.md#simulating-the-economy).
The inputs the simulation reads are defined in [Core features](core-features.md).

## Commands

Every subcommand accepts `--help` for detailed usage. Shell completion scripts
are available via `crackapack completion <shell>` (bash, zsh, fish, PowerShell).

### simulate

Run one solo-loop simulation and write results to a directory.

```
go run ./cmd/crackapack simulate \
  --catalogue-dir ./catalogues \
  --sets RAV,TSP \
  --supplementary TSB \
  --prices prices.json \
  --out ./out
```

Writes `result.json` (all metrics, one entry per day) and `days.csv`
(same data in tabular form) to `--out`.

### sweep

Run the simulation for each rate in the sweep grid, evaluate calibration targets
against each run, and report which rates pass.

```
go run ./cmd/crackapack sweep \
  --catalogue-dir ./catalogues \
  --sets RAV,TSP \
  --supplementary TSB \
  --prices prices.json \
  --out ./sweep-out
```

The sweep grid is taken from `sweep_rates` in the config file. Pass `--rates N,...`
to override it on the command line. Writes per-rate report files under `--out` and
a `sweep.json` aggregate, then prints the sweep table to stdout.

### compare

Compare two simulation results (JSON files produced by simulate or sweep).

```
go run ./cmd/crackapack compare before/result.json after/result.json
go run ./cmd/crackapack compare --days-csv delta.csv before/result.json after/result.json
go run ./cmd/crackapack compare before/result.json after/result.json --days-csv delta.csv
```

Reports which summary metrics changed and, optionally, the per-day delta as CSV.
Flags may appear before or after the positional file paths.

### demo

Run the sweep on the built-in synthetic demo dataset. No files or network needed.

```
go run ./cmd/crackapack demo
go run ./cmd/crackapack demo --out ./demo-out
go run ./cmd/crackapack demo --rates 100,200
```

The demo uses a fixed seed (42) so it is byte-identical on every run. Pass `--out`
to write the same per-rate reports as sweep. Pass `--rates` to override the default
grid (`[50, 100, 200, 400]`).

The demo runs in about one second. The binder milestones (`days_to_100pct_cu.*`) come
out identical at every rate on this dataset; the targets that split the grid are
`sub_coin_cards`, `daily_reward_commons` and `unsold_duplicate_copies`. Why the rate
leaves the milestones alone is explained in
[What the sweep answers](specs-review.md#what-the-sweep-answers-about-the-coin-rate).

## Input files

### Fetching real data

`tools/fetchdata` downloads the catalogue and price files from [MTGJSON](https://mtgjson.com)
on demand. Nothing is downloaded unless you run it.

```
go run ./tools/fetchdata --sets RAV,TSP,TSB
```

It writes `catalogues/<CODE>.json` for each set (override with `--catalogue-dir`) and a
`prices.json` snapshot (override with `--prices`). Pass them to the commands below with
`--catalogue-dir ./catalogues --prices prices.json`. Set codes are the MTGJSON ones; list a
supplementary pool such as `TSB` there too. Both default outputs are git-ignored.

The snapshot holds one price per card of the requested sets: the latest TCGplayer retail price in
US cents, non-foil first and foil for cards that exist only as foils. A card with no TCGplayer
price is left out, so the simulation prices it at `fallback_price` and counts it in
`unpriced_cards`.

Each run merges its prices into the `prices.json` that is already there, so the sets can be
downloaded in several calls.
- The prices of the sets of earlier calls stay as they were.
- A requested card takes its fresh price. One that has none now loses its old price.
- A `prices.json` the tool cannot decode stops the run and is left untouched.

Re-run the tool for a set to refresh its prices. The catalogue files are overwritten.

### Config YAML

All config keys and their defaults are listed in
[`internal/config/config.go`](../internal/config/config.go). The key groups:

| Group | Keys | Purpose |
|-------|------|---------|
| Economy | `coin_rate`, `counter_sell_pct`, `counter_buy_pct`, `welcome_bonus`, `daily_reward` | Coin flow and Counter pricing |
| Energy | `energy.*` | Opening pace; see [Energy](core-features.md#energy) |
| Counter | `initial_stock`, `fallback_price`, `counter_shelf_days` | Starting inventory, price floor and how long sold cards stay, see [The Counter's shelf life](#the-counters-shelf-life) |
| NPCs | `npcs.population`, `npcs.archetypes.*` | The NPC crowd; see [NPC crowd](#npc-crowd) |
| Simulation | `sessions_per_day`, `run_days`, `seed` | Run parameters |
| Sweep | `sweep_rates`, `targets` | Grid and calibration conditions |

Omit the config file entirely to use all defaults. Only the keys you set are
overridden; every other key keeps its default value, and a key set to `null` counts as not set.

Loading rejects settings that would make a run meaningless, naming the key: `sessions_per_day`
above the 144 ticks of a day, an energy cost no larger reserve plus savings can pay
(`energy.cost_table[i].cost` above `reserve_cap + savings_cap`), and a `days_to_*` target whose
maximum exceeds `run_days`.

### Catalogue directory

A directory of MTGJSON v5 set files, one per set code (`RAV.json`, `TSP.json`,
…). Openable sets (passed via `--sets`) must have a `booster.draft` section.
Supplementary pools (`--supplementary`) do not require it. An openable set keeps only the cards
that a booster sheet contains, see [ADR-0007](adr/0007-catalogues-count-only-obtainable-cards.md).

A card is common, uncommon, rare, mythic or special, see
[ADR-0013](adr/0013-mythic-is-a-rarity-of-its-own.md). Any other rarity stops the run.

MTGJSON files for sets before Mirage (1996) have no `booster.draft`, so the loader rejects them.
[Which sets can be simulated](specs-review.md#which-sets-can-be-simulated) lists them and the
workaround.

The directory also sets the reference of the energy cost tiers. A booster's cost is its average
value relative to the median over every openable set in the directory, not only the sets in `--sets`,
see [ADR-0008](adr/0008-energy-cost-reference-is-the-catalogue-directory.md).
- A file counts when it is named `<CODE>.json` with an upper-case code and has a `booster.draft`
  section. Other files, such as `prices.json` or a pool without `booster.draft` like `TSB.json`, are
  skipped.
- A set file with any other defect, such as malformed JSON or an unknown rarity, stops the run with
  an error that names the file, even when it is not in `--sets`.
- Compare runs on the same directory. Adding or removing a set file moves the median, and so every
  cost.

The price snapshot has to cover every set file of the directory. A set with no price in it would be
valued at `fallback_price` and pull the median down. `tools/fetchdata` merges each call into
`prices.json`, so fetch every set of the directory, in one call or several.

### Price snapshot JSON

A JSON object mapping MTGJSON UUID strings to integer prices in US cents:
`{"uuid": cents, ...}`. Cards absent from the snapshot fall back to
`fallback_price`.

## Reading the sweep table

The sweep prints a table with one row per rate and one column per calibration
target, plus the headline metrics `final_balance`, `sub_coin_cards`,
`daily_reward_commons` and `unsold_duplicate_copies` when they are not already
targets. Each target cell shows the metric value and `PASS` or `FAIL`.

The last line reports passing rates. When the passing rates form a contiguous block
in the grid, the report shows the range. When no rate passes, the line reads
"Passing rates: none". When no targets are configured, the line reads
"no targets configured".

`sweep.json` repeats the outcome for programs: the per-rate summaries and outcomes, the passing
rates, and `contiguous`, which is present (`true` or `false`) only when at least one rate passes.

## Summary metric names

`Summary` is a `map[string]float64` in the result JSON. All keys are lowercase
snake\_case; set-specific keys use the pattern `<name>.<setcode>` with the set
code lowercased.

A milestone not reached within the run is encoded as `float64(RunDays + 1)`, a censored value
above the run horizon. Config validation keeps it from passing by accident: an `at_most` or
`between` target on a `days_to_*` metric must have a maximum no greater than `run_days`.

| Metric | Definition |
|--------|-----------|
| `final_balance` | Player coin balance after the last day |
| `total_refused_trades` | Sum of sell attempts refused by the Counter (ErrSubCoin) across all days; volume signal for stuck inventory |
| `sub_coin_cards` | Distinct non-foil, non-basic-land cards, openable sets and supplementary pools included, that the Counter refuses to buy because the sale is worth less than one coin at the configured rate (a zero price counts); zero is the usual calibration target |
| `daily_reward_commons` | `floor(DailyReward / median_common_buy_coins)`; how many commons the daily reward buys at the Counter |
| `unpriced_cards` | Distinct non-basic-land cards, supplementary pools included, absent from the price snapshot and therefore priced at `fallback_price`; a count close to the whole catalogue means the snapshot is keyed wrongly |
| `unsold_duplicate_copies` | Copies above a playset (4) still held at the end of the run, whatever the policy's own keep count; non-zero when sub-coin cards cannot be sold |
| `days_to_50pct_cu.<setcode>` | First day (1-based) where the commons and uncommons owned reach 50% of the set's commons and uncommons |
| `days_to_100pct_cu.<setcode>` | First day (1-based) where every common and uncommon of the set is owned |
| `days_to_100pct.<setcode>` | First day (1-based) where every card of the set is owned: its whole binder |
| `coins_after_full_binder` | Coins the player gains from the first day every openable set is complete to the end of the run; `0` when that day is never reached. It measures the coins that have nothing left to buy |
| `max_booster_wait_minutes` | Longest wait, from empty reserve and empty savings, to afford one booster of any openable set. It follows from the energy settings and does not depend on play |
| `npc_boosters_opened` | Boosters the NPCs opened over the run. Present only when `npcs.population` is above zero |
| `npc_cards_sold` | Cards the NPCs sent to the Counter over the run. Present only with a crowd |
| `npc_cards_held` | Cards all NPC collections hold after the last day. Present only with a crowd |

Judge the pace of one set by running it alone, with `--sets RAV` for example. With several sets the
player shares energy between them, so every set takes longer. A set costs the same energy alone or
with others, because booster costs are relative to the median of the [catalogue directory](#catalogue-directory),
so `max_booster_wait_minutes` only changes when a set with a higher cost joins the run.

## Player policies

| Name | Behaviour |
|------|-----------|
| `collector` | Opens boosters for the set with the lowest C/U progress; sells duplicates above playset; buys missing non-foil cards cheapest-first up to a coin floor |
| `cash-out` | Opens boosters, sells everything above one copy and never buys, so the run measures how many coins the Counter creates |
| `idle` | Does nothing, so only the NPCs act and the run measures the supply they send to the Counter |
| `trader` | Plays like the `collector` and also trades with NPCs, see [Trades](#trades) |
| `exploiter` | Plays like the `collector` and chains trades across every NPC looking for a gain, see [Trades](#trades) |

## The Counter's shelf life

`counter_shelf_days` sets how long a card sold to the Counter stays in its stock. It is a feature
flag: the default `-1` keeps the Counter reselling what it buys, and a run with it is the same as
before it existed. Change one line in the config file to enable or disable it.

| Value | Meaning |
|-------|---------|
| `-1` | Cards stay forever. This is the default. |
| `0` | Cards vanish at once, so the Counter is a sink. |
| `N`, 1 or more | A card vanishes N days after it was sold. |

```yaml
counter_shelf_days: 2
```

The setting covers every card sold to the Counter, by the player or as NPC surplus. The player is
still paid for a sale, so coins are still created. A card the player bought from the Counter and
sells back goes on the shelf like any other.
- Initial stock never vanishes, since it is not a sold card.
- A purchase takes the oldest sold copy first, so the copies about to vanish are not wasted.
- A card sold at some tick vanishes exactly N days of ticks later.

With a crowd, the stock settles near the crowd's daily sales times N, so N becomes the dial of the
Counter's supply. With a sink or a short life the crowd stops flooding the Counter. A supply of
boosters opened by the Counter itself is not built yet, see
[TODO](todo.md#mechanics-not-simulated-yet-in-v1).

## NPC crowd

`npcs.population` sets the number of NPCs. It defaults to 0, and a run without NPCs is the same as
before the crowd existed: no NPC field appears in `result.json`, no NPC column in `days.csv` and no
`npc_*` metric in the summary. Which archetypes exist and what they stand for is described in
[NPCs and simulation](core-features.md#npcs-and-simulation).

Each tick, before the player's session, every NPC in ID order opens the boosters its rhythm has
come due for. A day's openings match its rhythm exactly, and an NPC never uses energy. Right after
each opening, the NPC sells the copy it just drew when it takes the NPC above its keep threshold.
The Counter takes it into stock without a ledger entry, because NPCs hold no coins.

An archetype is a named entry under `npcs.archetypes`, see
[ADR-0009](adr/0009-npc-archetypes-are-data.md) for supply and
[ADR-0010](adr/0010-npc-trade-behaviour-is-a-strategy.md) for trades. Its keys are in
[`internal/config/config.go`](../internal/config/config.go). The five default archetypes merge with
the ones in your file by name. An archetype you write replaces the default of the same name as a
whole, so every key of it must be set.

The crowd draws from its own random stream, derived from `seed`. The same seed still reproduces the
whole run. Adding NPCs to a config does not change what the player opens, so two runs that differ
only by the crowd stay comparable. The Counter's stock does change, and so can what a policy buys.

To measure supply alone, run the crowd without a player:

```
go run ./cmd/crackapack simulate \
  --catalogue-dir ./catalogues \
  --sets RAV,TSP \
  --supplementary TSB \
  --prices prices.json \
  --config npcs.yaml \
  --player idle \
  --out ./out
```

`compare` refuses a run with a crowd against a run without one, because their columns differ.

The archetype values in the defaults are starting guesses. Offline catch-up and persistence are not
built, see [Specs review](specs-review.md#v1-scope-and-build-order).

## Trades

`trades.enabled` turns on trades between the player and the NPCs. It defaults to `false`, and a run
without trades is the same as before they existed: no `trade_*` column in `days.csv`, no `Trade`
block in `result.json`, and no `trade_*` metric in the summary. Trades need a crowd
(`npcs.population` above 0) and `npcs.surplus_hold_days` of at least 1. The rules are in
[Trades with NPCs](core-features.md#trades-with-npcs).

An NPC holds the copies above its keep threshold on a shelf for `npcs.surplus_hold_days` days
before selling them to the Counter, see [ADR-0011](adr/0011-npcs-hold-surplus-on-a-shelf.md). Only
the shelf can be traded. Each NPC has a wish list, and when a card on it is a duplicate of the
player's, the NPC may propose a swap. A duplicate is offered for once, and the NPCs that may propose
today are drawn in random order, so the daily cap favours no archetype. The player can also propose one to any NPC. An NPC accepts
when it receives at least the reference value it gives plus its margin, never negative. When an
offer falls short, the NPC counters once with the smallest coin top-up that fits, see
[ADR-0012](adr/0012-an-npc-counters-once.md). The coins the player adds go to the
`trade_topup` account, because NPCs hold no coins.

An archetype names a trade behaviour, see
[ADR-0010](adr/0010-npc-trade-behaviour-is-a-strategy.md):

| Behaviour | Wishes for |
|-----------|------------|
| `casual` | Any card it lacks, at random |
| `playset_hunter` | Commons and uncommons, up to a playset, or up to its `keep_copies` when that is lower |
| `targeted_collector` | The cards it lacks, rarest first |
| `speculator` | The most valuable cards it lacks, and asks for the most valuable duplicates first |
| `whale` | Rares, mythics and specials, at random |

Every trade setting is a key of the configuration, so a scenario is a config file:

```yaml
npcs:
  population: 300
  surplus_hold_days: 2
  archetypes:
    speculator: {share: 10, min_boosters_per_day: 2, max_boosters_per_day: 4, keep_copies: 2,
                 behaviour: speculator, wish_size: 5, margin_pct: 25, offer_chance_pct: 1}
trades:
  enabled: true
  offer_expiry_days: 3
  max_cards_per_side: 10
  duplicate_keep: 1
  history_size: 0
  max_open_offers_per_day: 20
```

Run it with a player that trades:

```
go run ./cmd/crackapack simulate \
  --catalogue-dir ./catalogues --sets RAV,TSP --supplementary TSB \
  --prices prices.json --config trades.yaml --player trader --out ./out
```

Two policies trade, see `--player`:

- `trader` plays like the collector and also accepts NPC offers that bring a card it lacks, and
  swaps its duplicates for missing cards NPCs hold, paying a counter's top-up above the coin floor.
- `exploiter` chains trades across every NPC looking for a gain. It finds none: a test runs both
  policies for 30 days, at a margin of 0% and 10%, and checks that no proposal or acceptance raises
  the player's coins plus the reference value of its cards. A property test does the same for
  random offers straight against the Exchange.

A run with trades adds these metrics, summed over the run in the summary and per day in
`days.csv`: `trade_player_proposals`, `trade_npc_offers`, `trade_accepted`, `trade_countered`,
`trade_refused`, `trade_expired`, `trade_volume_coins` and `trade_topup_coins`. `days.csv` also
gets the `trade_topup` balance. `compare` refuses a run with trades against one without.

Which of these settings move the economy and which do not is measured in
[What the NPC crowd and trades change](specs-review.md#what-the-npc-crowd-and-trades-change).

To see the cost of a long run, `go test ./internal/sim -run '^$' -bench BenchmarkRun -benchmem`
runs 90 days with 500 NPCs with and without trades.

## Run diagram

```mermaid
flowchart LR
    C[config YAML] --> E[Engine]
    D[catalogue files] --> E
    P[price snapshot] --> E
    E --> R[Result<br/>Days + Summary]
    R --> J[result.json]
    R --> V[days.csv]
    R --> SW[sweep: rate grid]
    SW --> T[sweep table stdout]
    SW --> SJ[sweep.json]
```
