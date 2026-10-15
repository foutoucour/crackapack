# 0020 – A set file's booster is its draft or its default definition

## Status

Accepted — 2026-10-15

## Context

MTGJSON gives most sets a `booster.draft` definition. The sets before Mirage, from Alpha to
Alliances, have a `booster.default` one and no `draft`. The loader required `draft`, so it skipped
those files as unopenable and a run that named one failed with `data.booster.draft: missing or
empty`. The cheap and mid-priced of them, such as Fallen Empires and Ice Age, could not be played,
and the dear ones could not be assessed, see
[Expensive old sets and the 12-energy cap](../specs-review.md#expensive-old-sets-and-the-12-energy-cap).

## Decision

`LoadSetCatalogue` reads the booster from `booster.draft` and, when a file has none, from
`booster.default`. The two have the same shape. `draft` wins when a file has both. A file with
neither is still the error it was, naming `data.booster.draft`, so callers keep telling a card pool
from a set file. An error inside a `default` definition names its own path,
`data.booster.default.…`.

Other definitions, such as the `starter` one of Ice Age, are ignored.

## Consequences

- Every pre-Mirage set file in a catalogue directory is now openable, and counts for the median of
  the energy cost reference, see
  [ADR-0008](0008-energy-cost-reference-is-the-catalogue-directory.md). On the directory used while
  writing this, 56 sets are openable instead of 42 and the median moves from $8.34 to $9.68, so
  the tiers of other sets move: MID drops from 5 to 3 energy, and the tiers of 3, 5, 8 and 12
  energy go from 0, 21, 14 and 7 sets to 7, 21, 13 and 15. RAV stays at 5 energy and TSP at 8, but
  TSP sits only 5% above the 5-energy threshold.
- Their price spread is wide, from $3 a booster for Fallen Empires to over $1,000 for Alpha. That
  is the subject of a later decision, not of this one.
- The fidelity of these definitions to the real boosters is not checked. Arabian Nights,
  Antiquities, The Dark, Fallen Empires and Homelands boosters hold 8 cards, where V1 assumes 15.
- A set is only as priced as the snapshot: Alpha, Beta and Unlimited have rares with no price, which
  the simulation values at `fallback_price`.
- The demo serves DSP with a `default` booster, so the demo exercises the fallback.
