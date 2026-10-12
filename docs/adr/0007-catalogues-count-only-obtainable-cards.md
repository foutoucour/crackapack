# 0007 – Catalogues count only cards a booster can contain

## Status

Accepted — 2026-10-12

## Context

An MTGJSON set file lists cards that no booster holds, for example the collector-numbered promos of
Guildpact (`14★`, `37★`). The simulation counted them in the set's totals, so the milestone "every
common and uncommon is owned" could never be reached, and the catalogue-wide metrics included cards
a player cannot get.

## Decision

Loading an openable set keeps only the cards that appear in at least one booster sheet of its
`draft` booster. Supplementary pools have no sheets and are kept whole: their cards are obtainable
through the main set's sheets, as with the timeshifted cards of Time Spiral.

## Consequences

- Set totals, milestones and the catalogue-wide metrics describe cards a player can get.
- A card that is missing from every sheet of a set file is invisible to the simulation, even if it
  is a real printing. Check the sheets in the MTGJSON file when a card seems to be missing.
