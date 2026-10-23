# 0059 – An NPC answers on the market after the delay of its archetype

## Status

Proposed — 2026-10-23

## Context

On the market of [ADR-0058](0058-the-player-trades-on-a-market-of-requests-and-listings.md) the
player asks around for a card or lists one, and NPCs answer. The maintainers want the answers to come
at the pace of the NPC's personality: some at once, some over hours, some the next day. A player who
comes back later finds answers waiting, which gives a second reason to come back tomorrow beside the
energy.

The game runs on the real clock and catches the Counter and its NPCs up lazily, in whole days, see
[ADR-0055](0055-the-counter-catches-up-on-whole-days-up-to-a-configured-cap.md). The domain never
reads the clock.

## Decision

Each archetype has a response delay, `response_delay_hours`, a key of its embedded file, bounded and
checked like the others, see [ADR-0023](0023-archetypes-are-embedded-and-read-only.md). Every NPC of
the archetype takes that long to answer a request, to make an offer on a listing and to reply to a
counter. An answer is due at an instant: when the player asked, listed or countered, plus the delay.

The market delivers what is due when it is told the time, in the order it fell due. The game tells it
when it loads a save and before every market call or page.

- **In time with the NPCs:** the game interleaves the market with the days it catches up, so an
  answer due on a day sees the NPCs as they stood that day. An answer due inside the current day sees
  them as the last catch-up left them, as it would have live.
- **Deterministic:** the answers follow from the state of the NPCs and the rules, with no random draw,
  so a reload, or a page opened at another moment, delivers the same answers.

## Consequences

- A delay of zero answers at once, so a player sees answers in the demo without waiting.
- The game delivers answers lazily, so an answer due while the game is closed shows when it opens,
  with the instant it was due.
- An answer that falls due during days skipped by the catch-up cap sees the NPCs as the first day
  played starts them.
- The delays of the shipped archetypes, and why each was chosen, are in the
  [tuning log](../tuning-log.md). The simulation does not read the delay.
