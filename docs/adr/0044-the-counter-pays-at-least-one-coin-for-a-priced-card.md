# 0044 – The Counter pays at least one coin for a card it prices

## Status

Accepted — 2026-10-16

## Context

[ADR-0003](0003-prices-in-cents-and-integer-coin-rate.md) converts a price to coins with a floor and
refuses a trade worth less than one coin. At 100 coins per dollar and the Counter's sell percentage of
60, a card priced at one cent is worth 0.6 coin and is refused.

[ADR-0043](0043-no-card-is-worth-less-than-one-coin-at-the-counter.md) wants no such card in the sets
in scope. On those 107 sets, 14 of 26,448 distinct cards fall under it, and every one of the 14 is
priced at one cent: 5 cards carry that price in the snapshot of 2026-10-15, and 11 carry none and take
the fallback of one cent. A price of two cents already pays one coin.

The two ways out were a rate between 167 and 188 coins per dollar and a floor on the price. The
maintainers chose the floor, which leaves [ADR-0006](0006-coin-rate-is-100-per-dollar.md) as it is.

## Decision

When the Counter converts a positive price to the coins it pays, a result under one coin becomes one
coin. A price of zero is still refused.

ADR-0003's formula and its refusal stay as they are everywhere else: NPC trades, the coin fee on
boosters and the collection value do not get the floor.

## Consequences

- The Counter's buy price already pays at least one coin for any positive price at the default 120
  percent, so only the sell price changes. For a card at one cent, buying and selling both cost one
  coin, and the round trip through the Counter loses nothing. That is the one place the spread of
  ADR-0003 disappears, and it is worth one coin a card.
- `sub_coin_cards` has to use the same rule as the Counter, or the metric reports cards the Counter
  now buys.
- The code, its tests and the demo data are not part of this ADR. They ship in the change that
  implements it, see
  [Demo data ships with the feature](../../.claude/skills/crackapack-conventions/SKILL.md#demo-data-ships-with-the-feature).
