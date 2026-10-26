# Market page: browse what the NPCs sell, and show where each trade stands

Issue [#61](https://github.com/foutoucour/crackapack/issues/61). The decision to show the shelves is
[ADR-0077](../../adr/0077-the-market-shows-what-the-npcs-hold-on-their-shelves.md).

## Why

The Market page is a status board for trades the player started from a binder slot. With no request
and no listing it shows three empty sentences, so the player cannot open it to find a card. After
asking an NPC to trade, the player cannot tell where the trade stands.

## Goals and non-goals

- **Goal:** the Market page is a shop window: every card an NPC holds, at the price each NPC asks.
- **Goal:** the player buys a card at once, or offers less to one NPC.
- **Goal:** every open trade says whom it waits for and for how long.
- **Non-goal:** showing the Counter's stock, the NPCs' wish lists, or any rule change to prices,
  margins, delays or expiry.
- **Anti-goal:** a page rule. The web layer shows what the game returns and calls its methods.

## Decisions from the issue

1. Two tabs, **Browse** (the landing page, `/market`) and **My trades** (`/market/trades`).
2. Browse lists every card some NPC that trades holds on its shelf, in the sets the player has
   unlocked, since the player can buy no other card, as for Ask around. Filters: hunted, missing
   (the player holds no copy and the card has a slot in an open binder), colour, rarity.
3. A card shows one price per NPC, with who it is ("Ajani Goldmane (whale)").
4. **Buy now** takes one NPC's asking price, completes at once, and the copy leaves the shelf.
5. **Make an offer** names a lower price to one NPC, which answers once after its archetype's delay.
6. My trades is grouped by who is waiting: *Needs you*, *Waiting on an NPC*, *Done*. Each item says
   what it waits for and for how long.

## Guesses of the issue, taken as decided

The issue asked to confirm these; the session ran under a goal that does not pause, so they stand,
and each is one line to reverse.

- The grid tile shows the lowest price and the number of NPCs holding the card. A click opens the
  card's page, which lists the NPCs.
- Browse has a set filter and a name search besides the four above.
- Ask around stays.
- The Counter's stock stays hidden.

## Design

### Domain (`domain.Market`)

- `ForSale()` lists, per card an NPC that trades holds, the NPCs holding it with their asking price
  and copies, cheapest first.
- `BuyNow(npc, card, now)` posts the asking price, moves the card and records an accepted result.
  It fails with `ErrNPCLacksCard` when the shelf has no copy, and with `ErrInsufficientBalance`,
  and changes nothing.
- `Offer(npc, card, coins, now)` is a counter without the ask-around step: it opens a *direct*
  request to the one NPC with its asking price already answered and the player's counter on it. The
  existing reply, settle and expiry code does the rest. A direct request ends with its answer.
- The asking price is one function, used by the answer to a request, by `ForSale` and by `BuyNow`.
- A direct request is one more flag in the saved request, not a new state.

### Game (`play.Game`)

- `ForSale()` returns the cards with the sellers' names and archetypes, the card's set, whether it
  is hunted, and whether it is missing.
- `BuyNow(npc, id, finish)` and `OfferTo(npc, id, finish, coins)` wrap the market actions as the
  others do: same errors, `ErrGone` for a card that left the shelf, and a save after.
- `MarketView` gains the grouping: each request and listing says whether it needs the player or
  waits on an NPC, and the instant of the next answer or reply.

### Web (`internal/web`)

- `GET /market` Browse, with the filters as query fields and the empty, error and narrow-screen
  cases. `GET /market/card` is a card's page with one row per NPC: price, a Buy now form and an
  offer form. `POST /buy-now` and `POST /offer` come back to that page with a notice.
- `GET /market/trades` is today's content, grouped. Its forms come back to it.
- The header link stays on `/market`. The My trades tab carries the number of answers waiting.

### Demo

`crackapack play --demo` already gives the NPCs shelves; its test checks Browse is not empty and a
buy and an offer go through the pages.

## Left to the builder (my calls)

- Cards of locked sets are not listed, as Ask around refuses them.
- An offer is refused with the errors of a counter on an offer; it cannot start while the player
  already waits on that card (an open request), which `AskAround` also refuses.
- Prices show in coins with the existing `coins` helper; no new styling beyond a grid of tiles.
