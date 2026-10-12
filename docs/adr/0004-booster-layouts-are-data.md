# 0004 – Booster layouts are data

## Status

Accepted — 2026-10-11

## Context

MTGJSON describes each set's booster as a map of named sheets (with weighted card lists) and a
list of booster options (weighted picks over sheet slot counts). This is a data model, not code.

The crackapack-conventions skill sketched one `BoosterGenerator` implementation per layout era:
a `classicGenerator` for Ravnica-block packs and a `timeSpiralGenerator` for the timeshifted
slot. When the Time Spiral and Planar Chaos booster structures were implemented, both turned out
to be expressible as data — the timeshifted slot is a sheet named `timeshifted` in TSP's
catalogue, and the colourshifted slot in Planar Chaos is a sheet named `colorshifted` with
different rarity weights. No generator type was needed beyond the one already in place.

## Decision

One generator (`classicGenerator`) interprets the sheets and booster options defined in the
catalogue file. Adding a new booster layout means defining new sheets and booster options in
the catalogue data; a new generator type is only warranted when the data model genuinely cannot
express the layout.

## Consequences

- Adding the remaining V1 sets (Guildpact, Dissension, Coldsnap, Future Sight) requires only
  catalogue data, not new generator code.
- The approximations the current generator makes are accepted for now:
  - Colour balance uses the `BalanceColors` sheet flag; the exact per-card collation rules used
    by WotC are not reproduced.
  - No `allowDuplicates` flag: duplicate draws across sheets in the same pack are allowed.
  - Two-foil Ravnica packs (booster option weight 9) are assumed to draw both foils from the
    common-foil pool; the real per-foil rarity breakdown for that option is undocumented.
- The `crackapack-conventions` skill no longer lists booster layout as a strategy: it points here
  and keeps the `BoosterGenerator` and `Opener` shapes.
