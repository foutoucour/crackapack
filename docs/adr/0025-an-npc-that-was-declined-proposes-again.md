# 0025 – An NPC that was declined proposes again

## Status

Accepted — 2026-10-15

## Context

[ADR-0024](0024-a-decline-is-told-to-the-npcs-traits-and-counted.md) tells a trait about a decline
and gives it the NPC and its decline streak. It also forbids a trait from holding per-NPC state,
because one trait serves every NPC of an archetype.

Two personalities act on a decline in a way that reaches `ProposeFromNPCs`: the ego proposes again,
and the gambler proposes more often. Both effects depend on the NPC, so they cannot live in the
trait. [ADR-0012](0012-an-npc-counters-once.md) says an NPC counters once and that no negotiation
state exists beyond the open offers.

## Decision

A reactor records what it needs on the NPC, and `ProposeFromNPCs` reads it there.

The ego proposes again after the player declines one of its own proposals. That is an offer of origin
`OriginNPC`, including one the player countered and the ego refused. A decline of one of the NPC's
counter-offers, `OriginNPCCounter`, does not arm it.
- A follow-up skips the offer-chance roll. It still counts against `max_open_offers_per_day`.
- At most `max_retries` follow-ups are made per day. After that the ego holds for the rest of the day.
- A follow-up never gives a card of an offer the player declined in the same chain, so it is a
  different trade. When no such trade exists the ego holds for the rest of the day.
- A chain that crosses midnight gets a fresh budget on the new day.
- An accepted trade clears the state, because it resets the decline streak.

The gambler raises its offer chance by `chance_step_pct` for each decline in a row, up to
`max_offer_chance_pct`, and never below its base chance. An accepted trade ends the tilt for the same
reason.

## Consequences

- An ego makes at most `max_retries + 1` offers per day. An NPC with an open offer waits, so offers
  do not pile up.
- The state is the NPC's mood, not a negotiation: it holds a budget and the cards already refused,
  never terms. Each follow-up is a new proposal that is answered under ADR-0012's rules, so that ADR
  stands.
- No random draw is made, and a run without these traits is unchanged.
- The ego's endowment margin and the gambler's falling margin are [ADR-0021](0021-an-archetype-names-a-base-behaviour-and-traits.md)
  adders, so they price a player's proposal or counter, not the NPC's own proposals, which use the
  base margin.
