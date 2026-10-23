# 0057 – The game plays the product's base config

## Status

Accepted — 2026-10-23

## Context

`crackapack play` read no config file: it played `config.defaults()`. The defaults keep every feature
flag off, so a run without them is the same as before the flag existed. With
them the game's Counter would open no booster and would only stock what the player sells it.

The product's goals are judged on [`tune/base.yaml`](../../tune/base.yaml), the defaults with the
Counter's supply on, see the [tuning log](../tuning-log.md). A game on another config would not be the
game the goals describe.

## Decision

`crackapack play` loads `tune/base.yaml`, embedded in the binary, on top of the defaults. The demo keeps
a config of its own, small enough to show every feature in a few boosters.

## Consequences

- A change to `tune/base.yaml` changes the game. Today the file only turns the Counter's supply on, 13
  boosters a day.
- The base config has no NPCs, so the game has none either. The game builds and saves a crowd when a
  config has one, which the demo does.
- The search's candidates still change only the simulation. A tuned default reaches the game through
  `config.defaults()` or `tune/base.yaml`, the way it reaches the simulation.
- `play` still has no `--config`.
