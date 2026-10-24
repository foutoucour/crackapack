# 0071 – Quests count the change of the collection and of the ledger

## Status

Accepted — 2026-10-24

## Context

A quest asks for cards sold, cards bought, coins spent or won, and a deal on the market. Those things
happen in several places: the Counter, the market, and the NPCs' replies that are delivered later,
when the game is looked at again. A call to count each of them from each place would leave one out,
and a card sold by a path added later would count for nothing until someone remembered it.

## Decision

The game counts what changed since the last count, whichever path it came by.

- A card that is in the collection and was not at the last count is bought, and one that was and is
  gone is sold. A trade counts as both, one card for each side.
- Coins that entered the player's account in the ledger are won, and coins that left it are spent. A
  payment from the market's account is also a deal.
- The welcome bonus and the daily reward are paid for being there, not for playing, and are not
  coins won.
- A booster opened and a reward given are neither a purchase nor a sale: the game moves the point
  of comparison past them, so that their cards count for nothing.
- Every save counts first, since every change of the game ends with one, and so does looking at the
  quests.

## Consequences

- A sale at the Counter and an NPC's reply that arrives while the game is closed count the same way,
  with no hook in either.
- A card sold and bought back between two counts counts for nothing, since only the difference is
  seen. Each action of the player ends with a count, so this only happens to what the market
  delivers together.
- A loaded game starts counting from the collection and the ledger of its save, so reloading is
  never a purchase.
- What the NPCs delivered while the game was closed counts for the quests of the day the game is
  looked at again, since the quests are renewed before they are counted.
