# 0070 – A quest names its action and its reward, and pays at once

## Status

Accepted — 2026-10-24

## Context

Energy is what limits the pace of a binder, and coins barely move it, see
[Economy: coins, sources and sinks](../core-features.md#economy-coins-sources-and-sinks). A reward
that gives coins is weak for that reason. The player feels what energy costs, so a quest has to give
something that serves the player's goal, enough to pull energy away from the race for a binder.

The design in [Retention](../retention.md) had a catalogue of generic quests, a claim with a click,
and a chest. A claim and a choice of reward after the fact take the control away from the moment
that matters: the player decides whether a quest is worth its cost before doing it.

## Decision

A quest is a fixed pair: an action to do and a reward, and its wording names both. For example "Open
2 boosters of Mirage: win this card".

- The quest names everything it needs: the set, the colour or the rarity, the card, the copies.
- The reward lands the moment the action is done. There is nothing to claim and nothing to choose
  afterwards.
- The player's control is which quests to follow. An ignored quest costs nothing and nothing carries
  over: a daily quest goes at midnight UTC, a weekly one on Monday.
- A reward that cannot be given when the action is done, because the card or the spare copies it names
  are gone, is lost. The quest is shown as lost and nothing else changes.

## Consequences

- The cost of a quest is in the open before the player starts it, which is what lets the player weigh
  it against the race for a binder.
- A quest the game cannot name for lack of a card or a set is not drawn, so a new player's board may
  hold fewer quests than the configuration asks for.
- The values of the quests, their targets and their caps, are starting guesses. They belong to the
  [tuning log](../tuning-log.md) once tuned, and not to an ADR.
- Quests are drawn from templates of the configuration, see `internal/config/quests.go`, so a change
  of a target or a reward needs no code.
