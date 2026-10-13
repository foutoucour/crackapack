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

## Design priority: deep, configurable simulations

The point of the feature is to run long simulations across many scenarios and parameter sets. Every
choice below is checked against two requirements:

- **Configurable.** Every number that shapes trade behaviour is a flat config key with a default,
  validated by `config.Load`. A key addressed by its path (`trades.offer_expiry_days`,
  `npcs.archetypes.speculator.margin_pct`) can be overridden by the tuning system without new
  code. No tunable is a constant in the domain. A scenario is a config file plus a player policy.
- **Cheap per run.** A run with 500 NPCs over 90 days must stay fast enough for thousands of runs,
  and its memory must not grow with the run length. See [Cost of a run](#cost-of-a-run).

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
last valid day. Each side gives at most `trades.max_cards_per_side` cards (default 10, as in the
docs), counting quantities.

- **Player proposal.** `Propose` returns accepted (the trade executes), countered (same cards, the
  smallest top-up that satisfies the rule, rounded up to a whole coin) or refused (empty offer, or
  a card the NPC lacks).
- **NPC proposal.** The NPC asks for cards from its wish list that the player holds as duplicates
  (copies above `trades.duplicate_keep`, default 1) and gives cards from its shelf. The player accepts, ignores it until it
  expires, or counters once with different cards. The NPC then accepts or refuses, and does not
  counter again.
- **Atomic transfer.** Validate both sides' holdings and the player's balance, post the coin
  top-up (the only step that can still fail), then move the cards.
- **Expiry.** `StartDay(day)` lapses offers past their last valid day.
- **History and lookups.** Outcome counters always run. The per-offer log is bounded and opt-in
  (`trades.history_size`, default 0 keeps none), and `HoldersOf(CardID)` returns the NPCs that
  hold that card on their shelf.
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

- `trades.enabled` (default `false`), `trades.offer_expiry_days` (default 3),
  `trades.max_cards_per_side` (default 10), `trades.duplicate_keep` (default 1) and
  `trades.history_size` (default 0).
- `trades.max_open_offers_per_day`: a cap on the offers the crowd makes the player each day
  (default 20). It keeps the offer volume a parameter and the inbox bounded.
- `npcs.surplus_hold_days` (default 2).
- Per archetype: `behaviour` (a registered name), `wish_size`, `margin_pct` and `offer_chance_pct`,
  the chance per tick that an NPC with a match proposes it. The casual starts at 0% margin and the
  speculator at 15%, per [Core features](core-features.md#npcs-and-simulation).
- Load rejects a negative margin, an unknown behaviour, a negative wish size, and a percentage
  outside [0, 100] where one is expected. Errors name the offending key, as `config.Load` does.
- Behaviours read only these numbers. A behaviour that needs a new knob adds a key, not a constant.

## Cost of a run

- **Matching by index.** The crowd keeps `CardID → NPCs wishing it` and `CardID → NPCs holding it
  on the shelf`. Proposals iterate the player's few duplicates and look up these indexes, instead
  of scanning every NPC every tick.
- **Shelf aging.** The shelf is an arrival-ordered queue, so releasing aged surplus to the Counter
  is amortised constant time per card.
- **Bounded memory.** Offers are dropped on expiry or answer, the history is a ring of
  `trades.history_size` entries, and counters are plain integers. A longer run costs time, not
  memory.
- **No shared state.** `Exchange`, `Crowd` and the journal belong to one run, and each run draws
  from streams derived from its seed. Runs are independent, so a sweep or a search can evaluate
  them in parallel and reproduce any of them from its config and seed.
- **Benchmark.** A Go benchmark runs 500 NPCs for 90 days with trades on and off. The plan records
  the baseline, and CI does not gate on it.

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

Five phases, each green on its own. Config and metrics come before policies, so scenarios can be
swept as soon as the engine trades:

1. Valuer and ledger account.
2. Exchange core.
3. Shelf, wishes, behaviours, indexes and the ADRs.
4. Config keys, validation, metrics and the benchmark.
5. Session, port, the `trader` and `exploiter` policies, CLI wiring, demo data and docs.

The demo config enables trades with a small crowd, so `crackapack demo` shows trade metrics.

## Assumptions to confirm

- A player duplicate defaults to a copy above the first (`trades.duplicate_keep`).
- The card cap applies to the NPC side too. The docs state it for the player side only.
- `trades.max_open_offers_per_day` (20) and `offer_chance_pct` are new knobs with guessed defaults,
  to calibrate with runs.
- `Session` embeds `Trading`. The only other implementation is `fakeSession` in
  `internal/sim/collector_test.go`, which gets the new methods.
