# 0063 – The NPCs of the game open the sets the player can open

## Status

Proposed — 2026-10-23

## Context

The game plays every set of the product's scope, unlocked in stages, see
[ADR-0054](0054-a-binder-past-the-threshold-unlocks-random-sets-of-the-next-stage-once.md). Its NPCs
opened any set at random, locked ones included, so that the stock they sell the Counter would not
depend on how far the player got.

An NPC trades only the duplicates on its shelf, see
[ADR-0011](0011-npcs-hold-surplus-on-a-shelf.md), and a duplicate of a card takes many boosters of
its set. With the game's crowd spread over the whole scope, a measure on the real catalogues found
no NPC holding any card of the first set in the first days, and few after weeks. Asking around for a
card would answer "nobody has it" almost always, and the market would not work.

## Decision

In the game, an NPC with no focus set opens only the sets the player can open. When a binder unlocks
sets, the NPCs open them from then on. An NPC with a focus set keeps opening it, whether the player
can open it or not.

## Consequences

- The NPCs hold the cards the player looks for: on the real catalogues, most cards of the first stage
  are on some NPC's shelf within days, see the [tuning log](../tuning-log.md).
- The surplus the NPCs sell the Counter comes from the sets the player can open, so its stock of
  those sets grows faster than that of a locked set, which only its own boosters stock. The scarcity
  of the Counter's rares was never judged with it.
- The simulation is unchanged: its NPCs open every set loaded.
