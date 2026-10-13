# 0009 – NPC archetypes are data

## Status

Superseded by [ADR-0010](0010-npc-trade-behaviour-is-a-strategy.md) — 2026-10-13. Accepted
2026-10-13.

## Context

[Core features](../core-features.md#npcs-and-simulation) lists five NPC archetypes, and the
`crackapack-conventions` skill sketched one `NPCPolicy` implementation per personality.

The first NPC slice only covers supply: an NPC opens boosters and sells its surplus to the Counter.
The five archetypes differ there by three parameters and nothing else: how many boosters they open
a day, how many copies of a printing they keep, and whether they open a single set. Five policy
types would have held the same code with different constants.

## Decision

An archetype is a named entry in the configuration, interpreted by one NPC step. A new personality
that fits those parameters is a new config entry, not a new type.

The behaviour that trades add, such as proposing a trade or the margin an NPC asks for, may not fit
parameters. A strategy interface is introduced when that behaviour exists and a second
implementation needs it, with a new ADR that supersedes this one if it replaces the data model.

## Consequences

- `crackapack-conventions` no longer announces an `NPCPolicy` per personality. It points here.
- The five archetype names are defaults in the configuration. They have no meaning in the code, and
  a config can define others.
- The archetype values in the defaults are starting guesses to calibrate with simulation runs, see
  [Simulation guide](../simulation.md#npc-crowd).
