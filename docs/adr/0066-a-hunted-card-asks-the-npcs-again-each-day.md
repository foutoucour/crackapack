# 0066 – A hunted card asks the NPCs again each day

## Status

Proposed — 2026-10-24

## Context

On the market the player asks around for a card once, see
[ADR-0058](0058-the-player-trades-on-a-market-of-requests-and-listings.md). The NPCs that hold it
answer, and when those answers are over the player has to remember to ask again. A collector hunting
a rare wants the opposite: to say once which cards it wants, and to find the offers waiting when
it comes back.

## Decision

The player hunts up to 20 cards, the size of the wish list in
[Collection and binders](../core-features.md#collection-and-binders). A hunted card is a standing
request on the market.

- Hunting a card asks around for it at once, with the market's own rules and delays.
- Once the answers to a request are over, the hunt asks again, at most once a day. Asking again
  daily keeps the NPCs from offering the same card each time the player opens a page, and lets a
  card that reached an NPC's shelf since the last request be found.
- The hunt ends by itself once the player holds the card, however it came.
- A hunt is kept in the save, with when it last asked, so that a reload neither asks again nor
  forgets.

## Consequences

- The hunt adds no trade rule: its offers are the market's answers, accepted, declined or countered
  on the Market page.
- The hunt asks when the game is looked at, as the market delivers answers, not while the game is
  closed. A player away for a week finds one request, not seven.
- The hunted cards are on the `/hunted` page, and an empty slot of a binder has a Hunt button.
