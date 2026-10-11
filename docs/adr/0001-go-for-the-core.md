# 0001 – Go for the Core

## Status

Accepted — 2026-10-10

## Context

crackapack ships as one binary per OS that a player downloads and runs, with no runtime to install.
The core holds all business logic, and every UI is a client of its API, see
[Architecture](../architecture.md). The core language therefore has to:

- produce a single, small, dependency-free executable for Linux, macOS and Windows
- cross-compile from one CI runner
- embed SQLite and a web client without CGO
- be fast to write, for one developer working mostly with a coding agent
- model a domain made of many variants: rarities, finishes, booster slots, ledger entries, NPC
  actions

Candidates considered:

| Option | Why not chosen |
|--------|----------------|
| Rust | Strongest type system and a cheap path to WebAssembly, but slower to write and slower to iterate on with a coding agent |
| TypeScript | One language front and back, but the type system can be bypassed at runtime and the binaries are large |
| C# | Only compelling if a Godot client returns to the table |
| Kotlin | Sealed types fix Go's main gap, but shipping a native binary is the hardest of the set |
| Python | Fastest to write for the author, but packaging it as one binary is fragile |
| Godot as the whole app | Best for animation, weakest for the data-heavy screens; logic written in GDScript would lock the project into the engine |

## Decision

The core is written in Go.

## Consequences

- **Pros:**
  - `go build` produces one static binary per target, and GoReleaser publishes them.
  - A pure-Go SQLite driver and `embed` keep the build CGO-free.
  - The language is small, so both the author and the coding agent write it reliably.
  - The standard library covers the HTTP server and structured logging.
- **Cons:**
  - Go has no sum types and its `switch` is not exhaustive. The `exhaustive` and `gochecksumtype`
    linters close most of that gap and are mandatory, see
    [`go-development`](../../.claude/skills/go-development/SKILL.md#domain-modelling).
  - Running the core in a browser through WebAssembly is possible but heavy. A serverless,
    browser-only version of the game is therefore not a cheap option.
  - A future native client, such as Godot, can only reach the core over its API. It cannot link
    the core as a library.
