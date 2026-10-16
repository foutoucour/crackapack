# 0038 – The game's energy recharges by the clock, and the welcome boosters are free

## Status

Accepted — 2026-10-16

## Context

[Energy](../core-features.md#energy) is the only resource for opening boosters: +1 every ten
minutes up to a reserve, savings once it is full, and a cost that follows the booster's value
against the median of the catalogue. The simulation implements all of it on a tick that stands for
ten minutes. The interactive game had no energy, and the welcome boosters, which the docs promise
as "200 coins and 10 boosters", were its only way to open a booster.

Two things had to be decided that the simulation never faced: what a tick is when a person plays,
and whether a booster given as a welcome gift costs energy.

## Decision

The session recharges the energy from an injected clock. Each time it is asked something, it applies
the ticks that have gone by since the last time, one for every ten minutes, and keeps the part of a
tick that has not yet gone by. The energy keeps recharging while the page is closed, and it is the
same `domain.Energy` the simulation uses, so the rules are the same.

What a booster costs in energy is `domain.BoosterEnergyCosts`, which the simulation now calls as
well: the booster's average value against the median of every set in the catalogue directory, mapped
through the cost table. Without a price snapshot no set is worth more than another, and every
booster costs the middle tier.

The ten welcome boosters cost no energy and are opened first. Once a set has none left, a booster
costs that set's energy. An opening that the energy cannot pay for is refused before anything
happens, and says how long to wait.

The game has no coins yet, so it charges no coin fee and offers no coin top-up. The sets of
2005 to 2007 pay no fee, see
[ADR-0019](0019-the-player-pays-a-coin-fee-on-boosters-above-the-energy-cap.md), but a configuration
that prices a set above the cap would not be honoured.

## Consequences

- A fresh session starts with a full reserve and ten free boosters. After them, two boosters of the
  middle tier empty the reserve, and the next one is some fifty minutes away. The demo behaves the
  same way, since a shortcut would hide what the energy does.
- The session is in memory, so a restart gives back a full reserve and the welcome boosters. The
  energy will be saved with the rest when the store exists.
- The clock is a constructor parameter. Only the composition root reads the real one, and a test
  moves a fake by hand.
- The page shows the energy as it was when it loaded: it does not update by itself. A booster that
  became affordable while the page was open shows as such after a reload.
- Welcome boosters being free is a reading of the docs and not something they say. Making them cost
  energy is one condition in `Game.Open`.
