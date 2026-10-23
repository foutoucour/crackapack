# 0048 – A dearer energy tier costs more energy

## Status

Accepted — 2026-10-22

## Context

A booster's energy cost comes from a table of tiers: the booster's value against the median booster of
the catalogue picks a tier, and the tier gives the cost, see [Energy](../core-features.md#energy) and
[ADR-0008](0008-energy-cost-reference-is-the-catalogue-directory.md). The config validation checked that
the thresholds rise and that every cost is above zero and within reach of the player's reserve. It did
not look at the order of the costs.

The first tuning search over the cost table drew a table of 3, 9, 8 and 12: a booster worth twice the
median cost less energy than one worth the median. The search had no constraint on it, the config loaded,
and the table then led to a config whose tuned defaults could not ship.

## Decision

Each cost of the table is above the cost of the tier before it. A config whose table repeats or lowers a
cost is refused when it is loaded, naming the key, such as `energy.cost_table[2].cost`.

The same refusal is the search's constraint: a candidate that breaks it scores the worst loss, see
[ADR-0045](0045-a-config-is-scored-by-its-distance-outside-the-bands.md). The product's search space
keeps every draw valid by giving each tier a range of its own, so no candidate is spent on the refusal.

## Consequences

- A config that gave two tiers the same cost no longer loads. None of the shipped configs, tests or demo
  data did, and the defaults of 3, 5, 8 and 12 stay as they are.
- A table of one tier is still valid.
- A search over a cost of the table has to keep its range between the costs of its neighbours.
