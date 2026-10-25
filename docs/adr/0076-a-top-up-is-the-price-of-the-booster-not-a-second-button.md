# 0076 – A top-up is the price of the booster, not a second button

## Status

Proposed — 2026-10-25

## Context

[ADR-0069](0069-the-player-asks-for-a-coin-top-up-and-the-game-plays-one-price.md) has the home page
and the booster page offer the coin top-up as a second button next to Open. On the home page the two
buttons took most of a tile, which squeezed the name of the set, and the second one read as a separate
purchase when it is the same booster paid in energy and in coins.

## Decision

A booster has one button, and the button says the whole price: the energy the booster takes, and the
coins on top. A player short of energy, with the least it takes, sees the same button at its top-up
price, such as `4 + ◎70`, and a line under the name that says how much energy is missing and how long
to wait for it instead. The button posts the top-up, so the game still spends coins on energy only on
a click that names the price.

ADR-0069 holds for everything else: the player asks for it, the game plays one price a point, and the
top-up adds to the set's fee.

## Consequences

- The price of a booster follows the energy while the page is open, so `/energy` carries it for the
  script to show, told by the server like the waits.
- A booster that waits for energy can still be opened while a top-up pays for it, so the button is
  disabled only when none does.
