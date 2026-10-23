# 0051 – The pace of the scope is a bell curve around two weeks

## Status

Accepted — 2026-10-23

## Context

[ADR-0050](0050-a-binder-is-never-fast-and-some-sets-are-slow-on-purpose.md) lets slow sets pass and keeps
a floor for every set. It still judges each set on its own, and a set inside its band passes whatever the
other sets do. A config that finishes most sets at the floor passes as well as one that spreads them.

The maintainers want a spread: most sets take about two weeks, a few are quick and a few are slow, so the
times across the sets form a bell curve. That is a statement about the sets together, and a band on one
set cannot make it.

Today the times are a lopsided pile: most sets finish well before two weeks, a long tail runs out, and a
spike of sets does not finish at all, see the [tuning log](../tuning-log.md).

## Decision

The pace goal judges the shape of the days to a full binder across the sets of the scope, each played alone:
a bell curve around the pace the product aims at, about two weeks for a set at the usual visit rate. A
config scores by how far the sets sit from their place in that bell.

This replaces the floor and the ceiling of ADR-0050. The bell's own lower tail says how many quick sets are
acceptable, and its upper tail how many slow ones.

A set that does not finish by the end of the run reads as one day past it, far out in the tail, so it costs
more than a slow set that finishes.

The center, the spread and the run length of the bell are the goal's own. They live in
[`tune/goals.yaml`](../../tune/goals.yaml), and each change to them is recorded in the
[tuning log](../tuning-log.md).

## Consequences

- ADR-0050 is superseded. Its aim that slow sets are welcome stays. Its per-set floor and ceiling go, and
  so does "no binder is fast" as a hard rule.
- The tuning system's goals gain a second kind, a shape on a metric across the runs of a per-set scenario,
  next to the bands. Each set counts as a band does in the loss, see
  [ADR-0045](0045-a-config-is-scored-by-its-distance-outside-the-bands.md), and a loss under the new goal
  does not compare with one under the old.
- A shape needs a scenario of several sets, one run per set. With few sets the reading is coarse, as in
  the demo.
- A set that never finishes weighs heavily in the shape. Those sets need a cause of their own, see the
  [tuning log](../tuning-log.md).
- The demo ships a shape goal, and a test checks that a bell fits inside the run length of each shape of the
  product's goals.
