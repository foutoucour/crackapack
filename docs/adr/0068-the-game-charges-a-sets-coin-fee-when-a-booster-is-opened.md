# 0068 – The game charges a set's coin fee when a booster is opened

## Status

Proposed — 2026-10-24

## Context

[ADR-0019](0019-the-player-pays-a-coin-fee-on-boosters-above-the-energy-cap.md) makes the player pay
a coin fee on a booster worth more than the energy cap, so that a dear set cannot be farmed for the
price of the cap's energy. The simulation charges it. The game charged energy only, and its sets
worth more than the cap were as cheap to open as the others.

## Decision

The game charges the fee of ADR-0019 when the player opens a booster that is not a welcome one. The
fee of each set is computed once, when the game starts, by the function the simulation uses, from the
set's average booster value and `energy.coins_per_energy`.

The fee is paid on top of the energy, from the player's coins into the `booster_fee` account. A player
who cannot pay it is refused, and loses neither energy nor coins.

## Consequences

- A welcome booster stays free, fee included.
- The game and the simulation charge a set the same at the same price of energy, and the price is the
  `energy.coins_per_energy` of the config the game plays, see
  [ADR-0057](0057-the-game-plays-the-products-base-config.md).
- A set the player cannot afford stays on the home page with its fee in the open button.
- The coin top-up of [ADR-0069](0069-the-player-asks-for-a-coin-top-up-and-the-game-plays-one-price.md)
  adds to the fee and never replaces it.
- A game without a Counter has no coins, and so cannot pay a fee. The game of `crackapack play` always
  has one.
- The Counter's own fee, `counter_supply.coins_per_energy`, is not played yet, and the game refuses a
  config that sets it.
