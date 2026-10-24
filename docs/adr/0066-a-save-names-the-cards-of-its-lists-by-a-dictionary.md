# 0066 – A save names the cards of its lists by a dictionary

## Status

Proposed — 2026-10-24. Supersedes [ADR-0061](0061-the-npcs-are-saved-in-the-games-file-in-a-compact-form.md).

## Context

[ADR-0061](0061-the-npcs-are-saved-in-the-games-file-in-a-compact-form.md) keeps the NPCs in the
game's one file and writes their cards, and the Counter's limited shelf, as compact strings with the
card ID in every entry. The save is rewritten whole after every action, see
[ADR-0052](0052-the-game-is-saved-in-one-versioned-json-file.md).

The IDs are most of what is left. An MTGJSON ID is a 36-character UUID, and the same cards come back
in the held list and the shelf of every NPC, and in the Counter's shelf. With the game's 12 NPCs the
save is 1.6 MB after 30 days and 2.0 MB after 90, see the [tuning log](../tuning-log.md).

## Decision

A save holds a dictionary of the card IDs its compact lists name, each once. An entry names a card
by its place in the dictionary and keeps the rest of its form: the mark for a foil, the tick, the
count. A place is a whole number, assigned in the order the lists first use a card.

The dictionary is one field of the Counter's part of the save, since the Counter's shelf and the NPCs
are written together. The lists of the player's collection, the Counter's stock and the market keep
their IDs, as they are short.

A save from before the dictionary has none, and its entries hold the IDs. It loads, and the next
save writes it with a dictionary. The version does not change, as ADR-0052 asks for a field that is
added.

## Consequences

- A save of a game with a crowd shrinks by more than half. A copy of a real save went from 359 KB to
  134 KB with the same game in it.
- Entries are no longer readable without the dictionary. The file is still JSON, and a text editor
  can look a place up.
- A card ID with a space, `=`, `@` or `*` can be saved. ADR-0061 refused it, since the entry would
  not read back.
- The dictionary is built again at every save from the cards the lists hold, so it never lists a card
  that no list names. It grows with the number of distinct cards the NPCs hold.
