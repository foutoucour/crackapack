# TODO

What is left to build, as of 2026-10-13. The economy simulation is the detailed part. This page
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
- [x] Trades with NPCs in the simulation, see [Trades](simulation.md#trades).

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
  [Installation and saves](core-features.md#installation-and-saves). The engine grants only the
  coins.
- [ ] **Coin sinks.** Coins have nothing to buy once the binder is done, and the pile after a full
  binder was 330,000 to 1 million coins in the runs. Sinks were deferred until trades or cosmetics
  exist. Choose them, see [Economy: risks and fixes](specs-review.md#economy-risks-and-fixes).

### Calibration and decisions

The tuning system below is meant to settle most numeric values here. The decisions that stay manual
are the goals and their bands, the scope, and the remedies that change a rule.

- [ ] **Product targets** for the sweep. The window behind
  [ADR-0006](adr/0006-coin-rate-is-100-per-dollar.md) uses the demo's targets, not targets the
  maintainers chose.
- [ ] **NPC defaults:** population, archetype split, rhythms and keep thresholds. A first run on
  RAV and TSP with the defaults gave about 3,400 boosters a day at 500 NPCs and millions of cards
  in the Counter after 90 days. With the Counter starting empty, 50 NPCs were enough to match the
  default-stock pace. One seed, one dataset.
- [ ] **A lever that keeps the Counter's stock scarce.** With a crowd of that size, "limited stock"
  no longer limits what a player can buy, which weakens the fix for "everything can be bought at the
  Counter".
- [ ] **Initial Counter stock:** which cards, instead of a flat count per rarity.
- [ ] **Counter rates:** the 60% and 120% are starting values.
- [ ] **Energy:** calibrate the cost tiers against the real booster values, and check the
  collector's `collectorReserveDays = 1` and `sessions_per_day` assumptions.
- [ ] **Pacing target** of a set's binder in about two weeks at 3 visits a day. The simulation gives
  8 days for RAV and TSP, so decide whether to slow it, see
  [Open questions](specs-review.md#open-questions).
- [ ] **Remedy for the 12-energy cap.** The priciest old sets create too many coins per energy, see
  [Expensive old sets](specs-review.md#expensive-old-sets-and-the-12-energy-cap). No remedy is
  chosen, so no ADR is written. Accepting `booster.default` when `booster.draft` is absent would
  make pre-Mirage sets loadable.
- [ ] **Reference value of a card with no TCGplayer price.** Today it is `fallback_price`.
- [ ] **Foil prices.** A foil shares its non-foil's price, a documented simplification pending a
  per-finish source.
- [ ] **Duplicate protection and jokers:** commons and uncommons only, or rares once the set is
  complete, see [Lessons from MTG Arena](retention.md#lessons-from-mtg-arena).
- [ ] **Scope:** Ninth and Tenth Edition in V1, and a first economy run on Planar Chaos and Future
  Sight, which no recorded run covers.
- [ ] **When live prices replace the snapshot**, if ever.

### Tuning system and metrics

- [ ] **Tuning system:** search many parameters at once and find, from simulation data, configs
  that give a good economy. A run is scored by a loss, and the search lowers it, the way training
  lowers a loss. The design below is proposed and not yet approved. Today the sweep varies only the
  coin rate, uses one seed and judges runs pass or fail, which cannot rank two configs.
  - [ ] **Goals and bands**, a manual decision. Each goal is a band on a metric, reusing the
    `between`, `at_most` and `at_least` targets:
    - [ ] Pace per set: a binder in about two weeks at 3 visits a day and about one week at 6.
    - [ ] No runaway coins: the idle pile after a full binder and the money supply stay bounded.
    - [ ] Healthy Counter stock: rares neither dry up nor pile up.
    - [ ] Fair prices at small scale: no sub-coin cards, and a sensible number of commons for the
      daily reward.
  - [ ] **Search space file:** parameters addressed by their path in the config, such as
    `counter_buy_pct` or `npcs.archetypes.casual.keep_copies`, each with a range, a type and a
    scale. A candidate is the base config with those paths overridden, then loaded through the
    normal validation. Invalid combinations score worst. Every config key added later can be tuned
    without new code.
  - [ ] **Scenarios:** a player policy, a visit rate and a set list. Pace needs the 3 and 6 visit
    rates, and each goal names the scenarios it applies to.
  - [ ] **Loss:** the distance outside each band, normalised by the band's size, averaged over
    seeds and summed. A loss of 0 means every goal is met.
  - [ ] **Counter stock metrics:** the share of rare printings out of stock, and the average rare
    stock per printing. Neither exists today, and their bands are a manual decision.
  - [ ] **Parallel evaluation** across cores, reproducible from a search seed.
  - [ ] **Search:** staged random search. A cheap stage on one seed keeps the best fraction, and the
    full batch of seeds, policies and sets scores the survivors.
  - [ ] **Validation:** re-score the best configs on seeds the search never used and report the gap
    between the two scores, so a lucky seed or one play style cannot win.
  - [ ] **Report:** the best configs, the feasible window of each parameter, which parameters move
    the loss, every evaluated candidate in JSON, and the best config as YAML that `simulate` and
    `sweep` accept.
  - [ ] **Demo:** a small search space ships with the feature, see
    [Demo data ships with the feature](../.claude/skills/crackapack-conventions/SKILL.md#demo-data-ships-with-the-feature).
  - [ ] **Adaptive search:** an evolution strategy that starts from the survivors, once random
    search runs out of reach.
  - [ ] **New goals** for quests, milestones and trades as those mechanics land. The system does
    not need them to start.
- [ ] A metric for rare stock-outs, for example the days a card the player wants is out of stock.
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

## GUI (rough)

- [ ] The web client, embedded in the binary and served on localhost: opening a booster, binders,
  the Counter, trades, quests and the profile. The stack, htmx with `html/template` or Svelte, is
  not decided. It needs the API adapter first, see [Architecture](architecture.md).

## Other layers (rough)

- [ ] SQLite store, behind the interface that comes with its second implementation.
- [ ] API adapter: HTTP and JSON.
- [ ] Catalogue sync and the local image cache.
- [ ] Packaging: one binary per OS.
- [ ] Legal items still open: Scryfall terms, MTGJSON licence, code licence, README notice. See
  [Legal and data](specs-review.md#legal-and-data).
