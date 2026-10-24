# 0069 – The player asks for a coin top-up, and the game plays one price

## Status

Proposed — 2026-10-24

## Context

The simulation lets a player pay coins for the energy a booster lacks, at `energy.coins_per_energy` a
point, with at least `min_energy_units` of real energy, see
[Energy coin top-up](../simulation.md#energy-coin-top-up). A policy asks for it by opening with
`OpenBoosterTopUp`. The variant `escalating` raises the price for each point bought within a day.

A player at a screen has no policy, and a booster that spends coins without being asked to would
cost coins for a click meant to open one with energy.

## Decision

The game spends coins on energy only when the player asks. `Open` refuses a booster the energy cannot
pay for, as before. `OpenWithTopUp` takes the energy the player has, at least `min_energy_units`,
and charges the rest at the price of a point. The home page and the booster page offer it as a second
button, with its cost, while the player is short of energy and has the least energy it takes.

The game plays the `min_energy` variant, one price for every point. A config with the `escalating` or
`both` variant is refused when the game starts.

## Consequences

- No coin leaves the player for energy without a click that names the price.
- The top-up adds to the set's fee, see
  [ADR-0068](0068-the-game-charges-a-sets-coin-fee-when-a-booster-is-opened.md), and the button shows
  both together.
- The escalating variants need the number of points bought today, which the game does not keep. Playing
  them is a new decision that supersedes this one and saves that count.
- A welcome booster is free, so it has no top-up.
