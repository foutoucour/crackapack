# 0017 – Config input is strict about nulls, floats and rarity names

## Status

Accepted — 2026-10-14

## Context

The YAML decoder accepted three kinds of input silently, each giving a config the author did not
write:
- A null item in a sequence was skipped, so `cost_table: [null, {threshold: 400, cost: 12}]` lost a
  tier without a word, and `targets: [null]` passed as no targets. The code comments and the
  simulation guide said a null item was rejected, and one test pinned the opposite.
- A float in an integer field was truncated: `coin_rate: 1.5` ran as 1 and
  `coins_per_energy: 2.5` as 2.
- A name under `initial_stock` that is not a rarity was ignored, so `comon: 5` changed nothing.

## Decision

Config loading rejects all three, with a message naming the key.
- A null item in any sequence is an error: `<path> must not be null`.
- A float scalar, including one with no fractional part such as `100.0`, is an error where an
  integer is expected: `<path> must be a whole number`. The bounds of a calibration target,
  `targets[i].min` and `targets[i].max`, are the only keys that take floats.
- A key under `initial_stock` must be common, uncommon, rare, special or mythic.

A null value of a mapping key stays valid and means the key is absent, so the default applies.

## Consequences

- A config that relied on a skipped null item, a truncated float or an ignored rarity now fails to
  load. The test that pinned the skipped null item now pins the error.
- The check walks the raw YAML tree before the null pruning, so it does not depend on how the
  decoder handles a given type.
