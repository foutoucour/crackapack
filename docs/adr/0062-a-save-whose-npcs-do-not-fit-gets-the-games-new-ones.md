# 0062 – A save whose NPCs do not fit gets the game's new ones

## Status

Proposed — 2026-10-23

## Context

The NPCs of a game are drawn from its seed and from the config's population, see
[ADR-0061](0061-the-npcs-are-saved-in-the-games-file-in-a-compact-form.md). The population is a
number of the product's config, and the tuning system will move it. A save written with another
population holds the state of NPCs this game does not have.

A save that does not fit the game is refused, and the player has to move it away to play, see
[ADR-0052](0052-the-game-is-saved-in-one-versioned-json-file.md). The player's progress is in the
save, and a change of the NPCs' number would cost it.

## Decision

The NPCs are the world, not the player's progress. A save whose crowd has another number of NPCs
than the game's loads with the game's new crowd, as a new game has it. The market drops what the old
NPCs answered and were still to answer, and keeps what the player asked for and listed.

## Consequences

- The population can change between two versions of the game without breaking a save.
- A player loses the answers and counters waiting at the time of the change, and nothing else.
- A save with NPCs loaded into a game without any is still refused, since nothing would hold its
  market.
