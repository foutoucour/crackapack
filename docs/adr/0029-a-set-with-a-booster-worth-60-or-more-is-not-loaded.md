# 0029 – A set whose booster is worth $60 or more is not loaded

## Status

Accepted — 2026-10-16

## Context

The energy cost of a booster stops at 12 while booster values keep growing, so the coins a booster
pays per unit of energy grow with its value. At the 100 coins per dollar rate a Stronghold booster
($48) pays ×2.7 what a 2005-2007 booster pays per energy, Revised ($70) ×3.1 and Alpha ($1,080)
×65. One Alpha booster pays about 618 daily rewards. The figures, and the sets they come from, are
in [Expensive old sets and the 12-energy cap](../specs-review.md#expensive-old-sets-and-the-12-energy-cap).

These sets need their own design, which V1 does not have. Leaving them in the catalogue directory
without a rule also skews the median that sets every energy cost, see
[ADR-0008](0008-energy-cost-reference-is-the-catalogue-directory.md), since that median counts
every openable set of the directory.

## Decision

The sets of a catalogue directory fall into two bundles by the average value of one booster at the
price snapshot, [`BoosterAverageValue`](../../internal/domain/boostervalue.go). A set worth the limit
or more is in the expensive bundle, every other set in the common bundle. The limit is the config
key `expensive_booster_value`, in USD cents, default 6000 ($60).

Only the common bundle is loaded into the game.

- A set of the expensive bundle named in `--sets` stops the run with an error that names the set
  and its booster value. A run never drops a requested set without saying so.
- A set of the expensive bundle that only sits in the directory is left out without a message.
  `tools/fetchdata` puts such files there on purpose, so an error would make a shared directory
  unusable. It does not feed the median of the energy cost tiers.

The default is the real-data decision. The key exists because the synthetic prices of the tests are
worth far more than real ones, so those runs raise it. It is also how a later study brings the
expensive bundle back.

## Consequences

- The sets of the expensive bundle are outside V1, until a design handles the 12-energy cap for
  them.
- Moving the limit changes the median of the energy cost tiers, so compare runs at the same limit.
- The key is bounded above by 100,000,000 cents, following
  [ADR-0014](0014-numeric-config-keys-have-upper-bounds.md).
- The split reads prices, so it runs after the snapshot loads. `simulate`, `sweep`, `demo` and
  `play` all apply it. `play` without a price snapshot cannot tell, so it leaves nothing out.
- A card with no price is valued at `fallback_price`, so a set with many unpriced cards looks
  cheaper than it is. The study's values for Alpha, Beta and Unlimited are already lower bounds
  for that reason, and they sit far above the limit.
- A snapshot that lacks a whole set makes it look cheap, see
  [Catalogue directory](../simulation.md#catalogue-directory).
- The demo directory holds `DEX`, an expensive synthetic set, so the exclusion is exercised.
