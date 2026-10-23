# 0046 – The search is staged and validated on unseen seeds

## Status

Accepted — 2026-10-16

## Context

A run of one set takes about 0.4 s of CPU, and a candidate runs every scenario: a search over a few
hundred configs, three seeds and a handful of scenarios is thousands of runs. Scoring every candidate
on every seed wastes most of them on configs that are clearly bad.

Scoring on few seeds has the opposite flaw: the best of hundreds of candidates is partly the luckiest
on its seeds, and a config tuned on one play style or one seed can miss the goals on another. The
[loss](0045-a-config-is-scored-by-its-distance-outside-the-bands.md) of the winner then flatters it.

## Decision

The search is a staged random search:

1. Draw N configs from the search space, each key from its range, uniformly or on a log scale.
2. Screen every valid config on one seed.
3. Score the best fraction of the valid ones on all the search seeds.
4. Validate the best few on seeds that no earlier stage used, and report the gap between the loss on
   the search seeds and on the validation seeds.

The best config is the one with the lowest loss on the search seeds. The validation does not choose
it: choosing on the validation seeds would use them up.

Everything derives from one search seed: the candidates, then the search seeds and the validation
seeds, drawn from one stream so that the two sets never share a value. The same seed gives the same
report whatever the number of workers.

## Consequences

- The gap is the answer to "did it get lucky": a large positive one says to search again with more
  seeds. It is read by a person; no threshold stops the search.
- The base config is scored first, on the search seeds, so the report shows how much the search gained.
- Random search ignores what earlier candidates showed. The report's parameter windows and influence
  are for a person to narrow the space and search again. A search that starts from the survivors, such
  as an evolution strategy, is left for when random search runs out of reach.
- A key a goal does not depend on is drawn like the others and costs nothing but its place in the
  space; its correlation with the loss will be near zero.
