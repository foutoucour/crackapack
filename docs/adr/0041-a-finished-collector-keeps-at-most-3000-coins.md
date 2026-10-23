# 0041 – A finished collector keeps at most 3,000 coins

## Status

Accepted — 2026-10-16

## Context

After the binders are done, coins have nothing left to buy. The `collector` ended 90 days with 245,000
to 300,000 idle coins, and [ADR-0027](0027-a-foil-is-a-percentage-of-its-cards-price.md) gave the
master set as a sink that took the `master-collector` down to 632 to 2,984 coins on five seeds.

No band says how large a pile is acceptable, so the tuning system has nothing to hold the coin faucets
against.

## Decision

The tolerated pile is a `final_balance` of at most 3,000 coins. That is $30, thirty daily rewards at
the rate of [ADR-0006](0006-coin-rate-is-100-per-dollar.md).

It is read on the `master-collector`, over 90 days, with the Counter's fixed supply on, on a list of
several sets in scope. The plain `collector` is not judged: its pile measures what the sinks leave
unabsorbed.

The figure is set just above the worst of the five seeds, so it is the first reading and not a
calibration.

## Consequences

- A change that adds a faucet, such as a larger daily reward, has to come with a sink or it breaks the
  band.
- The band says nothing about the money supply across the crowd, which stays unmeasured.
- It rests on the foil premium of ADR-0027. Replacing that sink means revisiting this band.
