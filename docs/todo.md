# TODO

What is left to build, as of 2026-10-15. The economy simulation is the detailed part. This page
lists work, it does not re-explain it: each item links to the doc that owns the design.

What exists today is described in the [Simulation guide](simulation.md). The scope and the build
order are in [Specs review](specs-review.md#v1-scope-and-build-order).

The end goal of the economy work is a tuned economy found from data, not chosen by hand. The
mechanics make the economy simulable, and the [tuning system](#tuning-system-and-metrics) searches
many parameters at once for a config that meets the goals.

## Done so far

- [x] Booster generation at the real collation rates, energy, collection and binders.
- [x] The double-entry ledger, the Counter and the daily reward.
- [x] A headless, deterministic simulation with the `collector`, `cash-out` and `idle` player
  policies.
- [x] `simulate`, `sweep` over the coin rate, `compare` and `demo`, and the `fetchdata` downloader.
- [x] The coin rate settled at 100, see [ADR-0006](adr/0006-coin-rate-is-100-per-dollar.md).
- [x] The NPC crowd, supply only, see [NPC crowd](simulation.md#npc-crowd).
- [x] Trades with NPCs in the simulation, see [Trades](simulation.md#trades). The `trader` and
  `exploiter` policies exercise them, and the exploiter can decline counter-offers.
- [x] Mythic as a rarity of its own, see [ADR-0013](adr/0013-mythic-is-a-rarity-of-its-own.md).
- [x] Binder pace, idle coin and booster wait metrics, see
  [Summary metric names](simulation.md#summary-metric-names).
- [x] The Counter as a special player: a shelf life for what it buys, a daily supply of boosters, its
  own energy and a finite treasury. See
  [The Counter as a special player](simulation.md#the-counter-as-a-special-player) and
  [ADR-0018](adr/0018-cards-sold-to-the-counter-can-vanish.md).
- [x] Strict config input with upper bounds on numeric keys, see
  [ADR-0014](adr/0014-numeric-config-keys-have-upper-bounds.md) and
  [ADR-0017](adr/0017-config-input-is-strict.md).
- [x] The coin fee on boosters above the 12-energy cap, see
  [The coin fee on dear boosters](simulation.md#the-coin-fee-on-dear-boosters).
- [x] Coin top-up for missing energy and the `converting-collector` policy, see
  [Energy coin top-up](simulation.md#energy-coin-top-up).
- [x] The etched finish loads as a foil.
- [x] A card on a foil sheet loads with a foil finish, which fixes the SOI crash on Tamiyo's
  Journal, see [ADR-0028](adr/0028-a-foil-sheet-card-loads-with-a-foil-finish.md).
- [x] A binder counts the pool cards its boosters can draw: Time Spiral is 402 cards, in the
  simulation, the player policies and the collection page, see
  [ADR-0037](adr/0037-a-binder-counts-the-pool-cards-its-boosters-draw.md).
- [x] The pre-Mirage sets load: the loader reads `booster.default` when a set has no
  `booster.draft`, see [ADR-0020](adr/0020-a-set-file-booster-is-its-draft-or-default-definition.md).
- [x] The colour personalities: the `stingy`, `fish`, `ego`, `gambler` and `archivist` traits and
  archetypes, two mixes (`ego_stingy` UB and `archivist_ego_stingy` WUB), archetype colours,
  `focus_set: oldest` and the embedded archetype files, see
  [ADR-0021](adr/0021-an-archetype-names-a-base-behaviour-and-traits.md) to
  [ADR-0026](adr/0026-focus-set-accepts-oldest.md). Their shares are small starting guesses, and the
  demo population is 70 so that every archetype gets a member.

## Economy simulation

### Mechanics not simulated yet (in V1)

- [ ] **Trades with NPCs**, see [Trades with NPCs](core-features.md#trades-with-npcs) and
  [Trades](simulation.md#trades). Built, except what depends on the milestones item below.
  - [x] NPC wish lists: picked per archetype by its behaviour, sized by `wish_size`, and refilled as
    an NPC completes cards.
  - [x] Archetype margins in the configuration, with the casual at 0% and the speculator at 15%.
    Trade behaviour is a strategy per archetype, see
    [ADR-0010](adr/0010-npc-trade-behaviour-is-a-strategy.md). The margins of the other archetypes
    are guesses to calibrate.
  - [x] A reference value shared by the Counter and trades: `Valuer`.
  - [x] NPC-proposed trades when a wish list matches the player's duplicates.
  - [x] Player-proposed trades: up to 10 cards plus coins, against cards from one NPC.
  - [x] The acceptance rule: the NPC receives at least the reference value it gives, plus a margin
    that is never negative.
  - [x] Counter-offers on both sides, once, see [ADR-0012](adr/0012-an-npc-counters-once.md).
  - [x] Atomic transfer: everything goes through or nothing does.
  - [x] A ledger account for the coins a player adds to a trade, since NPCs hold no coins:
    `trade_topup`.
  - [x] Offer expiry: 3 days by default, `trades.offer_expiry_days`.
  - [x] Trade history, and "which NPCs have this card as a duplicate". NPCs hold their surplus on a
    shelf to have duplicates, see [ADR-0011](adr/0011-npcs-hold-surplus-on-a-shelf.md).
  - [ ] A trade that removes the last copy of a card can cost a milestone, which depends on the
    milestone item below. Not built: no hook exists for it yet.
  - [x] Simulated player policies that trade, `trader`, and one that hunts for exploits by chaining
    trades across NPCs, `exploiter`. A test shows that no trade gains value.
- [x] **Exploiter cost cliff.** The `exploiter` with trades takes minutes when sessions and NPCs are
  both high, see [Trades](simulation.md#trades).
  - [x] Symptom, from the stress test: the exploiter with trades on, `sessions_per_day: 144` and 300
    NPCs takes 74 seconds for 10 days on the 5-set pool. One relaxation brings it down: 3 sessions a
    day gives 2 s, 30 NPCs 9 s, trades off 2 s. The collector, trader and idle policies take 1 to 5
    seconds on the same config.
  - [x] Cause: a counter-offer the exploiter does not accept stays open until it expires, three days
    by default, and nothing lets a policy decline it. The open offers pile up, and each call of
    `Exchange.Offers` sorts all of them while `ProposeFromNPCs` scans all of them every tick.
  - [x] Fix, to choose one: a way to decline an open offer, on the Exchange and on the `Trading`
    interface, so the exploiter drops the counters it will not take; or closing a policy's own open
    counters when its session ends. Keeping the open offers in ID order, instead of sorting them on
    every call, would help either way.
  - [x] Done when the config above runs in a few seconds, and a benchmark covers the exploiter with
    trades at a high session rate so that the cliff cannot come back unseen.
- [ ] **Daily quests and chest**, see [Daily quests and chest](retention.md#daily-quests-and-chest).
  - [ ] Three quests a day drawn from a catalogue of quests, and a claim step.
  - [ ] The chest worth +2 energy. `Energy` has no way to add a bonus yet.
  - [ ] Reward values, which are to be calibrated.
  - [ ] Whether a new basic land counts for "discover 10 new cards".
- [ ] **Collection milestones and rewards**, see
  [Collection goals and badges](retention.md#collection-goals-and-badges).
  - [ ] Rewards in coins and energy for commons, uncommons, rares, the binder, the timeshifted
    cards and the playsets. Several values are marked "to calibrate".
  - [ ] Reversible rewards: giving away the last copy that completed a milestone takes the reward
    back.
  - [ ] Ledger source accounts for these rewards. The existing metrics only report the days to
    reach a milestone.
- [ ] **Welcome boosters.** The welcome bonus is 200 coins and 10 boosters, see
  [Installation and saves](core-features.md#installation-and-saves). The play session grants the
  boosters, see the GUI section below. The simulation engine still grants only the coins.
- [ ] **Coin sinks.** Coins have nothing to buy once the binder is done, and the pile after a full
  binder was 330,000 to 1 million coins in the runs. Sinks were deferred until trades or cosmetics
  exist. Choose them, see [Economy: risks and fixes](specs-review.md#economy-risks-and-fixes). The
  leading candidate is the coin top-up in the item below.
  - [x] Foils as a sink: `counter_foil_pct` and the `master-collector` policy, see
    [ADR-0027](adr/0027-a-foil-is-a-percentage-of-its-cards-price.md). At 300 the idle pile after 90
    days fell from 260,000-300,000 to 24,000-55,000 coins on two seeds, and to under 3,000 at 500 on five seeds.
  - [ ] Calibrate the foil factor on more seeds and sets, replace it with real foil prices in
    `fetchdata`, and give the master set a progress metric and a reward.
- [ ] **Energy cost plus coin top-up**, so that coins matter for pace. A booster keeps its energy
  cost and part of that energy can be paid in coins at one rate. Without energy a pack cannot be
  opened. Decided: the design above. To simulate: the limit on converting.
  - [x] A constant `p`, in coins per energy, at least the best coins per energy of the tiered sets
    (about 92). Start at 100. Built as `energy.coins_per_energy`, off by default.
  - [x] A mandatory coin fee for a booster worth more than the 12-energy cap, `V - 12p` for a booster
    of value V. The 2005-2007 sets pay nothing. See
    [The coin fee on dear boosters](simulation.md#the-coin-fee-on-dear-boosters) and
    [ADR-0019](adr/0019-the-player-pays-a-coin-fee-on-boosters-above-the-energy-cap.md).
  - [x] The `collector` stalls when no set's fee is affordable. On RAV, TSP and TSB over 30 days at a
    `p` of 10 it opened no booster, since it spends its coins on cards and never saves for a fee.
    Not a problem at a `p` of 100, where these sets pay nothing. Fixed by the `converting-collector`
    policy, which raises the coin floor by the minimum fee so it never spends the fee buffer on card
    purchases, and uses coin top-up when short on energy.
  - [x] Optional coin top-up when opening a booster: the player pays coins for any energy shortfall
    at rate `p`. Minimum real energy (`min_energy_units`) is always required; coins only is never
    allowed. Three variants: constant rate (`min_energy`), escalating price that resets daily
    (`escalating`), and both together. Built behind `energy.coin_topup.*`; see
    [Energy coin top-up](simulation.md#energy-coin-top-up).
  - [x] A property test that coin top-up never raises net worth. Asserts
    `coin_topup_coins >= coin_topup_units × p` for all variants over 54 seed/p/step/variant combos.
  - [x] A player policy that uses top-up. The `converting-collector` policy uses `OpenBoosterTopUp`
    in Phase 1 and raises the coin floor by the minimum booster fee to prevent spending the fee
    buffer on card purchases; see [Player policies](simulation.md#player-policies).
  - [x] Stall regression test on real RAV+TSP+TSB catalogues: `converting-collector` opens >= 1
    booster in 30 days at p=10; the plain collector opens 0.
  - [ ] Simulate the energy cost as the current 3, 5, 8 and 12 tiers and as a continuous price of
    `V / p`. Replacing the tiers needs a new ADR superseding
    [ADR-0008](adr/0008-energy-cost-reference-is-the-catalogue-directory.md).
  - [ ] An ADR when the variant is chosen.
- [ ] **The Counter as a special player.** Cards sold to the Counter vanish, whatever the source,
  and the Counter opens its own boosters from an energy budget. Stock then follows the real pull
  rates, and the NPC crowd no longer feeds it. Multiple counters were considered and rejected as
  too complex.
  - [x] Stage 1a: the `counter_shelf_days` flag, off by default: `-1` forever, `0` a sink, `N` days
    on the shelf, see [The Counter's shelf life](simulation.md#the-counters-shelf-life).
  - [x] Stage 1b: a fixed supply of boosters a day for the Counter, with no energy. Sweep the supply,
    since supply sets the pace: an empty Counter with no crowd took about 49 days for the binder
    against about 17 with 25 NPCs.
  - [x] Stage 2: the Counter pays energy and booster costs from an energy of its own. The coin top-up
    for expensive boosters comes with stage 3, since it needs the Counter's treasury.
  - [x] Stage 3: a finite coin budget for the Counter, instead of minting, with the coin fee for
    expensive boosters. Built behind `counter_budget` and `counter_supply.coins_per_energy`, to be
    simulated. It makes coins a closed loop with its own ledger invariants.
  - [x] Adapt the player policies, since selling becomes permanent. None lost a card it needs, and a
    test pins that; a broke Counter is now offered each card once per session, not each copy.
  - [x] An ADR for vanishing sales, see
    [ADR-0018](adr/0018-cards-sold-to-the-counter-can-vanish.md).

### Calibration and decisions

The tuning system below is meant to settle most numeric values here. The decisions that stay manual
are the goals and their bands, the scope, and the remedies that change a rule.

- [ ] **Re-run the sweep against the product targets.** The targets are set, see
  [ADR-0039](adr/0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md) to
  [ADR-0043](adr/0043-no-card-is-worth-less-than-one-coin-at-the-counter.md). The window behind
  [ADR-0006](adr/0006-coin-rate-is-100-per-dollar.md) still rests on the demo's targets.
  [ADR-0049](adr/0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md) is still
  Proposed: its Counter bands were read once, without trades, and no candidate landed inside the
  out-of-stock band, see the [tuning log](tuning-log.md).
- [x] **Implement the Counter's one-coin floor**, see
  [ADR-0044](adr/0044-the-counter-pays-at-least-one-coin-for-a-priced-card.md): the Counter's sell
  price and `sub_coin_cards` use `domain.PayoutCoins`, so only a card with no price is refused. The
  demo sweep used to split its grid on one-cent cards: it now splits it on the daily reward alone,
  and the sweep tests split theirs on `final_balance`.
- [ ] **NPC defaults:** population, archetype split, rhythms and keep thresholds. A first run on
  RAV and TSP with the defaults gave about 3,400 boosters a day at 500 NPCs and millions of cards
  in the Counter after 90 days. With the Counter starting empty, 50 NPCs were enough to match the
  default-stock pace. One seed, one dataset.
- [ ] **Keep thresholds of the trading archetypes.** A playset hunter keeps four copies and so has
  almost nothing to trade, and a speculator keeping two trades half as much as the others. Decide
  whether they should be trade partners, see
  [Why hunters and speculators trade less](specs-review.md#why-hunters-and-speculators-trade-less).
- [ ] **The numbers of the colour personalities.** The shares (4 for each single-colour archetype, 3
  and 2 for the mixes) and every trait parameter are starting guesses. Trait margins price a
  player's proposal or counter and not an NPC's own proposals, see
  [NPC crowd](simulation.md#npc-crowd).
- [ ] **The trader policy in a crowd of many archetypes.** `proposeSwaps` visits the NPCs that hold
  a wanted card in ascending ID order and stops after 20 proposals a session. An archetype with low
  IDs and many NPCs can use the whole budget, and the others are never offered a swap. Decide
  whether the policy should shuffle the holders with the session's random stream, which changes the
  runs of the `trader` and `exploiter` policies. Until then, a test runs each archetype alone, so a
  shipped archetype that cannot trade is still caught.
- [ ] **A lever that keeps the Counter's stock scarce.** With a crowd of that size, "limited stock"
  no longer limits what a player can buy, which weakens the fix for "everything can be bought at the
  Counter". The leading option is the Counter as a special player, see the item under
  [Mechanics not simulated yet](#mechanics-not-simulated-yet-in-v1).
- [ ] **Initial Counter stock:** which cards, instead of a flat count per rarity.
- [ ] **Counter rates:** the 60% and 120% are starting values.
- [ ] **Energy:** calibrate the cost tiers against the real booster values, and check the
  collector's `collectorReserveDays = 1` and `sessions_per_day` assumptions.
- [ ] **Pacing target** of a set's binder in about two weeks at 3 visits a day. Real-data runs
  (90-day cap, p=100, no NPCs, seeds 42-44) give 8.3 days for RAV (cheap, energy=5) and 11.0 days
  for TSP (mid, energy=8); at 6/day they take 6.7 and 8.7 days respectively. Both are within the
  goal but faster than the target; decide whether to slow the pacing, see
  [Open questions](specs-review.md#open-questions).
- [ ] **Remedy for the 12-energy cap.** The priciest old sets create too many coins per energy, see
  [Expensive old sets](specs-review.md#expensive-old-sets-and-the-12-energy-cap). No remedy is
  chosen, so no ADR is written. The leading candidate is the coin fee above the cap, in the energy
  cost plus coin top-up item.
- [ ] **Re-run the Time Spiral figures that count the whole binder.** Time Spiral's binder is now 402
  cards, the simulation included, see
  [ADR-0037](adr/0037-a-binder-counts-the-pool-cards-its-boosters-draw.md). A recorded
  `days_to_100pct.tsp` or `coins_after_full_binder` for a run with TSP is stale, and so may be the
  11.0 days quoted under the pacing target above if it was the whole binder. The common-and-uncommon
  figures are not. The runs need real catalogues and a price snapshot, so they were not redone.
- [ ] **Reference value of a card with no TCGplayer price.** Today it is `fallback_price`.
- [ ] **Foil prices.** A foil shares its non-foil's price, a documented simplification pending a
  per-finish source. An `etched` finish loads as a foil, so it shares that price too.
- [ ] **Duplicate protection and jokers:** commons and uncommons only, or rares once the set is
  complete, see [Lessons from MTG Arena](retention.md#lessons-from-mtg-arena).
- [ ] **Scope:** Ninth and Tenth Edition in V1, and a first economy run on Planar Chaos and Future
  Sight, which no recorded run covers.
- [ ] **Star printings:** a rule for the foil-only duplicate printings of 10E, 9ED and ONS, which
  make their binders almost unreachable, see
  [The star printings of 10E, 9ED and ONS](specs-review.md#the-star-printings-of-10e-9ed-and-ons).
- [ ] **When live prices replace the snapshot**, if ever.

### Scenarios to run

Runs that test whether the economy is as stable as the sensitivity runs suggest. The first and last
run today. The other two wait for features that are not built. The
[tuning system](#tuning-system-and-metrics) can reuse them as its scenarios.

- [ ] **A coin-bound run.** Energy is the throttle today. Players end with 0.4 to 2.2 million coins,
  and the coin rate, the Counter percentages and the trade settings did not move the pace, see
  [What each parameter does](specs-review.md#what-each-parameter-does). Lower the energy cost or
  raise `sessions_per_day` until coins limit what the player can buy, then repeat the coin sweeps
  there.
- [ ] **A run with faucets and sinks.** Run it once the quest, milestone and coin sink items above
  exist. Today's balance only grows, and the rewards add coins and energy on top.
- [ ] **A mixed player population.** Several play styles in one run, with skipped days and different
  session counts, instead of one rational player present every day. Needs the short-session policy
  listed under [Tuning system and metrics](#tuning-system-and-metrics).
- [ ] **An expensive-set run.** Run sets worth far more per booster than the 2005-2007 ones, to see
  where the economy breaks and to inform the [12-energy remedy](#calibration-and-decisions). The
  pre-Mirage sets load since
  [ADR-0020](adr/0020-a-set-file-booster-is-its-draft-or-default-definition.md), so a catalogue
  directory can hold them; no recorded run covers them as loaded.

### Tuning system and metrics

- [ ] **Tuning system:** search many parameters at once and find, from simulation data, configs
  that give a good economy. A run is scored by a loss, and the search lowers it, the way training
  lowers a loss. Built as `crackapack tune`, see [The tuning system](simulation.md#the-tuning-system),
  [ADR-0045](adr/0045-a-config-is-scored-by-its-distance-outside-the-bands.md) and
  [ADR-0046](adr/0046-the-search-is-staged-and-validated-on-unseen-seeds.md). The product's files
  are in `tune/` and a first search ran on the 107 sets, see the [tuning log](tuning-log.md). Its best
  config is not applied: the defaults already meet more of the economy bands. What is left is a search
  whose loss does not favour speed so strongly, and a wider range for the keys at the edge of theirs.
  - [x] **Goals and bands**, a manual decision. Each goal is a band on a metric, reusing the
    `between`, `at_most` and `at_least` targets. The sets they apply to are in
    [ADR-0039](adr/0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md):
    - [x] Pace per set: [ADR-0050](adr/0050-a-binder-is-never-fast-and-some-sets-are-slow-on-purpose.md).
    - [x] No runaway coins: [ADR-0041](adr/0041-a-finished-collector-keeps-at-most-3000-coins.md).
      The money supply has no band yet.
    - [x] Healthy Counter stock, trades included:
      [ADR-0049](adr/0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md).
    - [x] Fair prices at small scale:
      [ADR-0043](adr/0043-no-card-is-worth-less-than-one-coin-at-the-counter.md).
  - [x] **Search space file:** parameters addressed by their path in the config, such as
    `counter_buy_pct` or `npcs.archetypes.casual.keep_copies`, each with a range, a type and a
    scale. A candidate is the base config with those paths overridden, then loaded through the
    normal validation. Invalid combinations score worst. Every config key added later can be tuned
    without new code.
  - [x] **Scenarios:** a player policy, a visit rate and a set list. Pace needs the 3 and 6 visit
    rates, and each goal names the scenarios it applies to.
  - [x] **Loss:** the distance outside each band, normalised by the band's size, averaged over
    seeds and summed. A loss of 0 means every goal is met.
  - [x] **Counter stock metrics:** `rare_out_of_stock_pct` and `rare_stock_per_printing`, with the
    bands of [ADR-0049](adr/0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md), which
    kept the numbers of ADR-0042, now superseded. It is still Proposed: a first search judged the bands
    without trades, see the [tuning log](tuning-log.md), and the out-of-stock band was not met.
  - [ ] **`rare_unreachable_pct`**, the share of rare printings that neither the Counter nor an NPC's shelf
    holds, and a goal scenario with `trades.enabled`. Until then `tune/goals.yaml` judges the Counter's
    stock without trades.
  - [x] **Parallel evaluation** across cores, reproducible from a search seed.
  - [x] **Search:** staged random search. A cheap stage on one seed keeps the best fraction, and the
    full batch of seeds, policies and sets scores the survivors.
  - [x] **Validation:** re-score the best configs on seeds the search never used and report the gap
    between the two scores, so a lucky seed or one play style cannot win.
  - [x] **Report:** the best configs, the feasible window of each parameter, which parameters move
    the loss, every evaluated candidate in JSON, and the best config as YAML that `simulate` and
    `sweep` accept.
  - [x] **Demo:** `crackapack demo --tune` ships a small search space and goals with the feature, see
    [Demo data ships with the feature](../.claude/skills/crackapack-conventions/SKILL.md#demo-data-ships-with-the-feature).
  - [ ] **Pace of 14 sets that never finish in 40 days:** 10E, 7ED, 8ED, 9ED, AER, AKH, BFZ, FRF, HOU,
    KLD, OGW, ONS, PLS and SHM, see the [tuning log](tuning-log.md). Find why, then decide whether the goal
    or the binder rule changes, see [Star printings](#calibration-and-decisions).
  - [ ] **Adaptive search:** an evolution strategy that starts from the survivors, once random
    search runs out of reach.
  - [ ] **New goals** for quests, milestones and trades as those mechanics land. The system does
    not need them to start.
- [ ] A metric for rare stock-outs, for example the days a card the player wants is out of stock.
- [ ] Coin pressure metrics, to judge whether coins matter: the share of income absorbed by sinks,
  the days to afford a chosen goal, the energy converted per day and the change in net worth.
- [ ] Money supply by flow once quests, milestones and trades add flows.
- [ ] More player policies: short sessions. The trading policies exist.
- [ ] Move the findings of the supply runs into the docs once the defaults are settled.

### NPC state in the real app

These need the SQLite store, which does not exist yet.

- [ ] NPC generation on first launch, with the seed stored.
- [ ] NPC collections stored as quantities, with an index on (printing, NPC), see
  [Technical](specs-review.md#technical).
- [ ] Offline catch-up: replay the ticks missed, capped at 48 hours, in batches with grouped writes.
- [ ] NPC names and avatars. The simulation has no use for them.

### After V1

- [ ] Mini-games and their daily caps, see
  [Game room](retention.md#game-room-magic-mini-games).
- [ ] Temporary events and their levers, see [Temporary events](retention.md#temporary-events).
- [ ] Weekly challenge and its rewards.
- [ ] Recycling of duplicates, if it comes back.
- [ ] Sets outside the catalogue.

## GUI

The stack is server-rendered `html/template`, see
[ADR-0034](adr/0034-the-web-client-is-server-rendered.md). Run it with
`crackapack play --demo`.

- [x] Open the ten welcome boosters and reveal the cards one big card at a time, with the rare held
  back for the end, and the new cards flagged. A card already seen opens big again from its
  miniature on the shelf. The session is in memory: it starts over when the
  server restarts. The card back is the Magic back, downloaded once and cached, with an in-house
  drawing (`card-back.svg`) as the fallback, see
  [ADR-0036](adr/0036-the-card-back-is-downloaded-with-an-in-house-fallback.md). The reveal script
  has no automated test, since CI has no browser: it was checked
  by driving headless Chrome by hand.
- [x] Energy: a gauge in the header, a cost per set that follows the booster's value, a recharge
  by the clock, the free welcome boosters spent first, and a refusal that says how long to wait,
  see [ADR-0038](adr/0038-the-games-energy-recharges-by-the-clock.md). Costs need `--prices`, or
  every booster costs the middle tier. Left: coins, so the coin fee above the cap and the coin
  top-up, the energy saved with the session, a gauge that updates without a reload, and the
  bonuses that add to the savings, such as the chest and the milestones.
- [x] The collection: one binder per set at `/collection`, every card but the basic lands in
  collector number order with the empty slots greyed, progress overall and by rarity, complete
  playsets, the copies held by finish, a filter on rarity, a click to see a card big, and the
  timeshifted cards in Time Spiral's binder, counted in its progress. Left: the collection's value, the card page, filters, selling the
  duplicates, and the master set (foil as slots of their own).
- [ ] The Counter, trades with NPCs, quests and the profile.
- [x] Card pictures, downloaded from Scryfall on demand and cached, see
  [ADR-0035](adr/0035-card-pictures-are-served-from-a-local-cache.md). The demo draws its own.
  Left: a cache that can be refreshed, and the two faces of a double-faced card.
- [ ] Open the browser on launch.
- [ ] A JSON API adapter, when a second client needs one, see [Architecture](architecture.md).

## Other layers (rough)

- [ ] SQLite store, behind the interface that comes with its second implementation.
- [ ] API adapter: HTTP and JSON.
- [ ] Catalogue sync. The local image cache exists, see the GUI section.
- [ ] Packaging: one binary per OS.
- [ ] Legal items still open: Scryfall terms, MTGJSON licence, code licence, README notice. See
  [Legal and data](specs-review.md#legal-and-data).
