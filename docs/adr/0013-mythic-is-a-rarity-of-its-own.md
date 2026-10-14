# 0013 – Mythic is a rarity of its own

## Status

Accepted — 2026-10-13

## Context

The V1 catalogue is the sets of 2005-2007, which have no mythics, so the loader knew four rarities
and rejected any other with an error naming the file. The core sets of 2009 and later, Magic 2010 to
Magic 2015 among them, print `mythic` on their top rares. A catalogue directory holding one of them
stopped every run, even one that did not ask for that set, see
[Catalogue directory](../simulation.md#catalogue-directory).

## Decision

`mythic` is a rarity of its own, next to common, uncommon, rare and special. It is not folded into
rare, so prices, stock and NPC wishes can tell the two apart.

- **Reveal order:** a mythic ranks with a rare. They fill the same booster slot, so a foil mythic is
  never shown before a non-foil rare.
- **Counter stock:** `initial_stock` takes a `mythic` key. Its default is 2, the same as rare, and
  it is a starting guess like the other defaults.
- **NPC wishes:** the `whale` wishes for mythics along with rares and specials, and the
  `targeted_collector` ranks a mythic before a rare.
- **Output:** `days.csv` has a `stock_mythic` column, after `stock_special`, in every run.

## Consequences

- A run on V1 sets gives the same `result.json` as before and the same `days.csv` apart from the
  zero `stock_mythic` column. `compare` reads `result.json`, so it still compares old and new runs.
- A switch over the rarities must handle mythic. The `exhaustive` linter reports the ones that do
  not.
- The booster layouts of the core sets after 2009 may hold other features the loader does not know.
  Each is a separate decision, made when a run on that set fails.
