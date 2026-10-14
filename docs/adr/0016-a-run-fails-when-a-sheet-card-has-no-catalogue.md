# 0016 – A run fails when a sheet card has no catalogue

## Status

Accepted — 2026-10-14

## Context

The booster sheets of some sets hold cards that live in another file: Time Spiral's reference the
timeshifted cards of TSB, loaded as a supplementary pool, see
[Catalogue directory](../simulation.md#catalogue-directory). A run with `--sets RAV,TSP` and no
`--supplementary TSB` started without complaint. It failed later, with "card not in catalogue", as
soon as the Counter had a supply or the NPCs existed and one of those cards was drawn. A solo run
could fail the same way when the player sold such a card.

## Decision

`NewEngine` checks, before anything else is built, that every card on a booster sheet of an
openable set exists in an openable or supplementary catalogue. When one does not, it returns an
error naming the set, how many cards are missing, the smallest missing card ID and the flag that
fixes it, `--supplementary`.

## Consequences

- A run that was missing a supplementary pool now fails at the start, whether or not it would have
  drawn the missing card. Such runs were already incorrect.
- The error is the same on every run: it counts distinct cards and names the smallest ID.
- It does not say which pool to pass, since the engine only knows that no loaded catalogue has the card.
