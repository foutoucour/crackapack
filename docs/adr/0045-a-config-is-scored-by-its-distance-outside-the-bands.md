# 0045 – A config is scored by its distance outside the bands

## Status

Accepted — 2026-10-16

## Context

The sweep judges each run pass or fail against its targets, see
[Reading the sweep table](../simulation.md#reading-the-sweep-table). It varies the coin rate alone, and
a pass or fail cannot rank two configs that both fail: a config that misses the pace by one day looks
as bad as one that misses it by ten.

The study of 2026-10-15 ran 40 configs over 7 parameters on one seed. It showed the shape of the
answer and not the values. Finding the values means comparing many configs, so each needs a number
that says how far it is from meeting the goals, the way a training run needs a loss.

The goals are bands on metrics, see [ADR-0040](0040-a-binder-takes-about-two-weeks-for-a-set-played-alone.md)
to [ADR-0043](0043-no-card-is-worth-less-than-one-coin-at-the-counter.md). Their metrics have different
units: days, coins, a count of cards, a percentage.

## Decision

The loss of a config is the sum, over the runs of the scenarios and over the bands judged on each,
of the distance of the metric outside its band, divided by the size of the band. It is averaged over
the simulation seeds. A loss of 0 means every goal is met on every seed.

The size of a band is:

- its width, for a `between` band with a width above zero;
- otherwise the magnitude of its bound, and at least 1.

So a metric a quarter of a band's width too high costs 0.25 whatever its unit, and `sub_coin_cards`
at most 0 costs 1 for each card.

A config that the loader refuses, or whose run fails, has the loss 1,000,000,000. It ranks last and
is never kept, so the search can draw combinations the validation rejects without special cases.

## Consequences

- Goals add up, and every band weighs the same. A goal that matters more has to get a narrower band
  or wait for a weight; there is none.
- The loss of a `between` band is a flat line outside the band, not a curve: a metric twice as far
  costs twice as much.
- Goals on a metric that a run does not report stop the search at once, since no config can fix them.
- The sum grows with the number of runs, so a goal judged on each of 107 sets weighs 107 times a goal
  judged once. That is on purpose for the pace goal of
  [ADR-0040](0040-a-binder-takes-about-two-weeks-for-a-set-played-alone.md), which holds per set.
