# 0074 – A milestone pays energy as a share of the reserve cap, into the savings

## Status

Proposed — 2026-10-24

## Context

A milestone rewards a part of a binder with energy, see [Milestones](../core-features.md#milestones).
The reserve holds 10 energy and recharges by itself, and the savings hold up to 200 and fill only
while the reserve is full, see [Energy](../core-features.md#energy). The game had no way to add
energy: the quests of [ADR-0070](0070-a-quest-names-its-action-and-its-reward-and-pays-at-once.md)
pay cards and sets.

The size of the bar has changed before, and a reward written in points would then buy less.

## Decision

A milestone's energy reward is a percentage of `energy.reserve_cap`, the bar, and is paid into the
savings.

- Half a bar is a booster in a regular set. A reward above one bar does not fit the reserve, which
  is why the reward goes to the savings.
- The reward goes to the savings, not the reserve, since the reserve recharges by itself. A bonus
  there would stop the recharge it replaces, and the savings are the pool that a bonus is added to.
- Energy above `energy.savings_cap` is lost, and the notice tells what was added.
- A reward of 0, or a milestone kind with none, pays nothing and still locks its cards.
- No method pays energy for coins. `Energy.Grant` exists for rewards only.

The percentages are in [`internal/config/milestones.go`](../../internal/config/milestones.go), and
their current values in the [tuning log](../tuning-log.md).

## Consequences

- The rewards follow the bar when it changes, with no change to the configuration.
- A rounding down can make a reward 0 for a very small bar. The milestone is then a lock with no
  payment.
- No simulation plays milestones, so the faucet they add to energy is not measured yet, see the
  [TODO](../todo.md).
