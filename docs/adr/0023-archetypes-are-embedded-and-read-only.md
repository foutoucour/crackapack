# 0023 – Archetypes are embedded and read-only

## Status

Accepted — 2026-10-15

## Context

[ADR-0009](0009-npc-archetypes-are-data.md) recorded that archetypes are configuration data.
[ADR-0021](0021-an-archetype-names-a-base-behaviour-and-traits.md) fixed their shape: each archetype
names a base behaviour and a list of traits. The five default archetypes were a Go map literal
inside `defaults()` in `internal/config/config.go`, which the simulation YAML could replace
wholesale via `npcs.archetypes`.

Archetypes are game design, not player settings, and the player must not be able to change them.
The planned tuning system, on the other hand, searches paths such as
`npcs.archetypes.casual.keep_copies`, so a developer still needs to override them for a run, see the
[tuning system](../todo.md#tuning-system-and-metrics).

## Decision

Each archetype is one YAML file embedded in the binary with `go:embed`. The file name is the
archetype name and must match `^[a-z][a-z0-9_]*$`. Only `.yaml` files are accepted.

The loader (`LoadArchetypes`) takes an `fs.FS` and decodes each file strictly with
`KnownFields(true)`. Unknown fields, unknown traits, unknown colours and unknown behaviours are
all rejected. A malformed shipped file causes `Load` to return an error; the simulation does not
start and the player sees the error, not a crash or a silent fallback.

There is no flag and no run-time directory for archetypes. A player binary cannot change them.

The simulation config's `npcs.archetypes` map still overrides an embedded archetype for the
`simulate` and `sweep` subcommands, replacing the entry as a whole. This is a developer tool for
calibration runs, not a player-facing feature. A new name in `npcs.archetypes` adds an archetype
for that run only.

## Consequences

- No archetype value lives in Go. Adding a new default archetype is a new `.yaml` file in
  `internal/config/archetypes/`.
- The player cannot change the five default archetypes. Calibration overrides are a developer
  concern, written as `npcs.archetypes` entries in the config file passed to `simulate` or `sweep`.
- A one-field override in `npcs.archetypes` must write the whole entry, because the embedded value
  is replaced wholesale, not merged. A search over one field has to carry the rest of the entry.
- A broken embedded file is caught by the tests that load the shipped archetypes, before the binary
  ships.
- It supersedes nothing: ADR-0009 and ADR-0021 remain accepted and unchanged. This decision
  changes where the default values live, not what they mean.
