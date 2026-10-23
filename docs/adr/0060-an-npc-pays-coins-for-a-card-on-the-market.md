# 0060 – An NPC pays coins for a card on the market

## Status

Proposed — 2026-10-23

## Context

The specs say NPCs hold no coins: without auctions they have nothing to spend on, and budgets would be
machinery with no effect, see [Specs review](../specs-review.md#positioning-and-decisions). Trades
are card swaps, and coins only ever come from the player, through the `trade_topup` account.

The market's sell flow wants offers in coins as well as in cards, see
[ADR-0058](0058-the-player-trades-on-a-market-of-requests-and-listings.md). An NPC that pays coins
needs them from somewhere. Three sources were weighed:

- **What the NPCs earned from the player** on the market. Nothing is created. A new game has no such
  coins, so a listing would draw no coin offer until the player had bought from NPCs.
- **A purse per NPC.** It would be the budget the specs rejected.
- **The market itself,** as the Counter pays a sale: the coins enter the economy, against a card
  that leaves the player.

## Decision

An NPC pays coins from the market's account, `market_sale`, a source like the Counter's
`counter_sale`. NPCs still hold no coins.

An NPC never pays more than the card is worth to it: its reference value less the NPC's margin,
rounded down. That is the acceptance rule read the other way round. An offer is also never above the
player's asking price. The player's own coins still go to `trade_topup`, as in the Exchange.

## Consequences

- No trade raises what the player owns, coins plus the reference value of the cards. A coin offer
  turns a card into at most its value, and the property test of the market checks it.
- The market is a new source of coins, and a better one than the Counter for a card an NPC wants: the
  Counter pays a fixed share of the value, an NPC up to the value less its margin. The coin goals of
  [ADR-0041](0041-a-finished-collector-keeps-at-most-3000-coins.md) were never judged with it, and the
  simulation does not run the market. The speculators' margins and share are the levers.
- `market_sale`'s balance is the total NPCs paid, so the coins the market issued can be read.
