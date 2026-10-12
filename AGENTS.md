# AGENTS.md

Instructions for coding agents working on crackapack. Humans follow the same rules.

What the project is: see [README.md](README.md).
How the code is layered: see [docs/architecture.md](docs/architecture.md).

## Project rules

These rules are non-negotiable. The detail and the examples live in the skills; do not restate
them elsewhere.

| Rule | Where it is defined |
|------|---------------------|
| Idiomatic Go: naming, errors, concurrency, testing, tooling | [`go-development`](.claude/skills/go-development/SKILL.md) |
| KISS | [`go-house-style`](.claude/skills/go-house-style/SKILL.md#kiss) |
| TDD, including a refactor step that prunes and merges tests | [`go-house-style`](.claude/skills/go-house-style/SKILL.md#development-loop--tdd) |
| SOLID | [`go-house-style`](.claude/skills/go-house-style/SKILL.md#solid-in-go) |
| Dependency injection, never patching | [`go-house-style`](.claude/skills/go-house-style/SKILL.md#dependency-injection--never-patch) |
| Strategy pattern where it improves the code | [`go-house-style`](.claude/skills/go-house-style/SKILL.md#strategy-pattern--where-it-improves-the-code) |
| Demo data in the same PR as the feature | [`crackapack-conventions`](.claude/skills/crackapack-conventions/SKILL.md#demo-data-ships-with-the-feature) |
| Documentation and ADRs | [`project-documentation`](.claude/skills/project-documentation/SKILL.md) |
| One decision per ADR; supersede ADRs, never amend them | [`project-documentation`](.claude/skills/project-documentation/SKILL.md#single-decision-per-adr) |
| Whole-repo quality audits | [`codebase-audit`](.claude/skills/codebase-audit/SKILL.md) |

Architecture decisions are recorded in [docs/adr/](docs/adr/). A change that contradicts an
accepted ADR needs a new ADR first, one that supersedes the old one.

## Commands

- `go test -race ./...` — run all tests
- `golangci-lint run` — lint
- `go mod tidy -diff` — check that `go.mod` and `go.sum` are tidy
- `go run ./cmd/crackapack` — print the usage and the list of subcommands
- `go run ./tools/fetchdata --sets RAV,TSP,TSB` — download real MTGJSON catalogues and a price snapshot into `./catalogues` and `./prices.json`
- `go run ./cmd/crackapack simulate --catalogue-dir ./catalogues --sets RAV,CLS --prices prices.json --out ./out` — run one solo-loop simulation
- `go run ./cmd/crackapack sweep --catalogue-dir ./catalogues --sets RAV,CLS --prices prices.json --out ./sweep-out` — sweep the coin-per-USD rate over a grid and report calibration results
- `go run ./cmd/crackapack compare [--days-csv delta.csv] <a.json> <b.json>` — compare two simulation results
- `go run ./cmd/crackapack demo` — run a sweep on the built-in synthetic demo dataset

Every pull request runs lint, tests, the tidy check and `govulncheck` through
[`.github/workflows/ci.yml`](.github/workflows/ci.yml); the `vuln` job there shows how to run
`govulncheck` locally. On this private repository on a free plan, GitHub cannot enforce these as
required checks, so a red check is advisory: do not merge on red.

## Language

Code, comments, docs, commit messages and the UI are in English.
