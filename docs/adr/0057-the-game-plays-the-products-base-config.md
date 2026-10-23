# 0057 – The game plays the product's base config, with a play overlay

## Status

Proposed — 2026-10-23

## Context

`crackapack play` read no config file: it played `config.defaults()`. The defaults keep every feature
flag off, so a run without them is the same as before the flag existed. With them the game's Counter
would open no booster and would only stock what the player sells it.

The product's goals are judged on [`tune/base.yaml`](../../tune/base.yaml), the defaults with the
Counter's supply on, see the [tuning log](../tuning-log.md). A game on another config would not be the
game the goals describe.

The base config alone does not give the game the Counter the maintainers want. Its initial stock holds
every printing, rares included, and its shelf keeps every card forever, since initial stock never
vanishes, see [ADR-0018](0018-cards-sold-to-the-counter-can-vanish.md). Every card would then be in
stock from the first day, and "out of stock" would only ever show for a foil. The game wants a Counter
that can miss a card, so that a rare is something to watch for and a player can miss one by minutes,
see [ADR-0056](0056-the-counter-remembers-when-it-last-had-a-card.md).

## Decision

`crackapack play` loads `tune/base.yaml`, embedded in the binary, on top of the defaults, then applies
[`tune/play.yaml`](../../tune/play.yaml), a list of overrides in the form of the search's candidates.
The search reads the base config only.

The overlay makes the Counter scarce:
- **A limited shelf:** a card sold to the Counter, or opened by it, vanishes after a while.
- **No rare or mythic at the start:** they reach the Counter only through its daily boosters and the
  player's sales, and stay reachable by trade once trades are in the game, see
  [ADR-0049](0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md).

The demo keeps a config of its own, small enough to show every feature in a few boosters.

## Consequences

- A change to `tune/base.yaml` changes the game and the search. A change to `tune/play.yaml` changes
  the game only.
- The game is not exactly the world the goals judge: its Counter is scarcer. A search that should judge
  the game's world has to vary the overlay's keys too, or move them into the base.
- The base config has no NPCs, and neither does the overlay, so the game has none. The game builds and
  saves a crowd when a config has one, which the demo does.
- The values of the overlay and why they were chosen are in the [tuning log](../tuning-log.md).
- `play` still has no `--config`.
