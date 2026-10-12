# 0002 – YAML as the Simulation Configuration Format

## Status

Accepted — 2026-10-11

## Context

The simulation engine described in [Simulating the economy](../specs-review.md#simulating-the-economy)
is configuration-driven: every tunable value (Counter rates, NPC behaviour, rewards, energy
parameters, calibration targets) lives in a file rather than in code. That file must be:

- human-editable, so a developer or contributor can change a value without touching Go source;
- strict: an unknown key or a wrong value type must produce an error that names the offending key,
  not a silent default or a confusing runtime failure;
- pure Go with no CGO, matching the project's build requirement (see
  [ADR-0001](0001-go-for-the-core.md));
- covered by a maintained library.

Candidates considered:

| Format | Library | Why not chosen |
|--------|---------|----------------|
| JSON | `encoding/json` (stdlib) | No comments; verbose for configuration; no strict unknown-key mode in stdlib |
| TOML | `github.com/BurntSushi/toml` | Good format, but `go.yaml.in/yaml/v3` already meets every criterion and YAML is more familiar in this project's likely audience |
| HCL | HashiCorp HCL | Excellent for infrastructure; overkill for a game configuration file |
| YAML – `gopkg.in/yaml.v3` | — | Archived: in April 2025 the author @niemeyer labelled the repository as "unmaintained" (`go list -m -versions gopkg.in/yaml.v3` returns only v3.0.0 and v3.0.1, both pre-archive) |
| YAML – `go.yaml.in/yaml/v3` | YAML organization | See Decision |

Evidence for `go.yaml.in/yaml/v3`:

- `go list -m -versions go.yaml.in/yaml/v3` returns `v3.0.0 v3.0.1 v3.0.2 v3.0.3 v3.0.4`
  (five releases after the fork, vs. two for the archived upstream).
- The README states: "This project started as a fork of the extremely popular go-yaml project, and
  is being maintained by the official YAML organization… following his decision to label the project
  repository as 'unmaintained' in April 2025."
- The module is pure Go (no CGO), has the same API as `gopkg.in/yaml.v3` (same `Decoder`,
  `KnownFields`, `Node`, `TypeError` types), and the import path encodes the new home.

## Decision

The simulation configuration format is **YAML**, loaded with
**`go.yaml.in/yaml/v3 v3.0.4`**.

`Decoder.KnownFields(true)` rejects any key absent from the target struct with an error that
includes the field name. Type mismatch errors (a `yaml.TypeError` carrying a slice of
`"line N: cannot unmarshal …"` strings) are enriched by a two-pass decode: the YAML is first
parsed into a `yaml.Node` tree to build a line-number → dotted-key-path map, and each error
string's line number is replaced with its key name before the error is returned to the caller.
Semantic constraints (range checks, invariants) are validated in a `validate` step that also names
the offending key.

## Consequences

- **Pros:**
  - Human-editable, comment-friendly format.
  - `KnownFields(true)` enforces strict unknown-key rejection out of the box.
  - Type-error enrichment satisfies the requirement that an invalid value names its key.
  - Pure Go, CGO-free, consistent with ADR-0001.
  - Actively maintained by the YAML organization.
- **Cons:**
  - YAML's indentation sensitivity is an authoring hazard; invalid indentation yields a parse
    error, not a field-level one.
  - `go.yaml.in/yaml/v3` does not natively support default values; defaults are applied in code
    before decoding, and the decoder merges into the pre-populated struct. Maps merge per key,
    so a partial `initial_stock:` override keeps unspecified rarities at their defaults.
  - The two-pass decode (Node tree + struct decode) adds a small constant overhead at startup;
    it is negligible for a configuration file.
