# Market browse and trade stages: plan

Design: [market-browse-design](../specs/2026-10-26-market-browse-design.md). Each step starts from a
failing test and ends with `go test -race ./...` and `golangci-lint run` green.

1. **Domain: one asking price.** Extract the price of an ask answer into one function; the answer to
   a request calls it. Existing tests prove nothing changed.
2. **Domain: `ForSale`.** Test on the market fixture: cards held by NPCs that trade, cheapest first,
   an NPC with no behaviour left out.
3. **Domain: `BuyNow`.** Tests: price and card move, ledger sums to zero, sold-out shelf fails with
   `ErrNPCLacksCard` and no change, too few coins fails and no change.
4. **Domain: `Offer`.** Tests: a fair offer is accepted after the delay, a low one refused, the
   direct request ends with its answer, an open request refuses it, state round-trips through
   `State`/`Restore`.
5. **Domain: invariant.** Add Buy now and Offer to `randomMarketMove`.
6. **Play: `ForSale`, `BuyNow`, `OfferTo`, stages and next instants.** Persist the direct flag.
7. **Web: Browse, card page, buy and offer handlers.** Tests on the pages' HTML.
8. **Web: My trades.** Move today's page to `/market/trades`, group it, update the forms' way back.
9. **Demo and docs.** Demo test through the pages; update `docs/core-features.md`,
   `docs/todo.md`, and the `MarketView` comment.
10. **Verify.** Full tests, lint, a look at the pages in the demo.
