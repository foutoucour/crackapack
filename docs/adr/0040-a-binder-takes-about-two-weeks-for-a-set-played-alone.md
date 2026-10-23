# 0040 – A binder takes about two weeks for a set played alone

## Status

Superseded by [ADR-0050](0050-a-binder-is-never-fast-and-some-sets-are-slow-on-purpose.md)

## Context

The pacing goal agreed on 2026-10-12 is a set's binder in about two weeks at 3 visits a day, and
about one week at 6. It leaves open whether it holds for one set played alone or for a player who
collects several sets at once.

Energy is shared between the sets a player collects, so every set takes longer when more are in play,
see [Summary metric names](../simulation.md#summary-metric-names) and
[What it does to the pace](../specs-review.md#what-it-does-to-the-pace). A goal on a multi-set
player would change with the player's focus and the set list, and could not be tuned. The simulation
guide already says to judge the pace of one set by running it alone.

On RAV and TSP played alone, the simulation gave 8 days at 3 visits and 6 days at 6, so today the
pace is faster than the goal.

## Decision

The pacing goal is judged on one set played alone, on the metric `days_to_100pct.<set>`:

| Visits a day | Band, in days |
|-------------:|---------------|
| 3 | 10 to 18 |
| 6 | 5 to 9 |

The bands are the goal's two weeks and one week, give or take about four days at 3 visits and two at
6. The lower bound matters as much as the upper one: a binder in 8 days is too fast.

A player of several sets is not judged on pace. Played one after the other, the 107 sets of
[ADR-0039](0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md) make about four
years of play at 3 visits a day.

## Consequences

- Each set in scope is a scenario of its own, so the tuning system needs a set list per scenario
  rather than one list for the whole run.
- The bands are the product's, and the tuning system's loss measures the distance outside them.
- Whether the goal holds when a player does juggle several sets is left open on purpose. Energy
  sharing makes it slower, never faster.
