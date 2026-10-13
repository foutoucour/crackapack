# 0011 – NPCs hold surplus on a shelf before selling it

## Status

Accepted — 2026-10-13

## Context

The NPC crowd sold every copy above an NPC's keep threshold to the Counter the moment it was drawn,
see [NPC crowd](../simulation.md#npc-crowd). An NPC therefore never held a duplicate, so it had
nothing to give in a trade, and "which NPCs have this card as a duplicate" would always be empty.

## Decision

With trades enabled, surplus waits on the NPC's shelf for `npcs.surplus_hold_days` days, 2 by
default, and can be traded meanwhile. At the end of each day the crowd sells to the Counter every
shelf copy that has waited the hold. Trades need a hold of at least one day.

With trades disabled the hold does not apply, and the crowd sells at once as before, so a run
without trades is unchanged.

## Consequences

- Supply to the Counter is delayed by the hold, so Counter stock differs between a run with trades
  and one without. The two do not compare, and `compare` refuses to mix them.
- `npc_cards_held` counts shelf copies, which are still held.
- The hold is a parameter a search can tune, and so is `trades.max_open_offers_per_day`.
- An NPC that receives a copy above its keep threshold in a trade puts it on the shelf.
