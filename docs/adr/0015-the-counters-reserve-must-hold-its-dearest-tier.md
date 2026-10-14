# 0015 – The Counter's energy reserve must hold the dearest tier

## Status

Accepted — 2026-10-14

## Context

The Counter's energy mode, see
[The Counter as a special player](../simulation.md#the-counter-as-a-special-player), reused the
player's rule: a cost must not exceed `reserve_cap + savings_cap`, the most energy the pool can hold.
For the player that is right, since the player waits for a booster it chose. The Counter keeps its
pick until it can pay, and savings only grow while the reserve is full, one point each
`savings_interval_ticks`. A Counter with `reserve_cap: 6` and `savings_cap: 6` passed validation and
yet opened 7,138 boosters in 120 days at an interval of 1, 81 at 144, and almost none at 100,000. The
opening rate then depended on the savings interval instead of the recharge.

## Decision

For the Counter, the dearest energy cost of the cost table must fit in `counter_supply.energy.reserve_cap`
alone. Savings do not count. The error names `counter_supply.energy.reserve_cap`, the dearest cost
and the table entry that holds it. The player's own rule, `reserve_cap + savings_cap`, is unchanged.

## Consequences

- A Counter energy that relied on savings to afford a tier is rejected. The default, a reserve of
  100 against a dearest cost of 12, is unaffected.
- A tier that no set of the run maps to still counts, since the config does not know the sets.
- The Counter and the player now follow different rules, on purpose, because their picks differ.
