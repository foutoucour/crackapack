# 0073 – A milestone locks one copy of each of its cards

## Status

Proposed — 2026-10-24

## Context

A milestone is a goal of one set's binder, such as every common card of the set, see
[Milestones](../core-features.md#milestones). It pays a reward once. A player who could sell the
cards after earning the reward would complete a milestone, collect, and sell the cards back.

[Retention](../retention.md#collection-goals-and-badges) first proposed to take the reward back when
the last copy that completed a milestone leaves. That asks the game to track, on every sale and trade,
which milestones a card belongs to, and to pay the same reward again later, which rewards a player for
selling and buying back.

## Decision

Earning a milestone locks one copy of each card the milestone asks for. A locked copy cannot be sold
to the Counter, listed on the market or given in a trade, and stays in the collection for good. The
reward is never taken back and is paid once.

- A card has one lock, whatever the number of milestones that ask for it. A card locked by an earlier
  milestone needs no new lock.
- The lock is on a non-foil copy when the player holds one that the market does not hold, and on a
  foil otherwise, so that a foil stays free to trade.
- Copies above the locked one are free: a player who holds three copies of a card sells two.
- A card whose every copy the market holds for a listing does not count towards a milestone until
  the listing ends, since the lock needs a copy the player is free to keep.
- Before the milestone is earned nothing is locked: a player at 80% of the rares may sell all of
  them.

## Consequences

- The lock is a fact about the collection, so it lives on `domain.Collection`: `Remove` refuses the
  locked copy, and the market's availability check reads it, so no path to a sale can forget it.
- The save lists the locked cards and the milestones earned, see
  [ADR-0052](0052-the-game-is-saved-in-one-versioned-json-file.md).
- A milestone cannot be earned a second time, so a player cannot farm one.
- The collection keeps a floor that no sale can go below. A player who regrets a lock has no way out.
