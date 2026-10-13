# Trades with NPCs: design

Working design for the [Trades with NPCs](todo.md) item, agreed in chat on 2026-10-13. Once the
feature lands, its content moves into [Simulation guide](simulation.md), the ADRs and
[Core features](core-features.md), and this page is deleted.

## Goal

Make the economy simulable with trades, so the tuning system can later search over margins, wish
lists and offer expiry. Simulation only: no SQLite, API or GUI.

The rules that already hold, from [Trades with NPCs](core-features.md#trades-with-npcs):

- NPCs hold no coins. Only the player tops up with coins.
- An NPC receives at least the reference value it gives, plus a margin that is never negative.
- A transfer is atomic.
- No chain of trades gains value.

## Scope

In: the whole todo item, except the milestone interaction. A trade that removes the last copy of a
card can cost a milestone, but milestones do not exist yet, so `Exchange` only exposes a hook.

Out: NPC state in the real app, auctions, the player's own wish list.

## Decisions

| Question | Decision |
|----------|----------|
| Where trade rules live | A new `domain.Exchange`. `sim` only composes. |
| Reference value | A `Valuer` extracted from `Counter.referencePrice`, shared by Counter and Exchange. |
| Counter-offers | An NPC counters once, deterministically. No open-ended negotiation. |
| NPC behaviour | A `TradeBehaviour` strategy per archetype, which supersedes ADR-0009 for trades. |
| What an NPC can give | Surplus above its keep threshold waits on a shelf before going to the Counter. |
| Offer expiry | 3 days, configurable. |
| Existing runs | Unchanged: trades and the shelf are off by default. |

## Valuer and ledger

`NewValuer(prices, fallback)` with `Price(CardID) USDCents`: the snapshot price, else the fallback.
`Counter` builds one internally, so `NewCounter` keeps its signature. `sim` builds another for
`Exchange` from the same inputs.

Values are compared in exact integer arithmetic. A coin top-up counts at the coin rate. The NPC
accepts when `received × 100 ≥ given × (100 + margin)`. A negative margin is rejected.

A new unguarded sink, `AccountTradeTopUp`, receives the player's top-ups. NPCs have no account.

## Exchange

An offer holds an ID, an NPC, the cards each side gives, a coin top-up, the creation day and the
last valid day. Each side gives at most 10 cards, counting quantities (`MaxTradeCards`).

- **Player proposal.** `Propose` returns accepted (the trade executes), countered (same cards, the
  smallest top-up that satisfies the rule, rounded up to a whole coin) or refused (empty offer, or
  a card the NPC lacks).
- **NPC proposal.** The NPC asks for cards from its wish list that the player holds as duplicates
  (copies above the first) and gives cards from its shelf. The player accepts, ignores it until it
  expires, or counters once with different cards. The NPC then accepts or refuses, and does not
  counter again.
- **Atomic transfer.** Validate both sides' holdings and the player's balance, post the coin
  top-up (the only step that can still fail), then move the cards.
- **Expiry.** `StartDay(day)` lapses offers past their last valid day.
- **History and lookups.** An append-only log of every offer and its outcome, and
  `HoldersOf(CardID)`, the NPCs that hold that card on their shelf.
- **Milestone hook.** An optional `OnCardsLeaving` callback, called before commit, a no-op until
  milestones exist.

## NPC side

- An NPC gains a wish list and a shelf of surplus copies, each stamped with its arrival tick.
- At the end of a tick, shelf copies older than `npcs.surplus_hold_days` go to the Counter. With a
  hold of 0, supply behaves exactly as today.
- Wished cards the NPC has completed are replaced at the end of each tick.
- `TradeBehaviour`:
  - `PickWishes` fills the wish list up to `wish_size`.
  - `Margin` returns the non-negative margin for a given offer.
  - `Propose` picks which player duplicates to ask for and what to give from the shelf.
- Five implementations, one per default archetype: casual, playset hunter, targeted collector,
  speculator, whale. They are registered by name. A new personality is a type plus a registry entry.
- Offers and wish picks draw from the crowd's own random stream, so the player's draws do not move.

## Session, policies and metrics

- A `Trading` interface, implemented by the engine's port and embedded in `Session`:
  `Offers`, `ProposeTrade`, `AcceptOffer`, `CounterOffer`, `AcceptCounter`. `WorldView` adds
  `HoldersOf`, open offers and trade history.
- Refusals, insufficient balance and expired offers are normal outcomes. Other errors abort the
  run, as for the Counter.
- Policies, selected with `--player`:
  - `trader`: collects, accepts NPC offers that bring cards it lacks, proposes swaps for missing
    cards using `HoldersOf` with a top-up within a budget, and accepts counters.
  - `exploiter`: chains trades across NPCs to gain reference value, then sells to the Counter.
- Daily metrics: trades proposed (by the player and by NPCs), accepted, countered, refused, expired,
  trade volume in coins of reference value, and top-ups. As with the crowd, the columns appear only
  when trades are enabled, and `compare` refuses to mix the two.

## Configuration

- `trades.enabled` (default `false`) and `trades.offer_expiry_days` (default 3).
- `npcs.surplus_hold_days` (default 2).
- Per archetype: `behaviour` (a registered name), `wish_size`, `margin_pct`. The casual starts at
  0% and the speculator at 15%, per [Core features](core-features.md#npcs-and-simulation).
- Load rejects a negative margin, an unknown behaviour and a negative wish size.

## ADRs

One decision each, see [AGENTS.md](../AGENTS.md):

- 0010: NPC trade behaviour is a strategy. Supersedes ADR-0009.
- 0011: NPCs hold surplus on a shelf before selling it to the Counter.
- 0012: an NPC counters once.

## Testing

TDD, no mocks of internal code.

- `Valuer`: unit tests. The existing Counter tests stay untouched as the regression check.
- `Exchange`: table tests for the acceptance boundary (exact equality, the margin, rounding up), the
  smallest-top-up property and the 10-card cap. Atomicity tests: an insufficient balance or a
  missing card leaves both collections and the journal unchanged. Expiry and history tests.
- Invariant: a seeded property test runs random trade chains across NPCs. The player's coins plus
  collection reference value never rise through trades. A simulation test runs `exploiter` for 90
  days and asserts the same.
- Behaviours: a determinism test per implementation for wishes, margin and proposals.
- Engine: one seed gives one result. With trades disabled, the output is byte-identical to today's.
  Enabling trades leaves the player's booster draws unchanged.

## Delivery

Five phases, each green on its own: Valuer and ledger account; Exchange core; shelf, wishes,
behaviours and ADRs; Session, port and policies; metrics, config, CLI, demo data and docs. The demo
config enables trades with a small crowd, so `crackapack demo` shows trade metrics.

## Assumptions to confirm

- A player duplicate is a copy above the first. A playset-based definition would wait for milestones.
- The 10-card cap applies to the NPC side too. The docs state it for the player side only.
- `Session` embeds `Trading`. The only other implementation is `fakeSession` in
  `internal/sim/collector_test.go`, which gets the new methods.
