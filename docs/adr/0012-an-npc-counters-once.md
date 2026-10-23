# 0012 – An NPC counters once

## Status

Superseded by [ADR-0058](0058-the-player-trades-on-a-market-of-requests-and-listings.md) — 2026-10-23.
Accepted 2026-10-13.

## Context

[Trades with NPCs](../core-features.md#trades-with-npcs) says a counter-offer is possible on both
sides, and does not say how far a negotiation goes. An open-ended exchange needs a negotiation state
and a strategy for what each side asks next, and makes a run harder to reproduce and to tune.

## Decision

When a player's offer falls short of the acceptance rule, the NPC answers once with the same cards
and the smallest top-up that satisfies it. The counter is an open offer the player accepts or
ignores. When an NPC proposes, the player may counter once with different cards, and the NPC then
accepts or refuses without countering again.

## Consequences

- A trade is at most three messages, and no negotiation state exists beyond the open offers.
- A run stays deterministic from its seed.
- A multi-round negotiation would be a new ADR that supersedes this one.
