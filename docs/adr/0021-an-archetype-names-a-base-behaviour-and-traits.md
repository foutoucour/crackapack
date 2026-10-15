# 0021 – An archetype names a base behaviour and traits

## Status

Accepted — 2026-10-15. Supersedes [ADR-0010](0010-npc-trade-behaviour-is-a-strategy.md).

## Context

[ADR-0010](0010-npc-trade-behaviour-is-a-strategy.md) lets an archetype name exactly one registered
`TradeBehaviour`. That fits the five archetypes it was written for, each with a behaviour of its
own.

The next archetypes are personalities tied to the Magic colours: stingy, ego, gambler, fish and
archivist. They are meant to mix, so that one archetype can be both ego and stingy. With one
behaviour per archetype, every mix is a type of its own, and five personalities make 31 of them.
Most of what the personalities add does not choose wishes at all: it changes the margin, refuses a
counter or reacts to a declined offer. A strategy that picks wishes cannot express those on its own.

## Decision

An archetype names one base behaviour, as ADR-0010 decided, and a list of traits. The base behaviour
keeps supplying the wish candidates and the proposal. A trait shapes how the archetype trades on
top of it: it can add to the margin, score wishes, veto a counter and react to a declined offer.

Traits mix in a way that does not depend on their order, so `[ego, stingy]` and `[stingy, ego]`
give the same NPC.

- **Margin:** the base margin plus the sum of the trait adders. An adder is never negative, so an NPC
  still never gives more value than it gets. A config number caps the total.
- **Wishes:** each trait scores a candidate card. The NPC ranks the candidates by their total score,
  and the base behaviour's own order breaks ties.
- **Counters:** one veto from any trait makes the NPC refuse a player counter.
- **Declines:** every trait that reacts to a declined offer is told about it, and each reacts on its
  own.

A trait reads only numbers from the archetype's configuration, as a behaviour does. Traits are
registered by name, and config rejects an unknown name. A new personality is a trait type, one
registry entry and a config entry.

An archetype with no traits trades exactly as under ADR-0010. The five default archetypes keep
their base behaviour and get no trait.

## Consequences

- A mixed archetype is a list of traits in its configuration, not a new type: the ego-and-stingy
  archetype is `traits: [ego, stingy]`.
- The margin cap guards against stacked adders that make an archetype never trade. The cap value and
  the trait numbers are to calibrate with simulation runs.
- `TradeBehaviour` may need splitting, for instance into candidates and ranking, so that trait
  scores apply to what the base behaviour offers. The implementation settles that, within these
  rules.
- ADR-0010 stays as written, apart from its status.
