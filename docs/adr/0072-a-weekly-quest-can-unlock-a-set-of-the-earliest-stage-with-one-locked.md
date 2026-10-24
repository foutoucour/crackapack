# 0072 – A weekly quest can unlock a set of the earliest stage with one locked

## Status

Accepted — 2026-10-24

## Context

A binder filled to the unlock threshold unlocks sets of the next stage, drawn at random, see
[ADR-0054](0054-a-binder-past-the-threshold-unlocks-random-sets-of-the-next-stage-once.md). A player
who wants one set of the next stage cannot ask for it: the draw decides. A quest that names its reward,
see [ADR-0070](0070-a-quest-names-its-action-and-its-reward-and-pays-at-once.md), can let the player
go closer to the sets they want.

## Decision

A weekly quest can give the unlock of one named set.

- The set is drawn when the quests are drawn, among the locked sets of the earliest stage that still
  has one, the same stages as ADR-0054 reads. The quest never skips a stage.
- Doing the quest unlocks the set for good, as an unlock of a binder does.
- A game without stages has no locked set, so the quest is not drawn there.
- The unlock of a binder stays as it is. A quest adds a second way in and replaces nothing.

## Consequences

- A player who ignores the quest loses nothing, and one who follows it picks, among the sets the game
  names, which to go for. A quest names one set.
- The set a quest names can be unlocked by a binder before the quest is done. Doing the quest then
  changes nothing, and the quest is done all the same.
- An unlock is permanent, so no later sale locks the set again.
