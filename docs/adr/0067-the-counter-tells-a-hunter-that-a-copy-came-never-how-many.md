# 0067 – The Counter tells a hunter that a copy came, never how many

## Status

Proposed — 2026-10-24

## Context

The Counter's stock is meant to feel scarce, so the game never shows how many copies it holds, see
[ADR-0049](0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md) and
[ADR-0056](0056-the-counter-remembers-when-it-last-had-a-card.md). A player who hunts a card, see
[ADR-0066](0066-a-hunted-card-asks-the-npcs-again-each-day.md), would have to open its binder over
and over to catch the Counter having it.

## Decision

The game tells the player that the Counter has a copy of a hunted card, and never how many.

- A hunted card shows whether the Counter has a copy, and its price, on the `/hunted` page.
- A card that comes to the Counter is new until the player has been shown it on the `/hunted` page.
  The header's Hunted link counts the new ones.
- A card that runs out and comes back is new again.
- Only a hunted card is told: the Counter's other cards keep showing as they do in the binders.

## Consequences

- The Counter's stock stays hidden. A hunter learns that a copy is there, as the Buy button of a
  binder already says, without opening the binder.
- The game notes a copy when it looks, so a card that came and went between two looks is never told.
- The player who wants the copy still pays the Counter's price, and may lose it to the
  Counter's shelf life or to a sale before buying.
