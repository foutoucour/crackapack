# 0024 – A decline is told to the NPC's traits and counted

## Status

Accepted — 2026-10-15

## Context

[ADR-0021](0021-an-archetype-names-a-base-behaviour-and-traits.md) says that every trait that
reacts to a declined offer is told about it, and each reacts on its own. The ego and gambler
personalities (coming in a later story) need two things to act on a decline: the NPC itself, to
read its state, and a running count of how many times the player has turned that NPC down in a row
without a trade in between.

[ADR-0012](0012-an-npc-counters-once.md) says an NPC answers a short player offer with one counter,
and that when the player counters an NPC's proposal, the NPC accepts or refuses without countering
again. Those are the places where an offer can be turned down.

## Decision

A per-NPC integer called the decline streak is incremented each time the player turns down an NPC
offer, and reset to zero on any accepted trade. The streak is read through `NPC.DeclineStreak()`;
there is no setter.

What counts as a decline and increments the streak:

- The player calls `Decline` on an open offer whose origin is `OriginNPC` or `OriginNPCCounter`.
- The player calls `Counter` on an NPC offer and the NPC refuses on terms: `ErrBelowMargin`
  (the player's counter is worth less than the NPC's margin requires) or `ErrCounterVetoed` (a
  trait vetoed the counter). A counter refused for any other reason — `ErrPlayerLacksCard`,
  `ErrInvalidLine`, `ErrNPCLacksCard`, `ErrInsufficientBalance` — does not count; these are
  player mistakes or stale state, not a deliberate refusal of the NPC's terms. In both cases the
  player has turned down an offer of the NPC.

What resets the streak to zero: any accepted trade, via the single booking point `accepted()`.

What does not count and leaves the streak unchanged:

- Expiry in `StartDay`: the player never answered, so nothing was turned down. An NPC whose
  offer the player ignored does not have reason to tilt.
- A failed `Accept` returning `ErrInsufficientBalance`: the offer stays open and no decision was
  made.
- A player counter the NPC refuses for a player-error reason (see above).
- An NPC proposing via `ProposeFromNPCs`.

The increment happens first, then the notification: when `ReactToDecline` is called,
`ctx.NPC.DeclineStreak()` already reflects the current decline. Reactors must not call back into
the Exchange (not reentrant) and must not hold per-NPC state, because a behaviour is shared across
all NPCs of an archetype.

## Consequences

- A shared behaviour holds no per-NPC state; the streak lives on the NPC, not on the behaviour.
- A refused player counter is a decline of the NPC's original offer: `ctx.Offer` carries the
  original `OriginNPC` offer, not the counter, so a reactor always sees the NPC's proposal.
- A run whose archetypes carry no decline-reacting trait is byte-identical to a run without this
  feature: the streak increments and resets, but no random draw is made and no output changes.
- The ego and gambler traits will implement `DeclineReactor`, see
  [ADR-0021](0021-an-archetype-names-a-base-behaviour-and-traits.md).
