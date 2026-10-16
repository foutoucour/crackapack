# 0037 – A binder counts the pool cards its boosters can draw

## Status

Accepted — 2026-10-16

## Context

Time Spiral's boosters draw a card from the timeshifted set, `TSB`, which MTGJSON keeps as a file of
its own and the game loads as a supplementary pool, see
[ADR-0016](0016-a-run-fails-when-a-sheet-card-has-no-catalogue.md). A binder was the cards of the
set alone: 281 for Time Spiral, without the basic lands. A player who holds all of them still has
121 timeshifted cards to find, and the maintainer counts Time Spiral as 402 cards.

[Collection and binders](../core-features.md#collection-and-binders) says the master set follows what
MTGJSON describes, "at least foil, timeshifted and colorshifted", and
[Retention](../retention.md) has an "all timeshifted" milestone.

## Decision

A binder holds the cards of its set plus the cards of any supplementary pool that a booster sheet of
the set references and the set does not hold itself. `domain.DrawableCards` finds them and
`domain.WithPools` returns the set with them added, so that `Collection.BinderProgress` counts them
without a rule of its own.

The one timeshifted card that is in no booster is left out: nobody could ever hold it, and it would
make Time Spiral's binder impossible to complete.

Every consumer uses that one catalogue:

- the collection page, which shows the pool as a section of the binder, named after the pool;
- the simulation's daily snapshot, so `days_to_100pct.<set>` and `coins_after_full_binder` need the
  timeshifted cards;
- the player policies, through `WorldView.Catalogues`, so the collector, the converting collector
  and the trader buy and look for them like any other missing card. The Counter already stocks them.

Common-and-uncommon progress, which drives the choice of the set to open, is unaffected, since
timeshifted cards are special.

## Consequences

- Time Spiral's binder is 402 cards. A run that completes it must find the 121 timeshifted cards, by
  boosters, by the Counter or by trades, so `days_to_100pct.tsp` is later than it was.
- A figure recorded before this change for the whole binder of Time Spiral is stale. The
  common-and-uncommon figures are not.
- `WorldView.Catalogues` still lists openable sets only, but each holds more cards than the file
  of the set does. A policy that needs only the set's own cards must filter on the set's sheets.
- A set whose boosters draw from no pool is unchanged.
