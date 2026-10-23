# 0042 – The Counter's rares are scarce but not gone

## Status

Proposed — 2026-10-16

## Context

The Counter is where the player buys the cards a booster did not give. If its rares dry up, a binder
cannot be finished there. If they pile up, nothing is scarce and the collection loses its pull.

No metric reads the Counter's rare stock today, and no number has been chosen for it, so the tuning
system's goal of a healthy stock has no band.

## Decision

Two metrics measure the rare and mythic printings of the sets in scope at the end of a run:

| Metric | Band |
|--------|------|
| Share of rare printings out of stock | 10% to 50% |
| Average copies in stock per rare printing | 0.5 to 3 |

Both are new and do not exist yet. Printings are counted per set, over rare and mythic cards, see
[ADR-0013](0013-mythic-is-a-rarity-of-its-own.md).

The numbers are a first guess with no run behind them. A first run that reads the metrics is expected
to move them, through a new ADR that supersedes this one.

## Consequences

- The metrics have to be built before the band can be judged, see
  [the tuning system](../todo.md#tuning-system-and-metrics).
- A stock that is never empty, under 10% out of stock, fails the band: some scarcity is the point.
