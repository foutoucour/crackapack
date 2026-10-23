# 0055 – The Counter catches up on whole days, up to a configured cap

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
a few hours, a figure written before the game had a Counter of its own, with no decision behind it.
With so short a cap a player away for a week finds the stock of an evening. The maintainers want a
long absence to still show, while the start stays quick and the stock cannot pile up without bound.

## Decision

The game replays the Counter's days lazily, when it loads a save and before each Counter action.

- **Whole days:** a day is a day of the simulation's ticks, from the instant the last catch-up ended.
  A part of a day waits for the next catch-up.
- **The same steps as the simulation:** the treasury's daily float, then each tick the Counter's shelf,
  its supply and the NPCs, in that order, through the same domain objects.
- **Up to `catch_up_days`:** only the most recent days are played. The days before them are skipped,
  not kept for later.
- **Reproducible:** the supply and the NPCs draw on random streams of their own, seeded from the game's
  seed and saved with the game.

## Consequences

- A catch-up that is not followed by an action is not saved. That is safe, since the saved state and
  streams replay the same days again.
- A player away for longer than the cap finds the stock of the last days of it, so an absence is never
  worth more than the cap.
- The cap is a config key, so a search can vary it. Its value, and what a catch-up costs at it, are in
  the [tuning log](../tuning-log.md).
