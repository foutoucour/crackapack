# 0049 – A rare is scarce at the Counter and still reachable by trade

## Status

Proposed — 2026-10-23

## Context

[ADR-0042](0042-the-counters-rares-are-scarce-but-not-gone.md) put two bands on the Counter's stock of
rare and mythic printings. It argued that if the Counter's rares dry up, a binder cannot be finished
there. That reads the Counter as the only way to get a card a booster did not give, and it is not.
The player also trades with NPCs, see [Trades](../simulation.md#trades), and can always open another
booster.

The design wants the opposite of a shop that has everything. "Everything can be bought at the Counter"
is a listed risk of the economy, see [Economy: risks and fixes](../specs-review.md#economy-risks-and-fixes).
Scarcity at the Counter is what keeps boosters and trades worth using, so a rare missing from its shelf
is not a failure while another route reaches it.

Two things in ADR-0042 and its first reading leave trades out:
- Its metrics read the Counter alone. They cannot tell a rare that is gone from the shortcuts from one
  an NPC holds on its shelf.
- Trades are off by default. With them on, an NPC holds its surplus on a shelf for a few days before it
  reaches the Counter, so the Counter's stock differs, see
  [ADR-0011](0011-npcs-hold-surplus-on-a-shelf.md). Every reading so far, those of the
  [tuning log](../tuning-log.md) included, was taken with trades off.

## Decision

The stock goals judge what the player can reach by buying or trading, with trades on. Three metrics
carry them, each read after the last day and each with a band of its own.

| Metric | Reads |
|--------|-------|
| `rare_out_of_stock_pct` | Share of rare and mythic printings of which the Counter holds no copy |
| `rare_stock_per_printing` | Copies the Counter holds per rare or mythic printing |
| `rare_unreachable_pct` | Share of those printings of which neither the Counter nor any NPC's shelf holds a copy |

The first two keep the definitions of ADR-0042. They measure how scarce the Counter is, and no longer
stand for whether a binder can be finished.

The third is new. A printing the Counter and every shelf lack leaves the player only the boosters, and
the goal is that few printings are in that state.

The bands are the goals' own. They live in [`tune/goals.yaml`](../../tune/goals.yaml), and each change to
them is recorded in the [tuning log](../tuning-log.md).

The runs that read these goals set `trades.enabled`, with a crowd and a shelf hold of at least one day.

## Consequences

- ADR-0042 is superseded. Its metrics stay, and its reasoning does not.
- The bands of the two Counter metrics were never read with trades on. The readings in the tuning log do
  not carry over, and a first run with trades is expected to move the bands, which the log records.
- `rare_unreachable_pct` has to be built before its band can be judged. The code, its tests and the
  demo data are not part of this ADR, and ship with the change that implements it, see
  [Demo data ships with the feature](../../.claude/skills/crackapack-conventions/SKILL.md#demo-data-ships-with-the-feature).
- It counts a copy on a shelf, not an NPC's willingness to trade it. A copy the NPC refuses to part
  with for what the player offers still counts, so the metric under-reports the printings that are out
  of reach.
- A shelf is read at one moment, and a copy stays on it a few days, so one reading is thin. Averaging
  the metric over the last days is a later refinement.
- Until the metric exists, the product's `tune/goals.yaml` keeps the two Counter goals and judges them
  without trades, and the [TODO](../todo.md#tuning-system-and-metrics) says so.
