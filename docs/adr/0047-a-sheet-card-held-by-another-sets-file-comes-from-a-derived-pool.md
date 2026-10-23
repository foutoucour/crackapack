# 0047 – A sheet card held by another set's file comes from a derived pool

## Status

Accepted — 2026-10-22

## Context

The `prototypePraetor` sheet of Phyrexia: All Will Be One holds 5 cards. One is the set's own. The
other 4 are Phyrexian Praetor concept printings that MTGJSON lists in the files of Dominaria United,
Kamigawa: Neon Dynasty, Kaldheim and Streets of New Capenna, and in none of those sets' booster
sheets. Those four sets are openable, and an openable set keeps only the cards of its own sheets, see
[ADR-0007](0007-catalogues-count-only-obtainable-cards.md), so the 4 cards belong to no loaded
catalogue. The engine stopped ONE with the error of
[ADR-0016](0016-a-run-fails-when-a-sheet-card-has-no-catalogue.md), and passing a donor set as a
supplementary pool failed with a card in two catalogues, since a pool is kept whole. The scope of
[ADR-0039](0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md) had 106 sets
that load, not 107.

Two ways out: let a sheet draw a card from another openable set's file, or build a small pool.

## Decision

Build a small pool. `tools/fetchdata` writes it as `PRAETORS.json` next to the set files, holding the
4 borrowed cards as MTGJSON gives them, and it is passed with `--supplementary PRAETORS` like `TSB`
for Time Spiral.

- A table in `fetchdata` names each pool: the host set whose sheets borrow, the code of the pool and
  the donor sets. Only ONE is in it.
- When the host is requested, fetchdata also downloads the donors, whose cards it prices. It does not
  list the borrowed cards by hand: they are the cards on the host's draft sheets, or default ones,
  that the host file does not hold, found in the donor files.
- A borrowed card that no donor holds stops the run before any price is written, naming the host and
  the card IDs.

The loader and the engine do not change. A pool is what they already read, so the binder counts the
borrowed cards, see [ADR-0037](0037-a-binder-counts-the-pool-cards-its-boosters-draw.md), and the
Counter stocks them.

A sheet drawing straight from another set's file was left out: it would be a second mechanism next to
the pool, and the loader would have to find the donor file from a card ID alone.

## Consequences

- ONE loads, and the scope of ADR-0039 has 107 sets.
- A run that names ONE must pass `--supplementary PRAETORS`, and a scenario that names it lists
  `PRAETORS` under `supplementary`. Without it the run fails at the start with the error of ADR-0016.
- ONE's binder is its own cards plus the 4 praetors.
- A directory fetched before this change lacks `PRAETORS.json`. Running fetchdata for ONE again writes
  it.
- Another set that borrows from other files needs one more entry in the table.
- The 4 cards stay in the donor files, so the pool is a copy: a card that MTGJSON changes there is
  refreshed the next time fetchdata runs for ONE.
