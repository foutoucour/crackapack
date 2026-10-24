# 0061 – The NPCs are saved in the game's file, in a compact form

## Status

Superseded by [ADR-0066](0066-a-save-names-the-cards-of-its-lists-by-a-dictionary.md)

## Context

The game's market trades with a live crowd of NPCs, see
[ADR-0058](0058-the-player-trades-on-a-market-of-requests-and-listings.md). A reload must change no
answer, so what each NPC holds and remembers has to survive a restart: the cards it keeps, its shelf
with the day each copy arrived, its wish list, and the moods its traits keep after a decline.

[ADR-0052](0052-the-game-is-saved-in-one-versioned-json-file.md) saves the game in one JSON file,
written whole after every action. Its consequences expect the NPC population to need a SQLite store
instead, because it is large. The crowd of the game's first steps was saved in that file as a list of
objects, one per card, and it reached megabytes after a month with a handful of NPCs.

Two ways out were weighed:

- **A file of its own for the NPCs,** written only when they change and linked to the save by an id,
  so that the two cannot drift apart. The NPCs change on every day caught up, so it is written about
  as often, and two files must be kept in step.
- **A compact form in the same file.** The cards are most of the size, and most of each object is
  the JSON around one card ID.

## Decision

The NPCs stay in the game's one file. Who they are is drawn again from the saved seed and the
config when the game loads, and the file holds only what changes.

Their cards are written compactly: one string per list, of entries separated by spaces. An entry is
the card ID, a mark for a foil, the tick for a shelf copy, and a count when it is not one. The
Counter's limited shelf, which the NPCs' surplus fills with thousands of copies, is written the same
way. A save from before still loads: the older lists of objects are read, and written back in the
compact form.

## Consequences

- The game keeps one file and one atomic write, so the NPCs and the player can never be saved apart.
- The file stays readable in a text editor, if less comfortably for the NPCs' cards.
- It supersedes nothing. ADR-0052 stands, and its SQLite store waits for a crowd the file cannot
  hold. The size of the save with the game's crowd, over the days, is in the
  [tuning log](../tuning-log.md).
- A card ID with a space, `=`, `@` or `*` cannot be written compactly, and its save fails rather
  than writing what would not read back. The MTGJSON IDs, which are UUIDs, have none.
