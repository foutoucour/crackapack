---
name: codebase-audit
description: Use when asked to audit crackapack or assess a whole checkout's quality — code smells, dead code, test value, documentation freshness, agent instructions, CI hygiene. Read-only, produces cited findings and never fixes. Not for reviewing a diff, a PR or a single file.
---

# Codebase Audit

An audit is a read-only sweep of an entire checkout that ends in findings a human can act on. It
is not a code review of a change, and it is not a cleanup session. The value comes from evidence
and from naming recurring shapes, not from volume.

## Not This Skill

| Request | Use instead |
|---------|-------------|
| Review this diff, branch or pull request | A normal code review against [`go-development`](../go-development/SKILL.md), [`go-house-style`](../go-house-style/SKILL.md) and [`crackapack-conventions`](../crackapack-conventions/SKILL.md) |
| Is this one file or function any good? | [`go-development`](../go-development/SKILL.md), [`go-house-style`](../go-house-style/SKILL.md) and [`crackapack-conventions`](../crackapack-conventions/SKILL.md) |
| Fix the findings of a previous audit | A separate implementation session, after the decisions are answered |

## Audit Rules

Every rule is mandatory. A sweep that skips one produces findings the human cannot trust.

### 1. Read-only

Do not edit, stage, commit or switch branches. A one-character fix is still a finding, not an
edit. Posting the report where the human asked is not an edit, see [Rule 12](#12-stop-at-findings).

### 2. Freeze a scope contract before reading anything

Record the branch, the commit SHA, the surfaces in scope and the surfaces excluded. Excluded
surfaces are named, never silently dropped. Ask the human when the repo contains something the
sweep cannot judge, such as vendored or generated code.

### 3. One agent per surface, in parallel

When subagents are available, dispatch one per surface in [Dimensions](#dimensions), all at once.
Give each read-only tools only — no `Edit`, no `Write`, no mutating `git`. Rule 1 does not travel
to a subagent that was handed an editing tool. Without subagents, sweep the surfaces one by one
and say so in the scope line.

### 4. Evidence, or the finding is dropped

Every finding carries a `file:line` citation and the observation that proves it. Dead code carries
the search showing zero callers, quoted. A finding you cannot cite is deleted, not reworded into
something vaguer. When evidence collapses, remove every conclusion built on it.

### 5. Verified drift and suspicion are different buckets

Proven claims go in Detailed Findings. Suspicions go in Open Questions, as questions. Never
promote a suspicion into a task.

### 6. Audit documentation against code, in that direction

Take each claim in `README.md`, `AGENTS.md`, `docs/` and `docs/adr/`, then find the code that
proves or disproves it. Documentation that violates its own stated rules is a finding.

### 7. No silent dimensions

Each surface returns findings or an explicit "confirmed OK" list. Silence is indistinguishable
from not having looked.

### 8. Severity by user-visible impact

Rank by what the player or the maintainer would feel, not by how uncomfortable the code is to
read. See [Severity Ladder](#severity-ladder).

### 9. Judge against the repo's own rules first

Authority order, highest first:

1. `AGENTS.md`, the ADRs in `docs/adr/`, and the skills in `.claude/skills/`
2. General engineering practice

Name the rule each violation breaks: the ADR number, the `AGENTS.md` heading or the skill
section. A finding that cites a rule is not arguable; one that cites taste is.

### 10. No scope creep

Propose the smallest change that removes the defect. No rewrites, no new frameworks, no
abstractions the defect does not require. The report states what it deliberately leaves alone.

### 11. Decisions belong to the human

When a finding has more than one defensible resolution, write it as a numbered question with your
recommendation, and wait. Record answers in the Resolved Decisions table. The report is posted
first without that table; amend it in place when answers arrive. That amendment is the only write
allowed after posting.

### 12. Stop at findings

Post the report and stop. No implementation plan, no fixes.

| Invocation | Destination |
|------------|-------------|
| Invoked directly in a checkout | The session transcript — never a file in the audited repo |
| The human asks for a GitHub issue | That issue, posted once the human confirms |

### 13. A re-audit does not repeat declined findings

Look for previously accepted risk — a closed audit issue, or the last audit's decisions table —
and leave those findings out. Report the commit audited so the next sweep can diff against it.

## Dimensions

Seven surfaces. Probes are starting points; adapt the tooling, never the evidence bar.

### 1. Design and code smells

Duplication the code already pays for, hand-maintained `switch` chains next to a strategy
registry, functions with several concerns, layers importing past their boundary.

Probes:

- Author-flagged drift: `git grep -nE "TODO|FIXME|HACK|XXX|duplicate|copy of"`. These markers are
  this surface's only.
- Rank files by size and read the largest handful.
- For every strategy registry, search for `switch` or `==` comparisons on its keys elsewhere. That
  is a hand-maintained shadow.
- `go list -deps ./internal/domain/...` must show no `database/sql`, `net/http`, `os/exec` or
  adapter package.
- Patching smells in production code: a package-level `var` holding a function, `init()` with
  side effects, `os.Getenv` outside `cmd/`, `time.Now()` or global `rand` calls in the domain.

Evidence bar: cite both locations of a duplicate and what makes them the same. "Looks similar" is
not a finding. Two implementations sharing a shape but not an invariant are not duplication.

### 2. Dead code

Exported identifiers with no callers, packages nothing imports, struct fields nobody reads,
unreachable branches.

Probes:

- `go run golang.org/x/tools/cmd/deadcode@latest ./...` and `staticcheck` (U1000), then confirm
  each hit with `git grep -n "<symbol>"`.
- For each package: does a test cover it, does `cmd/` or CI reach it, does production code import
  it? All three "no" makes it a deletion candidate.
- Search escapes: `//nolint`, `//lint:ignore`, `t.Skip`. Read what they hide.

Evidence bar: quote the search and its result. Entry points reached from CI, GoReleaser,
`go:generate` or templates are not dead; say where you looked.

### 3. Test value

Ask what each test would catch if the code broke. Tests that fake the thing they claim to verify,
assert only `err == nil`, or duplicate a scenario covered elsewhere are cost without coverage.

Probes:

- Tests with no assertion beyond `if err != nil { t.Fatal(err) }`.
- White-box tests reaching unexported fields where a black-box test would do.
- Overlap between table-driven unit tests and higher-level tests on the same path. Keep the layer
  that catches the real failure.
- Interfaces with several implementations but no shared contract suite.
- Ledger invariant tests present and covering every transaction kind.
- Features with no demo data, or demo data not loaded by the demo test.
- Packages with no `_test.go`, judged by blast radius, not count.

Evidence bar: name the failure the test would miss. A thin test guarding a contract tooling cannot
check is doing real work; do not propose deleting it.

### 4. Documentation freshness

Docs against code, in that direction
([Rule 6](#6-audit-documentation-against-code-in-that-direction)).

Probes:

- Every command, flag, path and environment variable in `README.md` and `docs/` exists and works.
- Every behavioural claim ("idempotent", "offline", "deterministic") has code that proves it.
- ADR hygiene: duplicate numbers, `Accepted` ADRs describing structures the code no longer has.
- Apply [`project-documentation`](../project-documentation/SKILL.md): file listings, config dumps
  and restated facts are findings.

Evidence bar: cite the doc line and the code line that contradict each other.

### 5. Agent instructions

`AGENTS.md` and `.claude/skills/`, for accuracy and for cost. A wrong instruction file is worse
than a missing one: every session inherits the error.

Probes:

- Every command and target named in `AGENTS.md` exists; targets the repo has are not missing from
  it.
- `AGENTS.md` obeys its own rules.
- Rules duplicated between `AGENTS.md` and a skill, and generic advice with no repo-specific
  content.

Evidence bar: cite the instruction line and what contradicts it.

### 6. Operational and CI hygiene

Probes:

- GitHub Actions: actions pinned by tag instead of SHA, missing or broad `permissions:`, release
  steps without a tag condition, logic in shell instead of `tools/`.
- GoReleaser: `CGO_ENABLED=0`, targets matching the documented platforms.
- `go.mod`: `go mod tidy` leaves no diff, `govulncheck` is clean.
- Configuration: values a player would need to change without a release hardcoded in code; data
  paths not resolved through the OS user directories.
- Logging: `fmt.Print*` diagnostics, missing `status` attribute on outcome logs.

Evidence bar: cite the line and state the consequence in one sentence.

### 7. Anything odd

Where the most embarrassing findings come from.

Probes:

- Escaped placeholders: `git grep -nE "whatever|changeme|TBD|<your-|placeholder|lorem"`.
- Demo or fixture generators living in domain packages instead of `demo/`.
- Asymmetries: an API field the UI cannot show, a write path ignoring parameters its read path
  accepts.
- Anything you had to read twice to trust.

Evidence bar: say plainly why it is odd. Unproven oddities go in Open Questions.

## Report

Section order is fixed so two audits months apart are comparable.

1. **Scope line** — one italic paragraph: date, branch, commit SHA, number of agents and surfaces
   inspected, surfaces excluded.
2. **Summary — recurring shapes** — two to four numbered patterns, each one claim sentence then
   its instances. End with the strongest surface.
3. **Resolved Decisions** — only once answered. Table `# | Question | Decision`, decisions
   phrased as instructions.
4. **Open Questions** — suspicions and multi-resolution findings, each with a recommendation.
5. **Key Discoveries** — at most ten. Bold claim, citation, consequence.
6. **Detailed Findings** — one subsection per dimension, table `File:line | What | Evidence`.
   Merge duplicates across dimensions; keep the row where it matters most.
7. **Confirmed OK** — short lines: what was checked, and that it holds.
8. **Files Requiring Changes** — table `File | Lines | Change candidate`, ordered by severity,
   marked non-exhaustive if it is.
9. **What The Audit Is Not Proposing** — rewrites left alone, and previously declined findings
   skipped.

### Severity Ladder

| Severity | Means | Examples |
|----------|-------|----------|
| **Critical** | Ships wrong behaviour, or docs that cannot be followed | An unbalanced ledger transaction; a release binary that cannot load data; README install steps that fail |
| **Moderate** | Correct today, but a maintainer will break it | A `switch` shadowing a strategy registry; the domain importing `database/sql`; a package var swapped in tests |
| **Minor** | Cost without risk | A long single-concern function; a stale comment |

Effort is a separate axis and does not belong here. A critical one-character fix is still
critical.

Findings follow [`project-documentation`](../project-documentation/SKILL.md): one claim per
sentence, bold only for the claim a row is about, no fact stated in two sections.

## Checklist

- Scope line names date, branch, SHA, surfaces in scope and excluded
- Agent count matches the in-scope surfaces
- Every finding cites `file:line` plus its proving observation
- No suspicion outside Open Questions
- Documentation checked docs-first
- Every surface returned findings or a "confirmed OK" line
- Severity reflects user-visible impact
- Every violation names the rule it breaks
- What The Audit Is Not Proposing is filled
- Every multi-resolution finding is a numbered question with a recommendation
- Previously declined findings were left out
- No file edited, no branch changed, no plan written
