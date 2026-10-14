# 0018 – Cards sold to the Counter can vanish

## Status

Accepted — 2026-10-14

## Context

The Counter used to put every card sold to it, by the player or as NPC surplus, back into its stock.
With a crowd of 300 NPCs it ended a 120-day run holding millions of cards, so "limited stock" limited
nothing: a player could buy any card at any time. The simulations then showed that coins never
bound progression, since energy was the throttle and the Counter never ran out of anything. A
brainstorm on making coins matter chose to make the Counter's stock scarce, and to do it with the
fate of the cards it receives, see [The Counter's shelf life](../simulation.md#the-counters-shelf-life).

## Decision

`counter_shelf_days` sets how long a card on the Counter's shelf stays there.
- `-1`, the default, keeps it forever. A run with this value is the same as before the setting.
- `0` makes it vanish at once, so the Counter is a sink.
- `N` of 1 or more makes it vanish N days after it arrived.

The setting covers every card that reaches the shelf after the start: cards the player sells, NPC
surplus and, in a run where the Counter opens boosters, the cards it opens. Initial stock never
vanishes, since it is not a sold card. A purchase takes the oldest copy on the shelf first, so the
copies about to vanish are not wasted. The player is still paid for a sale, so coins are still
created.

## Consequences

- At a steady state the stock is the daily inflow times the shelf days, which makes the shelf life
  and the crowd or supply that feeds it one dial, not two.
- A shelf life of `0` starves the player at every supply level, as the stress test showed: the
  player buys nothing, even with 250 boosters a day or 300 NPCs. From `1` up it barely moves the
  pace and only sets the stock level. It is a cliff at `0`, not a gradual dial.
- A sale is permanent, and no player policy suffers from it. The collector, the trader and the
  exploiter keep a playset and sell only above it, and the trader and the exploiter trade a
  duplicate before they sell it. The cash-out player sells everything above one copy on purpose.
- With a treasury, a Counter that cannot pay refuses a sale and the card stays with the player, so
  a sale that was refused is never lost.
- Changing the rule, for example making the shelf life depend on the card's rarity, is a new ADR
  that supersedes this one.
