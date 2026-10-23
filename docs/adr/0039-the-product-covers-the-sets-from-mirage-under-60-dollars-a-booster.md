# 0039 – The product covers the sets from Mirage on whose booster is worth under $60

## Status

Accepted — 2026-10-16

## Context

Calibration so far ran on a handful of 2005-2007 sets, and the targets behind
[ADR-0006](0006-coin-rate-is-100-per-dollar.md) are the demo's. The maintainers want the game to aim
at the full binder of the modern sets, and to leave out the old expensive ones: The Dark, Legends and
the sets around them.

The engine's expected booster value, from the 2026-10-13 catalogues and the price snapshot of
2026-10-15, splits the sets cleanly:

- Seven sets are worth $60 or more a booster: Alpha, Beta, Unlimited, Arabian Nights, Antiquities,
  Revised and Legends. Alpha alone is worth $1,078. These are the sets that break the economy, see
  [Expensive old sets and the 12-energy cap](../specs-review.md#expensive-old-sets-and-the-12-energy-cap).
- Every set from Mirage on is under $60. The priciest is Stronghold at $48.
- Before Mirage, The Dark ($19), Ice Age, Fourth Edition, Homelands and Fallen Empires are cheap
  boosters too, but the maintainers name The Dark among the sets to leave out.

## Decision

A set is in scope when all of these hold:

- It was released on or after Mirage, 1996-10-08.
- Its booster is worth under $60 at the price snapshot in use.
- It has a booster definition, draft or default, so it can be opened, see
  [ADR-0020](0020-a-set-file-booster-is-its-draft-or-default-definition.md).

The goal is the full binder of every set in scope.

## Consequences

- On the 2026-10-13 catalogues that is 107 sets, from Mirage to Lost Caverns of Ixalan. Sets from
  Murders at Karlov Manor on carry no booster definition that the loader reads, so they stay out
  until one exists for them.
- The date rule, not the price rule, removes The Dark and the other cheap pre-Mirage sets. The price
  rule still guards the rest: Stronghold sits at $48, and a price snapshot that moves it above $60
  takes it out of scope.
- Calibration scenarios draw their set lists from this scope, see
  [ADR-0040](0040-a-binder-takes-about-two-weeks-for-a-set-played-alone.md).
