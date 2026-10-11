# AGENTS.md

Instructions for coding agents working on crackapack. Humans follow the same rules.

What the project is: see [README.md](README.md).
How the code is layered: see [docs/architecture.md](docs/architecture.md).

## Project rules

These rules are non-negotiable. The detail and the examples live in the skills; do not restate
them elsewhere.

| Rule | Where it is defined |
|------|---------------------|
| TDD, including a refactor step that prunes and merges tests | [`go-development`](.claude/skills/go-development/SKILL.md#development-loop--tdd) |
| SOLID | [`go-development`](.claude/skills/go-development/SKILL.md#solid-in-go) |
| Dependency injection, never patching | [`go-development`](.claude/skills/go-development/SKILL.md#dependency-injection--never-patch) |
| Strategy pattern for varying behaviour | [`go-development`](.claude/skills/go-development/SKILL.md#strategy-pattern--the-default-for-varying-behaviour) |
| Demo data in the same PR as the feature | [`go-development`](.claude/skills/go-development/SKILL.md#demo-data-ships-with-the-feature) |
| Documentation and ADRs | [`project-documentation`](.claude/skills/project-documentation/SKILL.md) |
| Whole-repo quality audits | [`codebase-audit`](.claude/skills/codebase-audit/SKILL.md) |

Architecture decisions are recorded in [docs/adr/](docs/adr/). A change that contradicts an
accepted ADR needs a new ADR first.

## Commands

The Go module does not exist yet. Once it is scaffolded, these are the commands, and this section
must be updated to match the real ones:

- `go test -race ./...` — run all tests
- `golangci-lint run` — lint
- `go run ./cmd/crackapack` — run the app locally
- `go run ./cmd/crackapack demo` — run the app on the demo dataset

## Language

Code, comments, docs, commit messages and the UI are in English.
