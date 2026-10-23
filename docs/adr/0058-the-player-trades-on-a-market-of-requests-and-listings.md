# 0058 – The player trades on a market of requests and listings

## Status

Proposed — 2026-10-23. Supersedes [ADR-0012](0012-an-npc-counters-once.md).

## Context

The simulation's trades start from a precise offer. An NPC proposes a swap when its wish list
matches the player's duplicates, or the player proposes cards and coins to one NPC it chose. The
answer comes at once. [ADR-0012](0012-an-npc-counters-once.md) bounds the talk to three messages and
says that no negotiation state exists beyond the open offers.

The game's player trades the way people do on Reverb or eBay, as the maintainers put it:

- **Hunt:** "I browse the binder and see a card. I ask if anyone has the card and the price he is
  asking for. I can counter the offer with coins or with cards."
- **Sell:** "I browse the binder and see a card. I can put it on the trade market for a price. I have
  offers in coins or in cards from NPCs."

The player does not pick an NPC, several NPCs answer one question, and they answer over time. That
is a second way to trade, started by the player, with state that lives between the messages. It
breaks ADR-0012's "no negotiation state", and it does not fit its rule that a short player offer is
countered by the NPC.

## Decision

In the game, the player trades on a market.

- **A request** asks around for a card. Every NPC that trades and holds the card on its shelf answers
  with its asking price: the card's reference value plus its margin, in coins, the price at which the
  Exchange's acceptance rule takes a coin offer.
- **A listing** puts one copy of a card the player holds on the market at an asking price, and reserves
  it until it is sold or unlisted. The NPCs that wish for the card, and the speculators that lack it,
  make an offer. It is cards from the NPC's shelf that the player lacks, worth no more than the card
  once the NPC's margin is paid. When the NPC has none, it offers coins, see
  [ADR-0060](0060-an-npc-pays-coins-for-a-card-on-the-market.md).
- **An answer** is the NPC's terms, as an NPC's own proposal is in the Exchange. The player accepts it,
  which trades at once and ends the request or the listing; declines it; or counters it once. On an
  asking price the counter offers coins and cards. On an offer it asks for coins. The NPC then accepts
  or refuses by the acceptance rule and its traits' vetoes, without countering again. A refused
  counter ends the answer.
- **Answers lapse** after the config's offer expiry, unless the player countered them.

The negotiation keeps ADR-0012's depth: the NPC's terms, one counter, a final yes or no. What
changes is that the market holds state between the messages: requests, listings, answers to come, and
counters waiting for a reply. When an answer comes is decided by
[ADR-0059](0059-an-npc-answers-on-the-market-after-the-delay-of-its-archetype.md).

The simulation's Exchange is unchanged. A player proposal there is still countered once with the
smallest top-up, and an NPC proposal can be countered once. The market reuses its acceptance rule,
its margins, its traits and its atomic transfer, so the two cannot disagree on what a card is worth.

## Consequences

- A trade never raises what the player owns, coins plus the reference value of the cards, as in the
  Exchange. A property test runs random market moves against that, against a ledger that must sum to
  zero, and against cards that must neither appear nor vanish.
- Declines follow [ADR-0024](0024-a-decline-is-told-to-the-npcs-traits-and-counted.md): the player
  declining an answer, or the NPC refusing a counter on terms, is a decline of the NPC's answer. A
  lapse is not.
- A listed card, and the cards of a counter waiting for its reply, are held for the market. The
  Counter does not buy them meanwhile.
- The market's state is saved with the game, so a reload changes no answer.
- ADR-0012 stays as written, apart from its status. Its rules still describe the Exchange.
