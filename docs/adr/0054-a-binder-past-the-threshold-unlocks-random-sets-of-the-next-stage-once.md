# 0054 – A binder past the threshold unlocks random sets of the next stage, once

## Status

Accepted — 2026-10-23

## Context

With every set in scope, a new player faces more than a hundred sets at once. The maintainers want the
game to open them up as the player progresses: a first stage of a few quick sets, and more sets each
time a binder is nearly full. The sets are ranked and cut into stages as
[ADR-0053](0053-the-stages-rank-the-sets-by-the-energy-to-the-unlock-threshold.md) and
[Stages](../core-features.md#stages) describe. This decision is the rule that opens the later stages.

## Decision

A binder that reaches `unlock.binder_threshold` of its cards unlocks `unlock.sets_per_unlock` sets.

- **Which sets:** drawn at random among the locked sets of the earliest stage that still has one. When
  that stage has fewer, the draw takes them all and fills up from the stages after it.
- **Once per binder:** a binder unlocks the first time it reaches the threshold, and never again.
- **Any binder counts,** a locked set's included, since cards of a locked set can come from another
  set's boosters or, later, from the Counter.
- **Permanent:** an unlocked set stays unlocked, even if cards are sold and the binder drops below the
  threshold.
- **Reproducible:** the draw uses a random stream of its own, seeded from the game's seed and saved with
  the game, so a reload does not draw again.

The game checks the rule after every change that adds cards to the collection, and tells the player
which sets were just unlocked.

## Consequences

- The threshold is the same for every set, so a small set unlocks sooner than a large one, which is
  the point of ranking the sets.
- A player who fills the binders of the first stage has unlocked `unlock.sets_per_unlock` sets for
  each of them. The values of the keys are in the [tuning log](../tuning-log.md).
- A game played from a list of sets, `--sets RAV,TSP`, has no stage: every set is open. Its save keeps
  the unlocks of a staged game untouched, for the next staged game on the same save.
- A save written before the stages loads with the first stage open and every other set locked.
- A locked set's welcome boosters, from a save written before the stages, wait until the set is
  unlocked.
