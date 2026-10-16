# Architecture

crackapack is a Go core with no UI, exposed through a JSON API, with a web client embedded in the
same binary. The goal is to defer the blocking choices — which UI, whether to go online — as late
as possible. What locks a project in is where the business logic lives, not which UI it uses.
So the logic lives in the core, and every UI is a replaceable client.

The language choice is recorded in [ADR-0001](adr/0001-go-for-the-core.md).

```mermaid
flowchart TB
    subgraph Binary["crackapack binary"]
        Domain["Domain<br/>boosters, collection, economy, ledger, NPCs"]
        Store["Store adapter<br/>SQLite"]
        API["API adapter<br/>HTTP + JSON"]
        Sync["Sync adapter<br/>catalogue updates"]
        Web["Embedded web client"]
        Store --> Domain
        API --> Domain
        Sync --> Domain
        Web --> API
    end
    Sync -->|set files| Sources[(MTGJSON, Scryfall)]
    Browser[Browser on localhost] --> Web
```

## Layers

The domain holds the rules: booster generation, collection, coins, the double-entry ledger and the
NPC simulation. It imports no infrastructure: no database, no HTTP, no filesystem, no clock, no
global randomness. Everything it needs arrives through interfaces it declares.

Adapters implement those interfaces and depend on the domain, never the reverse.

- The store adapter persists state in SQLite, through a pure-Go driver so the binary builds without
  CGO.
- The API adapter translates HTTP requests into domain calls and holds no business rules.
- The sync adapter keeps the local catalogue up to date, see
  [Legal and data](core-features.md#legal-and-data).

`cmd/crackapack` is the only place where real adapters are built and wired together. `demo/` holds
the deterministic demo dataset. `tools/` holds Go programs run by CI.

### Packages in the simulation MVP

`internal/config` loads and validates the YAML configuration, translating it into a typed
`Config` that `cmd/crackapack` passes to the engine. It reads from an `io.Reader` and never opens
files itself. The NPC archetypes are the one exception to "the configuration comes from the
caller": they ship inside the binary as embedded files, read through an `fs.FS`, so the player
cannot change them, see [ADR-0023](adr/0023-archetypes-are-embedded-and-read-only.md).

`internal/sim` is the composition layer: it wires `internal/domain` objects with `internal/config`
values and records per-day metrics. The domain never imports sim.

The domain also holds the trades: `Exchange` prices and executes offers between the player and the
crowd, `Valuer` gives the Counter and the Exchange the same reference value, and a `TradeBehaviour`
per archetype picks wishes, the margin and proposals. `internal/sim` composes them behind
`trades.enabled` and exposes them to player policies through `Session`.

`demo/` generates two synthetic catalogues (DEM and DSP/DST) from a fixed seed and exposes them
via `demo.Build()`. The domain never imports demo; the dependency runs one way, from demo into
the domain.

All simulation state is held in plain in-memory domain objects for the MVP. No store interface
exists yet: it will be introduced with the SQLite adapter, together with the shared contract suite
that both implementations must pass.

## Local storage

The binary is separate from the player's data.

- The database lives under the OS user config directory.
- The image cache lives under the OS user cache directory.

Replacing or upgrading the binary never touches either.

## Time and the NPC market

The NPC market keeps moving while the app is closed. The simulation runs on a fixed tick with a
seeded random generator. On launch, it replays the ticks missed since the last session, with an
upper cap so that a long absence does not stall the start-up.

Because every random draw comes from an injected, seeded generator, a simulation run is
reproducible from its seed. That is what makes the economy debuggable and the demo deterministic.

## Going online later

Going online means deploying the same core as a server, and changing two adapters.

- The store moves from SQLite to a server database behind the same interfaces.
- The API gains authentication.

The web client keeps calling the same API. Draws stay server-side because the domain already owns
them. Nothing in the domain changes.

A native client, such as a game-engine front end, would be another client of the same API.
