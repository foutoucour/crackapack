# 0067 – The daily reward is paid for each day the game catches up

## Status

Proposed — 2026-10-24

## Context

The economy table of [Core features](../core-features.md#economy-coins-sources-and-sinks) lists a daily reward of
100 coins as a source for the player, and the simulation pays it at the start of each simulated day.
The game had none: its coins came from the welcome bonus and from sales.

The game runs on the real clock, also while it is closed. Its Counter, supply and NPCs are played
for the whole days that went by when the game is next used, up to a cap, see
[ADR-0055](0055-the-counter-catches-up-on-whole-days-up-to-a-configured-cap.md).

## Decision

The player is paid `daily_reward` coins for each day the game catches up on, in the same pass that
plays the Counter's day. It is paid from the `daily_reward` ledger account, and the player claims
nothing.

## Consequences

- A day is paid once it is whole, as the Counter's supply is, and the first reward comes a day after
  the game starts.
- The days beyond `catch_up_days` are skipped, and so is their reward. A player away for months does
  not come back to a pile of coins, and the reward is worth no more to a game left closed than to one
  played every day.
- The header's coins catch the game up before they read the balance, so that a reward shows as soon as
  a page loads. A catch-up that fails there is not shown, and the next action returns the error.
- The save keeps the balances and the day caught up to, so a reload pays no reward twice.
- `daily_reward` is the simulation's key, so a change to it moves the game and the simulation together.
