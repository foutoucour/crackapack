# 0014 – Numeric config keys have upper bounds

## Status

Accepted — 2026-10-14

## Context

A stress test of the simulation, with randomised configs and the extreme values of every key, found
that nothing bounded the numeric keys from above. A `daily_reward`, a `welcome_bonus` or a
`counter_budget.initial` near the int64 limit overflowed: the player balance or the treasury went
negative and the ledger stopped summing to zero. A `coin_rate` or a percentage of 1e15 failed at run
time. An `energy` or `boosters_per_day` of a million made a run last minutes or fill the memory. The
cost table of the energy could hold costs large enough to overflow the weights of the Counter's pick.
None of these values means anything for the game.

## Decision

Config validation rejects a numeric key above an upper bound, with a message naming the key and the
bound. The bounds sit far above any sensible setting, and the code that checks them is the single
place that holds the numbers.

| Keys | Upper bound |
|------|------------:|
| `coin_rate` | 1,000 |
| `counter_sell_pct`, `counter_buy_pct` | 10,000 |
| `welcome_bonus`, `daily_reward`, `counter_budget.initial`, `counter_budget.daily_float`, `counter_supply.coins_per_energy` | 1,000,000,000,000 coins |
| `counter_supply.boosters_per_day` | 100,000 |
| `energy.recharge_per_tick`, `counter_supply.energy.recharge_per_tick` | 1,000 |
| `energy.reserve_cap`, `counter_supply.energy.reserve_cap` | 10,000 |
| `energy.savings_cap`, `counter_supply.energy.savings_cap` | 100,000 |
| `energy.cost_table[i].cost` | 10,000 |

`coin_rate` and the percentages follow the bound the price conversion already documents. The amounts
of coins leave room for a year of them far below the int64 limit.

## Consequences

- A config that used a value above a bound is now rejected. No known config does.
- A bound is raised by changing one constant and this ADR's successor, never by amending this one.
- A key that is not in the table has no upper bound. `fallback_price` and the NPC settings are the
  ones left out, and a later stress test can add them.
