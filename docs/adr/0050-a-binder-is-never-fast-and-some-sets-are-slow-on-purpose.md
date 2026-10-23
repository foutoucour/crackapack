# 0050 – A binder is never fast, and some sets are slow on purpose

## Status

Accepted — 2026-10-23

## Context

[ADR-0040](0040-a-binder-takes-about-two-weeks-for-a-set-played-alone.md) judges each set played alone
against one band, so every set shares the same ceiling and a set that takes longer is a miss.

The maintainers want the opposite for the ceiling. Sets differ in size and in the value of their
boosters, and a set that takes longer is rarer to complete and gives the game some feeling. A flat pace
across the whole scope would take that away.

The first search over the whole scope shows what the flat ceiling costs. Most of its loss came from the
slow sets, while many sets already finished under the floor on the defaults, see the
[tuning log](../tuning-log.md).

## Decision

The floor of ADR-0040 stays for every set: no binder is fast.

The ceiling is the length of the run the set is judged on, so a slow set passes and a set that never
finishes still fails. A set that is not finished by the end of the run reads as one day past it, see
[Summary metric names](../simulation.md#summary-metric-names), so a ceiling above the run length would
let it pass, and the two are set together.

The set played alone, and the exclusion of a player of several sets, stay as ADR-0040 decided.

The values of the floor, of the ceiling and of the run lengths are the goals' own. They live in
[`tune/goals.yaml`](../../tune/goals.yaml), and each change to them is recorded in the
[tuning log](../tuning-log.md).

## Consequences

- ADR-0040 is superseded. Its floor and its scope of one set alone stay, its ceiling does not.
- The tuning system's loss measures a distance outside a band, divided by the band's size, see
  [ADR-0045](0045-a-config-is-scored-by-its-distance-outside-the-bands.md). A wider band makes a set under
  the floor weigh less than before, so a loss under these bands does not compare with one under the old
  bands.
- A set that does not finish in the run still fails. Some sets did not finish on the first search, and
  their cause is not known, see the [tuning log](../tuning-log.md). Slow is welcome, never is not.
- The goals file of the tuning system and the demo's follow the new bands, and a test checks that no
  `days_to_` ceiling of the product's goals is above its scenario's run length.
