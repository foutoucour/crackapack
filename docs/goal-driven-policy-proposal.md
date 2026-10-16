# Proposal: a goal-driven player policy (`chaser`)

Status: proposed, not implemented. Implementation waits for the last NPC archetype story (T-009,
the mixed archetypes and the activated personalities) on branch `002-npc-archetypes-r5` to merge,
because both change `docs/simulation.md`.

## Why

The open "Coin sinks" item in [TODO](todo.md#mechanics-not-simulated-yet-in-v1) asks for coins to
have uses while a player collects, at a small daily faucet. Raising the daily reward does not do
that: it adds coins that go into the same sink, buying cards at the Counter.

A first test of energy as a competing use failed. The `energy-first` policy saves for the next
[top-up](simulation.md#energy-coin-top-up) before it buys cards. Over 72 runs on 105 sets, with the
daily reward at 100 and 300 and `p` from 50 to 300, it ended with about 2 to 10 points fewer
distinct cards than `converting-collector`. The standard deviation over 3 seeds was at most 0.8
points.
- A coin buys about 90 to 135 new cards at the Counter, against at most 16 to 43 as energy.
- The collector buys its cheapest missing cards first, at about 9 coins each, so cards win on card
  count at every `p`.
- The value of the final collection did not separate the two policies: the difference changed sign
  between configurations, which is noise at 3 seeds.

So for a player who wants to complete sets, coins already have one clearly best use. A choice has to
come from value, meaning rare cards, not from card count.

## Design

`chaser` is a player policy in the existing `PlayerPolicy` registry, see
[Player policies](simulation.md#player-policies).

- **Boosters and duplicates:** as `collector`. It opens boosters with energy only, with no top-up,
  and sells duplicates above a playset.
- **Goal:** the most expensive missing non-foil card whose buy price is at most 14 times the daily
  reward. That is the longest wait the player accepts for one card. At a daily reward of 100 it is
  1,400 coins. Without the cap the goal could be unaffordable for ever, and the player would hoard
  coins, the opposite of using them.
- **Buying:** if the balance minus the usual floor, one daily reward, covers the goal, it buys the
  goal and moves on to the next one. Otherwise it buys nothing and saves. A goal that is out of stock
  is skipped.
- **No state:** the goal is recomputed each session, so it stays the same until it is bought.
- **Placement:** one new file in `internal/sim`, next to the other policies, registered in
  `cmd/crackapack/policies.go`. It reuses `buildBuyCandidates`. The 14 days is a named constant, not a
  config key.

## Tests, written first

Table-driven tests with `fakeSession`, as for the other policies.
- It buys the most expensive affordable goal, not the cheapest card.
- It buys nothing and keeps saving when the balance falls short.
- It skips a goal priced above the 14-day cap and takes the next one.
- It skips an out-of-stock goal.
- The registry test includes `chaser`.

## Experiment once built

The same fixed-faucet comparison as for `energy-first`: daily reward 100 and 300, 3 seeds, 105 sets,
`chaser` against `collector` and `energy-first`. No new metric is needed, since rares and above owned
is `Owned - CUOwned` in the progress data. Four things to read:
- Rares owned.
- Collection value.
- Mean idle balance, which shows whether the coins get used.
- The wait between goals, which shows whether it feels like a goal.

## Not included

- A top-up inside the chaser.
- Working one set at a time.
- New config keys.
- Any change to the Counter's prices. A minimum price per card is the alternative lever, since
  commons at about 9 coins are why cards win on count. It changes an economy rule and needs its own
  ADR.
- Demo data. `crackapack demo` has no `--player` flag, so the demo rule in
  [`crackapack-conventions`](../.claude/skills/crackapack-conventions/SKILL.md#demo-data-ships-with-the-feature)
  cannot be met by following the precedent of `converting-collector`. This needs a decision.

## Sequencing

- The archetype stories T-001 to T-008 are on `main`. T-009 is still on `002-npc-archetypes-r5`, and
  it changes `docs/simulation.md`, `docs/core-features.md` and `docs/todo.md`. `chaser` needs
  `docs/simulation.md` and `cmd/crackapack/policies.go`, which that branch does not touch.
- `main` no longer has a `CHANGELOG.md`, so the policy needs no changelog entry.
- The uncommitted `energy-first` policy on branch `energy-first-policy` is based on an older
  `main`. Its code files do not overlap with the archetype work, and its `docs/simulation.md` edit
  needs rebasing onto the merged text.
- Implement both after T-009 merges, so the doc edits apply to the merged text.
