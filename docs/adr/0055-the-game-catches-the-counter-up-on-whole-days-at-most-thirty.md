# 0055 – The game catches the Counter up on whole days, at most thirty

## Status

Accepted — 2026-10-23

## Context

The game's Counter stocks itself as the simulation's does: it opens boosters of its own, and NPCs sell
it the copies above what they keep, see
[The Counter as a special player](../simulation.md#the-counter-as-a-special-player) and
[NPC crowd](../simulation.md#npc-crowd). The game is not running most of the time, yet a player who
comes back tomorrow should find a Counter that moved. Energy already recharges for the time the game
was closed, see [ADR-0038](0038-the-games-energy-recharges-by-the-clock.md).

[NPCs and simulation](../core-features.md#npcs-and-simulation) asked for an offline catch-up capped at
48 hours, a figure written before the game had a Counter of its own, with no decision behind it. At 48
hours a player away for a week finds the stock of two days. The maintainers asked for a cap of about a
month instead: enough that a long absence still shows, small enough that the start stays quick and the
stock cannot pile up without bound.

## Decision

The game replays the Counter's days lazily, when it loads a save and before each Counter action.

- **Whole days:** a day is 144 ticks of ten minutes, from the instant the last catch-up ended. A part of
  a day waits for the next catch-up.
- **The same steps as the simulation:** the treasury's daily float, then each tick the Counter's shelf,
  its supply and the NPCs, in that order, through the same domain objects.
- **At most `catch_up_days`:** only the most recent days are played, 30 by default. The days before them
  are skipped, not kept for later.
- **Reproducible:** the supply and the NPCs draw on random streams of their own, seeded from the game's
  seed and saved with the game.

## Consequences

- A catch-up that is not followed by an action is not saved. That is safe, since the saved state and
  streams replay the same days again.
- On the 107 sets of the scope, with the product's supply of 13 boosters a day, 30 days take 5 ms, and
  the start stays near one second. With 9 NPCs they take 34 ms.
- A player away for more than 30 days finds the stock of the last 30, so a long absence is not worth
  more than a month.
- The cap is a config key, so a search can vary it. Its value is in the [tuning log](../tuning-log.md).
