# 0056 – The Counter remembers when it last had a card

## Status

Accepted — 2026-10-23

## Context

The game never shows how many copies the Counter holds: its stock is meant to feel scarce, see
[ADR-0049](0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md). A card out of stock
then needs another hint, and the maintainers chose one: the empty slot says when the Counter last had
the card ("last in stock 5 minutes ago"), or that it never did.

The domain never reads the clock, and the Counter counts its own time in ticks, which the game only
moves when it catches up, see
[ADR-0055](0055-the-game-catches-the-counter-up-on-whole-days-at-most-thirty.md). A purchase happens
between catch-ups, at an instant no tick stands for.

## Decision

A Counter built with `WithLastInStock(now)` remembers, for each card out of stock that it had, the
instant its last copy left: bought by the player, or gone at the end of its shelf life. It reads the
instant from `now`, the caller's clock.

- **A purchase** is remembered at the real instant.
- **A copy that vanished during a catch-up** is remembered at the instant of the tick it vanished,
  which the game sets while it replays the day.
- **A card back in stock** is forgotten, and remembered again when it next runs out.
- **A card the Counter never had,** such as a foil no booster gave it, has no instant: the slot says
  it was never in stock.

The simulation builds its Counter without the option, so it remembers nothing and costs nothing.

## Consequences

- The memory is saved with the game, and only holds cards out of stock, so it stays small.
- The game knows when a card ran out but never says how many copies there were.
- A card that came and went inside one catch-up shows the last instant it left, not the first.
