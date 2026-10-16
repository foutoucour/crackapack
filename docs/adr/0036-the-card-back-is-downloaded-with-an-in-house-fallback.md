# 0036 – The card back is downloaded once, with an in-house fallback

## Status

Accepted — 2026-10-16

## Context

A face-down card shows the standard Magic card back, which the maintainer chose over a drawing of
our own. It carries the Magic logo and a Wizards trademark, and the repository holds no images and
no Wizards content, see [Legal and data](../core-features.md#legal-and-data). The picture the
maintainer chose is a file on a fan wiki, Tolkien Gateway. That wiki has already replaced it once: the
revision of 2024-12-17 is 267 by 373 pixels, too small for a big card, and the current one is 744 by
1039.

[ADR-0035](0035-card-pictures-are-served-from-a-local-cache.md) already fetches pictures into a
local cache and serves them from this origin.

## Decision

The back is downloaded the same way as a card picture. `images.Fixed` fetches the one file at its
URL, under `images.CardBackID`, and a `Cached` keeps it in the image cache, so it is downloaded
once. `images.Chain` puts it ahead of the card pictures, and the stylesheet paints it from
`/card-image/<CardBackID>`.

Under it, the stylesheet paints an in-house drawing, `card-back.svg`. A background layer that fails to
load paints nothing, so the drawing shows on the first launch, offline, and if the wiki ever drops
the file. Nothing in the page has to know which of the two is showing.

Both pictures have white corners outside their rounded black border, since a JPEG has no
transparency. The stylesheet clips the pictures a little inside the real corner radius, in
percentages, so that no white shows at any size.

## Consequences

- The repository still holds no Wizards content: the back, like the card pictures, lives in the
  user's cache directory.
- The Magic logo and a Wizards trademark are on screen. The Fan Content notice is in every page's
  footer, and the open question of Scryfall's and Wizards' terms stays open, see
  [Specs review](../specs-review.md#legal-and-data).
- The back depends on a third-party fan wiki, one request per installation. If the file moves, the
  in-house drawing stands in until the URL is updated in `cmd/crackapack/play.go`. Switching to
  Scryfall's copy of the back means a new `Fixed` URL and nothing else.
- The pictures are masked at their corners, so they are not shown strictly as they are, which
  [Legal and data](../core-features.md#legal-and-data) asks for. What is hidden is the white outside
  the card's own border.
