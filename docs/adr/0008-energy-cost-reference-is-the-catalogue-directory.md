# 0008 – The energy cost reference is the whole catalogue directory

## Status

Accepted — 2026-10-12

## Context

A booster's energy cost is its average value relative to a median booster value, mapped to a cost
tier, see [Energy](../core-features.md#energy). The simulation took that median over the openable
sets of the run. A set run alone was always at ratio 1 and cost 5, and its cost changed with
`--sets`. The game has about 150 sets, and a booster's cost cannot depend on which of them a player
happens to open.

## Decision

The median is taken over every openable set found in the catalogue directory, which means every set
file with a `booster.draft` section, whether or not `--sets` names it.

A set therefore costs the same energy alone or with others, as long as the directory is the same.
The commands read the directory through the injected `fs.FS` and pass the sets to the engine, which
does not read files. How files are picked, and which defects stop a run, is described in
[Catalogue directory](../simulation.md#catalogue-directory).

## Consequences

- `max_booster_wait_minutes` and the pace of a run no longer change when a set is added to `--sets`.
- Two runs compare only on the same directory: adding or removing a set file moves the median.
- Every run parses every set file of the directory, so the start-up cost grows with the number of
  files, not with `--sets`. No cache is built until that cost is a problem.
- A defective file that no run requested still stops the run. That is deliberate: skipping it would
  shift the median without a sign.
