# 0064 – A new game's NPCs have most of a hold behind them

## Status

Proposed — 2026-10-23

## Context

The NPCs of a new game start with nothing, and a duplicate reaches an NPC's shelf only once it has
opened a few boosters, see [ADR-0011](0011-npcs-hold-surplus-on-a-shelf.md). A new player who asks
around for a card on the first day would find nobody holding it. The maintainers want both sides of
the market to work in the first minute, in the demo as in the game.

## Decision

When a new game has a market, its NPCs play the days of their surplus hold but one before the game
starts, a day at least. A copy leaves a shelf for the Counter only once it has waited the whole
hold, so the Counter receives none of that surplus, and its stock at the start is what its config
gives.

## Consequences

- The NPCs' shelves hold cards from the first minute.
- A new game takes a little longer to start, the time to play those days.
- A game loaded from a save is not warmed up again: its NPCs are in the save. A save whose NPCs are
  replaced gets new ones warmed up, see
  [ADR-0062](0062-a-save-whose-npcs-do-not-fit-gets-the-games-new-ones.md).
