# 0010 – NPC trade behaviour is a strategy

## Status

Accepted — 2026-10-13. Supersedes [ADR-0009](0009-npc-archetypes-are-data.md).

## Context

[ADR-0009](0009-npc-archetypes-are-data.md) made archetypes data, because supply differs between
them by three numbers only. It expected a strategy to appear with trades, where behaviour may not
fit parameters.

It does not. The five archetypes pick what they wish for in different ways: a casual at random, a
playset hunter among commons and uncommons it holds fewer than four of, a targeted collector from
the rarest down, a speculator by value, a whale among rares. They also differ in what they ask of
the player. A parameter cannot say that.

## Decision

An archetype names a registered `TradeBehaviour` and carries the numbers it reads: `behaviour`,
`wish_size`, `margin_pct` and `offer_chance_pct`. The behaviour picks wishes, gives the margin and
builds a proposal.

Supply stays as ADR-0009 decided: `min_boosters_per_day`, `max_boosters_per_day`, `keep_copies`
and `focus_set` are data read by one step. A new personality is a type in `internal/domain`, one
registry entry and a config entry.

## Consequences

- The five default archetypes name the five behaviours of the same name, and a config can define
  others that reuse a registered behaviour with other numbers.
- Config rejects an unknown behaviour, and requires one for every archetype with a share when
  trades are enabled. A behaviour reads only config numbers, so a scenario is still a config file.
- A negative margin is rejected twice, at config load and when a behaviour is built, so an NPC
  never gives more value than it gets.
- ADR-0009 stays as written, apart from its status.
