# 0028 – A card on a foil sheet loads with a foil finish

## Status

Accepted — 2026-10-16

## Context

MTGJSON lists Tamiyo's Journal of Shadows over Innistrad (`b33e380c…`) as non-foil only, yet the
`foil` sheet of the SOI booster holds it. The booster draw gives every card of a `foil: true`
sheet the foil finish, so the engine dealt a foil copy of a card whose catalogue entry has no foil
finish. The Counter refuses such a copy with "card not in catalogue",
and the run stopped as soon as the Counter's supply or an NPC drew it.

In the 2026-10-15 studies this stopped about one run in six on 105 sets. A scan of the 146 files in
`catalogues/` finds this one card: no other foil-sheet card lacks a foil or etched finish, in its own
file or in a supplementary pool.

Two fixes were open:

- Fail the load, as [ADR-0016](0016-a-run-fails-when-a-sheet-card-has-no-catalogue.md) does for a
  sheet card that no catalogue has.
- Load the card with a foil finish.

## Decision

`LoadSetCatalogue` adds `FinishFoil` to every card that a `foil: true` sheet holds and whose
finishes lack it. The sheet is the authority on what a booster can contain, as in
[ADR-0007](0007-catalogues-count-only-obtainable-cards.md), so a card the sheet deals as a foil is
obtainable as one.

Failing the load was rejected. ADR-0016 fails because a missing card has no entry and no price. This
card has both, and a foil shares its card's price. Failing would make SOI unusable for one data quirk
of an upstream file, and the studies would lose a set.

## Consequences

- SOI runs complete with the Counter's supply and with NPCs. A 60-day run on SOI with the Counter
  opening 100 boosters a day stopped on day 2 before and finishes now.
- The loader overrides MTGJSON's finishes for these cards. If MTGJSON corrects the file, the card
  already lists a foil and nothing changes.
- The card gains a foil to collect: the `master-collector` now counts it, and a trait that scores
  foils sees it.
- A foil-sheet card that lives in a supplementary pool is not touched, because pools have no sheets.
  None exists in the catalogues today.
