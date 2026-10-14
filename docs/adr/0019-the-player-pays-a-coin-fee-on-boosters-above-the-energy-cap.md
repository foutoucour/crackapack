# 0019 – The player pays a coin fee on boosters above the energy cap

## Status

Accepted — 2026-10-14

## Context

A booster's energy cost stops at the dearest tier of the cost table, 12 energy by default, while
booster values vary widely. Past the cap the coins a booster pays per energy grow with its value:
about ×3 at $70 and ×65 at $1,080, against the 2005-2007 sets, see
[Expensive old sets and the 12-energy cap](../specs-review.md#expensive-old-sets-and-the-12-energy-cap).
Longer tier tables cannot close that gap, since the reserve and savings cannot hold the energy it
would take.

The Counter already has such a rule: its treasury pays a fee on the boosters it opens above the
cap, see [The coin top-up and the treasury](../simulation.md#the-coin-top-up-and-the-treasury). The
player had none.

## Decision

`energy.coins_per_energy`, the price `p` of one energy point, charges the player a coin fee on a
booster worth more than the cap times `p`. The fee is the booster's cash-out value, what the Counter
would pay for its cards, minus the cap times `p`. It is paid on top of the cap's energy and leaves
the economy into the `booster_fee` account.

The player and the Counter share one fee function, so a set costs both the same at the same `p`. The
default 0 charges nothing. Energy itself is still never bought: the fee prices a dear booster, it
does not convert coins into energy.

## Consequences

- The 2005-2007 sets pay nothing at a `p` of 100, so the rule leaves the sets of the first version
  alone and only bites on the dear old sets.
- A coin sink exists for the player without any limit design, since the fee has nothing to convert.
  Optional conversion of energy into coins, and the continuous price `V / p`, stay open in the
  [TODO](../todo.md).
- `p` is a price in coins and a booster's value scales with `coin_rate`, so the two have to move
  together, see [The coin fee on dear boosters](../simulation.md#the-coin-fee-on-dear-boosters).
- A policy that cannot pay a fee must choose another set, or it waits for ever. The `collector` and
  `cash-out` policies do. A policy written later has to do the same.
- Changing the rule, for example to scale the fee with the rarity of a set, is a new ADR that
  supersedes this one.
