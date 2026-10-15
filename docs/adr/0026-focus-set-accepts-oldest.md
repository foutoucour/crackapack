# 0026 – focus_set accepts a third mode: oldest

## Status

Accepted — 2026-10-15

## Context

[ADR-0023](0023-archetypes-are-embedded-and-read-only.md) makes each archetype an embedded, strict
YAML file, and [ADR-0022](0022-an-npc-wishes-only-for-cards-in-its-colours.md) applies an NPC's
focus set before its colour filter.

The `focus_set` field originally accepted a boolean: `false` (no focus, each booster picks a
random set) and `true` (focus, one set drawn at generation). The archivist archetype — a
white-aligned collector that hunts old sets — needs to lock every NPC in the archetype to the
oldest set currently loaded, without consuming a random draw and without the caller having to
know which set is oldest.

Two constraints shape the decision:

1. **Share-0 safety**: The archivist ships with `share: 0` until a calibration story raises it.
   A share-0 archetype generates no NPC, so resolution must not happen (and must not fail) when
   `share` is zero — even when the loaded sets carry no release dates.

2. **Backward compatibility**: existing YAML files use `false` and `true`; both must keep their
   current meaning. Adding a new string value must not break those files.

## Decision

`focus_set` becomes a three-way enum. The YAML unmarshaler accepts:

| YAML value | Meaning |
|---|---|
| `false`, `no`, `off` (YAML bool-false) | `FocusSetNone` — no restriction; crowd picks a random set per booster |
| `true`, `yes`, `on` (YAML bool-true) | `FocusSetRandom` — one random set drawn per NPC at generation |
| `"oldest"` (bare or quoted string) | `FocusSetOldest` — resolved to the set with the earliest release date at `GeneratePopulation` time |

On the domain side, `FocusSetMode` is an `int` iota (`FocusSetNone = 0`, `FocusSetRandom`,
`FocusSetOldest`). The zero value is `FocusSetNone`, matching the prior `bool false` default.

Resolution of `FocusSetOldest` happens once per archetype inside `GeneratePopulation`, before the
NPC loop, and only when the archetype's count is greater than zero. All sets must carry a
non-empty `releaseDate` field or the call fails with an error that names the missing codes. This
prevents silent errors when a set file was loaded without a date.

`FocusSetOldest` consumes no random draw, so the RNG state at a given seed is unaffected by
adding an archivist archetype with `share: 0`.

## Consequences

- Config files that use `false` or `true` for `focus_set` continue to work unchanged.
- `focus_set: oldest` is the only new string form; any other string, integer, list, or map is
  rejected with a clear error.
- A catalogue file without a date loads and validates normally. It produces an error only at
  `GeneratePopulation` time, when an archetype with at least one NPC asks for `oldest`.
- Ties between two sets with the same earliest date go to the first set in the order the caller
  passes. The result is deterministic as long as that order is stable.
- The `ReleaseDate` field is added to `SetCatalogue` and read from `data.releaseDate` in the
  MTGJSON file. A missing or empty field is not an error; an invalid ISO date is.
- Which set the archivist opens depends on the sets that are loaded.
