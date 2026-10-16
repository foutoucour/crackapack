# 0022 – An NPC wishes only for cards in its colours

## Status

Accepted — 2026-10-15

## Context

[ADR-0021](0021-an-archetype-names-a-base-behaviour-and-traits.md) introduces personalities that
are tied to the Magic colours and mix in one archetype. A colour personality should only wish for
cards it would plausibly play. Without a scope filter, a black NPC wishes for white cards, which
conflicts with the colour identity idea and makes calibration harder.

Guilds are colour pairs, so the colour restriction is a list, not a single value.

## Decision

A card is in an NPC's scope when all of its colours are among the NPC's colours. Colourless cards
and cards with no colour data are in scope for everyone, because an empty set is a subset of every
set.

An empty colour list and all five colours both mean no restriction: the NPC's scope is unchanged
from what it would be without the feature.

When an NPC has a focus set, the focus set is applied first; the colour filter then narrows the
result. A colour plus a focus set that matches no card leaves the NPC with nothing to wish for.

Colours are validated at config load (via `validateArchetypeColors`) and inside
`GeneratePopulation` (via `validateColors`), so an unknown or duplicate colour is caught before the
simulation starts.

## Consequences

- Scope is built once per distinct (focus set, colour set) pair and shared: all NPCs of the same
  archetype with the same focus and colours use one backing array, making the crowd cheap to build
  even at large sizes.
- A gold card needs all of its colours to be in the NPC's list: a WU NPC does not wish for a UB
  card.
- The five default archetypes carry no colours and are therefore unrestricted, keeping their
  existing behaviour.
- The demo dataset has cards in all five mono colours, at least one gold card and at least one
  non-basic colourless card, verified by `TestBuild_demoCataloguesCarryColoursForAllFiveColours`.
