# 0077 – The market shows what the NPCs hold on their shelves

## Status

Proposed — 2026-10-26. Supersedes the part of
[ADR-0058](0058-the-player-trades-on-a-market-of-requests-and-listings.md) that lets the player see
what an NPC holds only through its answer to a request or a listing.

## Context

[ADR-0058](0058-the-player-trades-on-a-market-of-requests-and-listings.md) makes the player ask
around for a card and read the answers. The market shows who answered and their price, never what an
NPC holds beyond its answer. A player who has no request and no listing opens the Market page and
finds nothing to look at, and cannot find a card they would be interested in without picking the
card first, from a binder slot.

The Counter's stock is another matter: it is meant to feel scarce, see
[ADR-0056](0056-the-counter-remembers-when-it-last-had-a-card.md), and this ADR leaves it hidden.

## Decision

The player sees every card that an NPC that trades holds on its shelf, with who holds it and the
price it asks. The price is the one an answer to a request would carry: the card's reference value
plus the NPC's margin.

The player acts on that price without asking around first:

- **Buy now** takes one NPC's asking price and trades at once.
- **Make an offer** names a lower price to one NPC, which answers once after the delay of its
  archetype, by the acceptance rule of a counter, see
  [ADR-0059](0059-an-npc-answers-on-the-market-after-the-delay-of-its-archetype.md).

## Consequences

- The shelves are public: the player can plan a purchase from them. The Counter's stock, an NPC's
  wish list and the cards an NPC keeps stay hidden.
- Buy now and an offer use the price and the acceptance rule of an answer, so the two cannot
  disagree on what a card is worth. The market's invariants hold for both: a trade never raises what
  the player owns, the ledger sums to zero, and cards neither appear nor vanish.
- A card can leave a shelf between the page and the click. Buy now then fails with the error of an
  answer whose NPC no longer has its card, and changes nothing.
- Ask around stays. It reaches the NPCs that will hold the card later, and the hunt builds on it, see
  [ADR-0066](0066-a-hunted-card-asks-the-npcs-again-each-day.md).
