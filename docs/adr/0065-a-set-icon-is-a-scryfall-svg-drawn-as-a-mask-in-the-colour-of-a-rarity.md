# 0065 – A set icon is a Scryfall SVG, drawn as a mask in the colour of a rarity

## Status

Proposed — 2026-10-23

## Context

The player picks a set on the boosters page and on the tabs of the collection, among more than a
hundred. A name alone is slow to scan, and a card on the page does not say which set it is from.
The maintainers want the set's icon on the buttons that pick a set, and on the cards, in the colour
of a rarity: black for a common, silver for an uncommon, gold for a rare, purple for a mythic.

Colouring needs the icon as a vector. MTGJSON gives each set the code of its icon, `keyruneCode`, but
no picture. The Keyrune font would draw it, but its code-to-glyph table ships in a GPL stylesheet,
which this repository should not copy. Scryfall serves one SVG per set under the same rules as the
card pictures of [ADR-0035](0035-card-pictures-are-served-from-a-local-cache.md). The pages keep a
Content Security Policy of `default-src 'self'`, which refuses inline styles.

## Decision

- **Source:** the catalogue's `keyruneCode` is the set's `IconCode`. `internal/images` downloads the
  SVG at `svgs.scryfall.io/sets/{code}.svg` with the pacing of the card pictures, and `Cached` keeps
  it under the image cache directory. The browser asks `/set-icon/{code}` on its own server.
- **Drawing:** the page does not embed the SVG. `/set-icons.css` has one rule per set that masks an
  element of class `set-icon-{code}` with the picture, and the rarity class sets the colour behind
  the mask. A style attribute could not carry the picture, as the policy refuses it.
- **Colour:** a card's icon takes the colour of the card's rarity. A button that picks a set takes the
  colour of the rarest card the player holds in it, common when they hold none.
- **Legibility:** the icon sits on a round chip, light behind the black common icon and dark behind
  the others, so that each colour reads on the dark page, on the gold button and on a card picture.
- **Fallback:** a set whose icon cannot be downloaded, and the made-up sets of the demo, get a plain
  built-in icon that the browser does not keep, so the real one shows once it can be had.

## Consequences

- A set file written before the icons has no `IconCode` and shows no icon. It still opens.
- The first sight of a set waits for its icon, one at a time, and the next ones come from disk. A
  set that Scryfall does not draw asks again on every page until it does.
- The icons are Wizards of the Coast trademarks shown for what they name, a set, as the card pictures
  are. The footer credits Scryfall for them.
- The cache of icons is never emptied or refreshed. Deleting the directory is the way to reset it.
