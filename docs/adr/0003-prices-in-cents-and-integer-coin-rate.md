# 0003 – Prices in cents and integer coin rate

## Status

Accepted — 2026-10-11

## Context

The simulation sweeps the coin-to-USD rate over a grid (for example 10, 25, 50, 100, 250,
500 coins per USD) and compares results across runs. Two requirements follow:

- Money arithmetic must be exact and reproducible. `float64` rounds implicitly, so sums depend on
  the order of operations and two runs that differ only in the coin rate would not compare
  cleanly.
- A single price-to-coin expression must absorb Counter percentages (sell at 60%, buy at 120%)
  without compounding truncation. Applying the percentage in a separate step after converting
  to coins would truncate twice, widening the buy/sell spread non-uniformly at low rates.

At low coin rates a $0.05 common converts to a fractional coin. Silently rounding to zero would
hide a real economic loss from the caller.

## Decision

Prices are `USDCents` (`int64`, US cents). The in-game currency is `Coins` (`int64`). The
coin-to-USD rate is `CoinRate` (`int64`, whole coins per US dollar). Counter percentages are
`Percent` (`int64`, 100 = 100%).

Conversion is one integer expression with one truncation:

    coins = floor(price × rate × pct / 10_000)

`pct = 100` gives the reference value. The Counter passes 60 or 120 directly, so no second
truncation step occurs. When `price > 0` and `pct > 0` and the result is zero, `PriceToCoins`
returns `ErrSubCoin` instead of a silent zero. When any input is negative it returns
`ErrNegativeInput`.

## Consequences

- All money arithmetic uses integer types; `float64` is excluded from the domain.
- The single-truncation formula guarantees that the buy price is always at least twice the sell
  price (`floor(2x) >= 2·floor(x)` for x ≥ 0), so a buy-then-sell round trip at the Counter
  never profits. This is verified by a property test in `money_test.go`.
- Callers that display a card price must handle `ErrSubCoin` explicitly; the card cannot be
  silently treated as free.
- The safe input range is documented in the `PriceToCoins` doc comment. For realistic game
  values (price ≤ $10 000, rate ≤ 1 000, pct ≤ 10 000) the intermediate product stays below
  10^13, well within `int64`.
