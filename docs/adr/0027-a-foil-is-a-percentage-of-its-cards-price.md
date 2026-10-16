# 0027 – A foil is worth a percentage of its card's price at the Counter

## Status

Accepted — 2026-10-16

## Context

The price snapshot holds one price per card, see [Price snapshot JSON](../simulation.md#price-snapshot-json),
so the Counter priced a foil as its non-foil. The Counter's code called that a simplification pending a
per-finish price source.

That left coins with one use. On RAV, GPT, DIS, CSP, TSP, PLC and FUT with TSB, over 90 days, the
`collector` finished every binder on day 44 and then piled up 260,000 to 300,000 idle coins on two
seeds. About 96% of its coins came from duplicates sold to the Counter, and the daily reward gave
9,000 of about 410,000. Two attempts to give those coins a use failed:

- A policy that saved for the dearest missing card it could reach never finished the binders and
  idled 264,000 to 273,000 coins.
- A throwaway policy that bought every missing foil bought none, because the Counter never stocked a
  foil in a default run.

The master set, every card in normal and foil, is already a planned long-term goal, see
[Collection goals and badges](../retention.md#collection-goals-and-badges). It is a goal worth
spending coins on only if a foil costs more than its non-foil.

## Decision

`counter_foil_pct` is the percentage of a card's reference price that its foil is worth at the
Counter, for the price the Counter pays and the price it asks. The default is 500, the factor that
absorbed the idle pile in the runs below. A config that sets 100 keeps the price a foil had before
this ADR. It is a whole number above zero and at most 10,000, the bound of the
other percentages, see [ADR-0014](0014-numeric-config-keys-have-upper-bounds.md).

The same factor applies to both directions, so the round trip through the Counter still loses coins
whenever `counter_sell_pct` is below `counter_buy_pct`.

A factor stands in for a per-finish price source until `fetchdata` keeps the foil price. Replacing it
with real foil prices is a new ADR that supersedes this one.

## Consequences

- A run that does not set the key prices a foil at five times its card, so a run's numbers differ from
  those before this ADR, and a foil sold to the Counter pays five times as much. Set the key to 100
  to get the old prices back.
- Foils reach the Counter's shelf only from sales to it and from the boosters it opens, so a run that
  wants foils to buy turns on `counter_supply`, see
  [The Counter as a special player](../simulation.md#the-counter-as-a-special-player).
- On the seven sets above, with the Counter opening 100 boosters a day, a `master-collector` that
  buys every missing foil after the non-foil cards ended with 24,000 to 55,000 coins at day 90 at a
  factor of 300, against 260,000 to 300,000 for the `collector`. On seed 1 its balance stayed under 2,500
  coins through day 60, so the coins were used while it collected. At a factor of 100 it bought about
  as many foils and still ended with 215,000 coins: the premium, not the foils, is what makes the sink.
- A factor of 500 absorbs the whole pile: the `master-collector` ended with 632 to 2,984 coins at
  day 90 on five seeds, against 245,000 to 300,000 for the `collector`, and bought 1,337 to 1,427
  cards at the Counter. At 1,000 it ended with about
  1,900 coins and bought fewer, 1,300 to 1,342, because it runs out of coins before it runs out of
  foils. A factor between 300 and 1,000 therefore tunes how much of the master set the coins reach.
- One group of sets, and five seeds at 500 but two at 300 and 1,000, so the factors are a first read,
  not a calibration.
- A foil is priced at the Counter only. NPC trades, the coin fee on boosters and the collection value
  still use the card's price, so a foil is worth more to the Counter than to the rest of the game.
- The master set has no progress metric of its own yet: the policy counts a card done when it owns
  one foil copy.
