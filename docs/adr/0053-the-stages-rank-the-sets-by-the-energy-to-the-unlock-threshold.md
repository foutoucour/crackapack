# 0053 – The stages rank the sets by the energy to the unlock threshold

## Status

Accepted — 2026-10-23

## Context

The game opens its sets in stages, from the quickest binder to fill to the slowest, see
[Stages](../core-features.md#stages). The 107 sets of
[ADR-0039](0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md) need an order,
computed when the game starts.

The pace the product aims at is the days to a full binder of a set played alone, see
[ADR-0051](0051-the-pace-of-the-scope-is-a-bell-curve-around-two-weeks.md). The tuning system reads it
from simulations, which take about a minute of CPU per candidate. That is far too slow for a start-up,
and the runs depend on the config being tuned.

The days to 100% are dominated by the rarest card. Ten sets hold cards that a booster gives once in
thousands, see [the tuning log](../tuning-log.md). A measure of the whole binder would rank those ten
sets by one card each, and not by how they play. The unlock happens before the binder is whole, at
`unlock.binder_threshold` of it, 90% by default.

## Decision

A set ranks by the energy a player who only opens its boosters spends to hold the unlock threshold of
its binder:

- **Boosters to the threshold:** the fewest boosters after which the expected number of binder cards
  not yet given is at most the share the threshold leaves out. Each card's chance to be in a booster
  comes from the booster options and the sheet weights of the catalogue. The binder counts the cards of
  the pools its boosters draw, as [ADR-0037](0037-a-binder-counts-the-pool-cards-its-boosters-draw.md)
  does, and leaves out basic lands.
- **Energy:** those boosters times the energy cost of one, so a dear booster ranks a set later, as it
  slows a player down, see [ADR-0048](0048-a-dearer-energy-tier-costs-more-energy.md).
- **Ties** break by set code, so the order is the same on every start.

It is computed from the catalogues and the price snapshot only, with no simulation.

## Consequences

- The order is cheap: the 107 sets rank in about 20 ms on a laptop, after the catalogues load.
- It ignores the Counter, trades and the daily reward. A set whose binder the Counter helps fill ranks
  as if the player opened boosters alone, the same for every set.
- A sheet's draws are counted as if each card were put back, which overstates the repeats in a booster
  a little, the same for every set.
- A threshold of 100% would rank by the rarest card again. The binder then counts as reached when less
  than half a card is expected to be missing, so the measure stays finite.
- On the real catalogues the quickest sets are small expansions such as Dragon's Maze and Born of the
  Gods, and the slowest include the sets with star printings and Masterpieces, such as Aether Revolt and
  the core sets from Seventh to Tenth Edition.
- Changing `unlock.binder_threshold` changes the order as well as the unlock.
