# 0006 – The coin rate is 100 coins per US dollar

## Status

Accepted — 2026-10-12

## Context

[ADR-0003](0003-prices-in-cents-and-integer-coin-rate.md) made the coin-to-USD rate a whole number
of coins per dollar and left its value to the simulation sweep. The sweep has now run on real data:
the Ravnica block and Time Spiral from MTGJSON, with TCGplayer retail prices.

- Both player policies, two set scenarios and ten seeds per combination gave the same passing
  window: 90 to 200 coins per dollar.
- At 50 and below, cheap commons are worth under one coin, so the Counter refuses to buy them and
  duplicates pile up unsold.
- Above 200, the daily reward buys fewer than 3 commons.
- At 60 to 80 only some seeds pass, because of a handful of unsold duplicates.

The targets behind that window are the demo's, chosen to compare the two datasets. They are
pacing targets, not a product decision.

## Decision

The default `coin_rate` is 100 coins per US dollar. It sits in the middle of the passing window,
so a change to the price snapshot or to the targets is unlikely to move it out. The daily reward
of 100 coins is worth one dollar and buys about 6 commons.

## Consequences

- A $0.05 common is worth 5 coins at the reference value and 3 coins when sold to the Counter at
  60%, so the cheapest cards stay tradeable but coarsely priced.
- The rate hardly changes pacing: across the window the median day of the binder milestone moves
  by about one day, see
  [What the sweep answers](../specs-review.md#what-the-sweep-answers-about-the-coin-rate).
- Revisit it if the targets change, if the price source changes, or if coin sinks change the
  daily reward's weight.
