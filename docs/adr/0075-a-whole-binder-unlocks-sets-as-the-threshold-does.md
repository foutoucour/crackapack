# 0075 – A whole binder unlocks sets as the threshold does

## Status

Proposed — 2026-10-24

## Context

A binder that reaches `unlock.binder_threshold` unlocks `unlock.sets_per_unlock` sets, drawn at random,
once, see [ADR-0054](0054-a-binder-past-the-threshold-unlocks-random-sets-of-the-next-stage-once.md).
Filling the binder whole is the hardest goal of a set, and it has paid nothing more than the
threshold did.

A first idea was to let the player pick any set of the next stage, or of the stage after it. That
needs a pending choice in the save and a page to make it, and a player who has not chosen yet leaves
the game in a state that none has today.

## Decision

The milestone of a whole binder unlocks sets with the same draw as the threshold: the sets of the
earliest stage that still has a locked one, `unlock.sets_per_unlock` of them, at random. It comes on
top of the unlock of the threshold, which stays as it is, and pays no energy.

- The player does not choose the sets.
- A game without stages has no locked set, so the milestone only locks its cards.

## Consequences

- No state is added for a choice, and the unlock lands the moment the binder is whole, as the
  threshold's does.
- A player who fills a binder opens twice as many sets as one who stops at the threshold.
- The choice of a set stays possible later, as a weekly quest already gives one, see
  [ADR-0072](0072-a-weekly-quest-can-unlock-a-set-of-the-earliest-stage-with-one-locked.md).
