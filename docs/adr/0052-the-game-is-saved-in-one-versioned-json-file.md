# 0052 – The game is saved in one versioned JSON file

## Status

Accepted — 2026-10-23

## Context

The game is meant to be played over several days: energy recharges by the clock, so "wait for
tomorrow" only works if the collection, the energy and what the player was doing survive a restart.
Until now `play.Game` lived in memory and started over with the server, see
[ADR-0038](0038-the-games-energy-recharges-by-the-clock.md).

The architecture notes plan a SQLite store, with the NPC population and its collections in mind, see
[Architecture](../architecture.md#layers). The player's own state is small, a few kilobytes of
quantities, and nothing queries it. The game gains new state in each step, so the format must take
a new field without breaking the saves of the players who already have one.

## Decision

The state of the game is one JSON file, written whole after every action that changes it. The file
is replaced atomically: it is written to a temporary file next to it, flushed, and renamed over the
save, with mode 0600, so a crash leaves the previous save and never half of one.

The file has a `version`. A file of another version, with a field the game does not know, with data
after the document or that does not parse is an error that names the path. The game never replaces
such a file with a new game, since the player's progress is in it.

A part of the state that a file lacks loads as a new game has it, so that a step that adds a field
does not bump the version and an older save still loads. The version changes only when a field's
meaning changes.

The booster stream is saved as its seed and the opaque state of its generator, so a loaded game
opens the next boosters of the run instead of the first ones again. The seed that was saved wins
over `--seed`.

The file is `<user config dir>/crackapack/save.json`, `save-demo.json` for the demo, or the path of
`--save`. The composition root resolves it and injects a `play.Store`.

## Consequences

- A save is a file the player can copy, back up or move away, and read with a text editor.
- A save that fails, from a full disk or a path that cannot be written, does not stop the game. The
  game tells the player on every page until a save goes through, and the server logs it.
- Every save rewrites the whole file. That is fine for the player's state. The NPC population, which
  is large, does not belong in it: it needs the SQLite store, which this decision does not replace.
- A game that was saved with a set it no longer plays keeps that set's welcome boosters, and the
  cards it holds, until the set is played again.
- A set added after the first save has no welcome boosters: they were given once, at the start.
- A save written by a newer game is refused by an older one, since an older game would drop its
  state at the next save.
