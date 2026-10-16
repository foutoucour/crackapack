# 0034 – The web client is server-rendered with html/template

## Status

Accepted — 2026-10-16

## Context

[Architecture](../architecture.md) keeps every UI a replaceable client of the Go core, and left the
choice of stack open between htmx with `html/template` and Svelte. The first UI opens boosters and
nothing else: a list of sets, one action, and a page that reveals the cards.

Two constraints shape the decision:

1. **One binary.** The README promises a single executable per OS that serves its UI on
   `localhost`. A Svelte client adds a Node toolchain and a build step to CI and to every
   contributor's machine.

2. **No rules in the client.** Prices, drop rates and the ledger stay in Go, so that the online
   option stays open.

## Decision

The web client is server-rendered. `internal/web` renders `html/template` pages embedded in the
binary, over `internal/play`, which owns the player's session. Actions are plain HTML forms that
answer with a redirect, so a refresh never repeats an action.

Interaction that a page load cannot give, such as the card-by-card reveal, is a small script served
from the binary and loaded as progressive enhancement: without it the page shows every card face
up. There is no framework, no build step, and no inline script or style, so the Content Security
Policy is `default-src 'self'`.

htmx is not adopted yet. No page needs a partial update, and adding it later does not change the
templates' structure.

## Consequences

- No Node toolchain: `go build` produces the whole product, and `go test` covers the pages through
  `httptest`.
- Every action is a page load. That suits opening boosters, and it will be revisited for features
  that need a live view, such as a trade in progress.
- The handlers of `internal/web` call `internal/play` in process, so they are both the API adapter
  and the client for now. A JSON API, when a second client needs one, is another adapter over the
  same `play.Game`; it does not move any rule.
- Card names and rules text are external data. The templates escape them, and a test pins it.
- A Svelte or game-engine client stays possible beside this one. Choosing it would supersede this
  ADR for that client only.
