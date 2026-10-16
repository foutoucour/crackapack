# 0035 – Card pictures are fetched by the server and served from a local cache

## Status

Accepted — 2026-10-16

## Context

A card is shown by its Scryfall picture, unchanged, see
[Legal and data](../core-features.md#legal-and-data). The terms ask for 50 to 100 ms between
requests, and the page opens fifteen cards at once. The game must keep working offline once it has
seen a card, and [ADR-0034](0034-the-web-client-is-server-rendered.md) gives the pages a Content
Security Policy of `default-src 'self'`.

## Decision

The browser never talks to Scryfall. The page points each card at `/card-image/{scryfallId}` on its
own server. `internal/images` answers it:

- `Scryfall` downloads the "normal" picture, one request at a time, with a pause after each, an
  identifying `User-Agent`, and a bound on the size.
- `Cached` keeps each picture in a file under the OS user cache directory, so a card is downloaded
  once. A failed download is not cached, so the next request retries.
- The ID must be a lower-case UUID. It is checked before it reaches a URL or a file name.

A card with no Scryfall ID, or whose picture cannot be downloaded, shows as text. The demo draws its
own SVG pictures, since its cards are synthetic, so `crackapack play --demo` needs no network.

## Consequences

- The Content Security Policy needs no exception for another origin, and no request leaves the
  machine from the browser.
- The first sight of a card waits for its download, one card at a time. A booster of fifteen new
  cards takes a few seconds, and the next ones are served from disk.
- The cache is never emptied or refreshed. Deleting the directory is the way to reset it.
- Scryfall's terms for a local tool that downloads pictures are still to be checked, see
  [Legal and data](../specs-review.md#legal-and-data).
