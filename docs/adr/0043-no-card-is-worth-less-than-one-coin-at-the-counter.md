# 0043 – No card is worth less than one coin at the Counter

## Status

Accepted — 2026-10-16

## Context

At the rate of [ADR-0006](0006-coin-rate-is-100-per-dollar.md), a card the Counter would buy for less
than one coin is refused, and its duplicates pile up unsold. The window behind that ADR allowed up to
five such cards, a target taken from the demo, not chosen by the maintainers.

The same window kept the daily reward between 3 and 15 commons, which is what makes the reward worth
collecting without making it enough to play on.

## Decision

Two bands hold for the sets in scope, see
[ADR-0039](0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md):

| Metric | Band |
|--------|------|
| `sub_coin_cards` | 0 |
| `daily_reward_commons` | 3 to 15 |

## Consequences

- The five tolerated cards of the demo become zero.
- Measured on the 107 sets, 26,448 distinct cards, with the price snapshot of 2026-10-15, the default
  `counter_sell_pct` of 60 and `counter_buy_pct` of 120, and a daily reward of 100. The figures use
  the metrics' own formulas, computed outside the simulation. Both depend on prices only, not on play.
  - At 100 coins per dollar, 14 cards are sub-coin and a daily reward buys 5 commons. The sub-coin
    band fails.
  - From 167 to 188 coins per dollar both bands hold: no sub-coin card, and 3 commons a day.
  - Below 167, the cards priced at one cent are refused. At 189 and above, the reward buys 2 commons.
- The sub-coin band fails at the default rate of [ADR-0006](0006-coin-rate-is-100-per-dollar.md).
  The maintainers chose a price floor over a rate in the window, see
  [ADR-0044](0044-the-counter-pays-at-least-one-coin-for-a-priced-card.md). The band holds once the
  floor is implemented, and the rate stays at 100.
