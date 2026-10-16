---
name: crackapack-conventions
description: Use when writing, reviewing or testing code in crackapack. Covers package layering, domain rules (money, ledger, seeded randomness), demo data per PR, frontend rules and GitHub Actions rules. Pairs with go-development, which holds the generic Go practices.
---

# crackapack Conventions

The rules here apply because of what crackapack is. Idiomatic Go lives in
[`go-development`](../go-development/SKILL.md), and the team's opinions in
[`go-house-style`](../go-house-style/SKILL.md). Follow all three.

The layering these rules protect is described in [Architecture](../../../docs/architecture.md).

## Layering

- `internal/domain` never imports `database/sql`, `net/http`, `os` or reads `time.Now`. Adapters
  depend on the domain, never the reverse. Enforced by `depguard`.
- Real collaborators are built in one place: `cmd/crackapack/main.go`, or an `app.New(cfg)` it
  calls. Wiring in `cmd/` is exempt from test-first.
- User data and cache directories come from `os.UserConfigDir()`/`os.UserCacheDir()`, resolved in
  the composition root and injected. The domain reads files through `fs.FS`.
- Store implementations share one contract suite, run against SQLite and against the in-memory
  fake: `storetest.Run(t, newStore)`.

## Domain Rules

- Coins are `int64`. Never `float64` for money.
- The ledger is double-entry. Invariant tests assert every transaction sums to zero and no balance
  goes negative.
- Randomness is an injected, seeded `*rand.Rand`, so a simulation run is reproducible from its seed.
- The domain does not log. It returns errors and events; adapters log them.
- Metrics and OpenTelemetry are out of scope while the app is local. If the project goes online,
  instrumentation lives in adapters, never in the domain.

## Strategies in the Domain

Varying behaviour here is pricing policy, player policy, NPC personality and reveal ordering. Each
is a strategy where it passes the criteria in
[`go-house-style`](../go-house-style/SKILL.md#strategy-pattern--where-it-improves-the-code).

Booster layout per set era is not a strategy: a layout is data, the sheets and weighted options of
the set's catalogue, interpreted by one generator. A new generator is only warranted when the data
cannot express a layout, see [ADR-0004](../../../docs/adr/0004-booster-layouts-are-data.md).

```go
// BoosterGenerator builds one booster from a set catalogue.
type BoosterGenerator interface {
    Generate(cat SetCatalogue, rng *rand.Rand) (Booster, error)
}

type Opener struct {
    generators map[BoosterLayout]BoosterGenerator
    rng        *rand.Rand
}

func NewOpener(generators map[BoosterLayout]BoosterGenerator, rng *rand.Rand) *Opener {
    return &Opener{generators: generators, rng: rng}
}
```

The simulated player follows the same shape: `sim.PlayerPolicy` has one implementation per play
style (`collector`, `cash-out`, `idle`, `trader`, `exploiter`), registered by name in
`cmd/crackapack/policies.go`. NPC archetypes split in two: supply is config data interpreted by one
NPC step, see [ADR-0009](../../../docs/adr/0009-npc-archetypes-are-data.md), and trade behaviour is
a registered `TradeBehaviour` per archetype, see
[ADR-0010](../../../docs/adr/0010-npc-trade-behaviour-is-a-strategy.md), with traits mixed on top of
it, see [ADR-0021](../../../docs/adr/0021-an-archetype-names-a-base-behaviour-and-traits.md). An
archetype is an embedded, read-only YAML file, see
[ADR-0023](../../../docs/adr/0023-archetypes-are-embedded-and-read-only.md).

## Demo Data Ships With the Feature

Every PR that adds or changes user-visible behaviour includes, in the same PR, demo data that
exercises it.

- Demo data lives in `demo/` and is loaded by `crackapack demo` (or a `--demo` flag). It is never
  compiled into domain packages.
- It is deterministic: fixed seed, fixed clock, same demo on every run.
- It covers the new path. A new booster layout ships a demo set using it; a new NPC archetype
  ships NPCs of that archetype.
- A test loads the whole demo dataset through the real stores, so demo data cannot rot.
- It is small and hand-curated. No bulk Scryfall or MTGJSON dumps.

## Frontend

The web client is server-rendered `html/template` pages in `internal/web`, with one small script for
the reveal, see [ADR-0034](../../../docs/adr/0034-the-web-client-is-server-rendered.md). These rules
hold for any client.

- KISS. No abstraction until a second use exists.
- The frontend holds no business rules. Prices, drop rates and the ledger live in the Go core; the
  UI renders API responses. This keeps the online option open.
- Every view handles loading, error and empty states.
- Accessibility targets WCAG 2.1 AA: semantic HTML, full keyboard use (open the next booster
  without a mouse), alt text with card names.
- Reveal animations are skippable and honour `prefers-reduced-motion`.
- Card names and rules text are external data. Render them escaped; never `template.HTML` or
  `{@html}` on them.
- Layouts work on narrow and wide screens.

## CI and Releases — GitHub Actions

- Every PR runs lint, `go test -race ./...`, the `go mod tidy` diff check, `govulncheck` and the
  demo-data load test. All are required checks.
- Releases use GoReleaser on `v*` tags, with `CGO_ENABLED=0`, one binary per OS/arch in GitHub
  Releases.
- Third-party actions are pinned by commit SHA. `permissions:` defaults to `contents: read`; only
  the release job gets write.
- No secrets in the repo. Secrets come from `${{ secrets.X }}` and are never echoed.
- Logic lives in Go under `tools/`, not in shell inside YAML. Steps call `go run ./tools/...`.
- Before editing a workflow, read it whole. Make the minimal diff. Never rename a job: required
  checks are bound by job name.

When proposing a CI change, state: the existing pattern, the minimal change, the risk, how to
validate, how to roll back.

The commands are listed in [AGENTS.md](../../../AGENTS.md#commands).

## Review Checklist

Add these to the [`go-development`](../go-development/SKILL.md#review-checklist) and
[`go-house-style`](../go-house-style/SKILL.md#review-checklist) checklists.

- `internal/domain` imports no infrastructure package
- Coins are `int64`; ledger invariant tests pass
- Demo data for the feature is in the same PR and loads in the demo test
