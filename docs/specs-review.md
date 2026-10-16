# Specs review

## Positioning and decisions

crackapack began as an online game with no NPCs. It is now a local, solo application: relive the
feeling of opening 2005-2007 boosters, with a crowd of NPCs to trade with. The code is published on
GitHub.

| Topic | Decision |
| --- | --- |
| Format | Solo, local, no server or accounts; code on GitHub |
| Interactions | The Counter (fixed prices) and trades with a crowd of NPCs (300 to 1,000) that have real collections |
| Removed | Teams, binder contests, Discord, moderation, multiplayer |
| V1 catalogue | Ravnica, Guildpact, Dissension, Coldsnap, Time Spiral, Planar Chaos, Future Sight |
| Expensive sets | A set whose booster is worth $60 or more is not loaded into the game, even though it is in the catalogue. They break the economy and need their own design, see [Expensive old sets and the 12-energy cap](#expensive-old-sets-and-the-12-energy-cap) and [ADR-0029](adr/0029-a-set-with-a-booster-worth-60-or-more-is-not-loaded.md) |
| Stack | A Go core in a single binary, see [ADR-0001](adr/0001-go-for-the-core.md) |
| Language | Everything in English |
| Name | crackapack |
| Prices | A price snapshot to start with, not live daily prices. Price changes alter the inputs of a seeded simulation, so a snapshot keeps runs reproducible and the economy tunable |
| NPC coins | NPCs hold no coins. Without auctions they have nothing to spend on, and budgets would be machinery with no effect. Trades are card swaps, with a coin top-up from the player when needed |
| NPC trades | An NPC never gives more reference value than it receives. Archetypes differ only in the margin they demand, which is never negative. A negative margin would let the player inflate a collection's value for free, by trading through a chain of NPCs and selling the result to the Counter |
| Basic lands | Excluded from binder totals, progress and milestones. They still appear in packs as filler. Ravnica and Time Spiral basics only come as foils, so counting them would make a complete binder nearly impossible |

## Fixed Counter instead of an auction market

The first design had an auction market where NPCs bid against the player. It is replaced by the
Counter, which buys and sells at fixed prices.

- **No price formation:** NPC valuations are anchored to TCGplayer prices with bounded noise. One
  player barely moves a 500-NPC market, so an auction would not set prices, it would only add
  noise around the reference.
- **Cost:** an auction house needs timers, locked coins, outbid notifications and anti-sniping, for
  little decision-making.
- **Online heritage:** the 5% seller tax, the listing fee and the 10-listing cap existed to police a
  market between real players.
- **Consequence for the Counter:** its reference value is the TCGplayer price at the fixed coin ↔
  USD rate. The "minimum of the 30-day median and the recent price" rule and the daily sales cap
  existed only to stop market-to-Counter arbitrage, and are dropped.

The auction design is kept as a deferred section in [Core features](core-features.md#deferred-auction-house).

Recycling duplicates into coins is dropped for now. It overlapped the Counter: both turn a
duplicate into coins, and the flat recycling value only beats the Counter's 60% for the cheapest
cards. If it comes back, it should pay something the Counter does not, such as Arena's vault
progress.

## V1 scope and build order

V1 validates the loop open → sort → sell to the Counter or trade with NPCs.

**In V1**

- Catalogue sync and local image cache.
- Energy (reserve, savings, cost relative to the catalogue).
- Boosters of the 7 sets, with the real compositions including the Time Spiral block's special
  slots.
- Binders, progress, milestones, playsets.
- The Counter, the coin ledger, the daily reward and simple quests.
- NPCs: generation, step-by-step simulation, offline catch-up.
- Trades with NPCs.

**Not in V1:** auction house, recycling, mini-games, events, weekly challenge, sets outside the
catalogue, and the expensive sets, those whose booster is worth $60 or more.

**Build order**, so that each step is playable and can be tested before the next:

1. Sync, boosters, energy and opening.
2. Collection: binders, progress, milestones, playsets.
3. Coins: ledger, Counter, daily reward, quests.
4. NPCs and trades.

**What we measure (for ourselves):** does the economy hold? The player's money supply, the
Counter's stock levels, the trade volume per day. The simulation MVP already runs the boosters,
energy, collection, ledger, Counter and daily reward headlessly and sweeps them across coin
rates. An NPC crowd also runs there, supply only: it opens boosters and sells its surplus to the
Counter. Sync, milestones, quests, trades, NPC wish lists and offline catch-up are not built. See
[Simulation guide](simulation.md).

## Contradictions to fix

Moving to solo local, and then to a fixed Counter, settles eight of nine; one stays open.

- [x] **"Collection tool, not a game":** moot, the app is an explicit solo game.
- [ ] **Jokers:** the retention doc still mentions them (Snap and Arena sections). Decide.
- [x] **Booster size:** V1 is limited to 15-card boosters from 2005-2007. Revisit for every set
  added.
- [x] **Arbitrage:** the Counter's reference is a single price, and there is no market to arbitrage
  against.
- [x] **Universes Beyond:** moot in V1.
- [x] **Recycling vs Counter:** recycling is dropped for now, see above.
- [x] **Contest vote paid in energy:** contests removed.
- [x] **Double-faced cards:** none in the V1 catalogue (Dissension's split cards display on a
  single image).
- [x] **Stack:** earlier drafts assumed Python and Django. The direction is Go.

## Economy: risks and fixes

Without other players, the risk is no longer cheating but a simulated economy that dries up, runs
away or makes the collection too easy.

| Risk | Fix |
| --- | --- |
| Counter stock dries up on rares | Share of casuals, NPC opening pace, initial Counter stock |
| Inflation: the player gets rich fast | Capped rewards; cosmetic coin sinks |
| Collection too easy: everything can be bought at the Counter | Purchase price at 120% or more, limited stock |
| Arbitrage | Both Counter prices come from one reference, so a round trip loses half at the default rates and never gains. NPC trades never have a negative margin |
| Commons worth nothing | Playsets, milestones, playset hunters among the NPCs |

- **Coin ↔ USD rate:** 100 coins per dollar, see [ADR-0006](adr/0006-coin-rate-is-100-per-dollar.md).
- **Tuning:** the values that shape the economy are tuned through simulation, see
  [Simulating the economy](#simulating-the-economy).

## What the sweep answers about the coin rate

The sweep runs one simulation per rate in the grid and evaluates each run against
maintainer-set calibration targets (see [Simulation guide](simulation.md#reading-the-sweep-table)).

- **Method:** every rate gets the same seed, config, catalogues and price snapshot.
  The only variable is `coin_rate`.
- **Targets are pacing targets**, not game-balance assertions. They express what
  the maintainers consider acceptable: for example, "the daily reward should buy at
  least two commons from the Counter" or "no card's sell value should be sub-coin."
- **Results depend on player policy and prices.** The same targets may produce a
  different passing range when the price snapshot is updated or a new policy is added.

- **What the rate changes.** Card prices, sale proceeds and purchase costs all scale with the
  rate, so the rate only moves two things: the weight of coin amounts fixed in the configuration
  (welcome bonus, daily reward) against card prices, and the rounding of cheap cards, which can
  fall under one coin at a low rate. On the demo dataset the binder milestone days are the same at
  every rate. Pacing is shaped by the other levers: the Counter percentages, the initial stock and
  the energy settings.

What the sweep leaves open (see [Open questions](#open-questions)):

- Product targets: the passing window behind [ADR-0006](adr/0006-coin-rate-is-100-per-dollar.md)
  uses the demo's targets, not targets the maintainers have chosen.
- Stock assumptions: the initial Counter stock and NPC opening pace are not yet
  calibrated; they affect how quickly the Counter runs out of rare copies.
- The collector's reserve rule: `collectorReserveDays = 1` is an assumption; a
  higher value slows buying and may change which rates pass.

## Expensive old sets and the 12-energy cap

The result is a decision: a set whose booster is worth $60 or more is out of V1, and the game does
not load it, see [ADR-0029](adr/0029-a-set-with-a-booster-worth-60-or-more-is-not-loaded.md). The
study below is the evidence and the starting point for the design they need.

Once the 12-energy tier binds, the coins a booster pays per unit of energy grow with its value:
×1.1 at $19 (The Dark), ×2.7 at $48 (Stronghold), ×3.1 at $70 (Revised), ×8.2 at $162 (Legends)
and ×65 at $1,080 (Alpha), each against the 2005-2007 sets.

The five V1 sets tested are worth $5 to $13 per booster and get a tier of 5 or 8.

### Which sets can be simulated

- **Before Mirage:** MTGJSON gives these sets a `booster.default` definition and no
  `booster.draft`. They load as downloaded since
  [ADR-0020](adr/0020-a-set-file-booster-is-its-draft-or-default-definition.md), which reads
  `default` when there is no `draft`.
  - Checked: LEA, LEB, 2ED, 3ED, 4ED, ARN, ATQ, LEG, DRK, FEM, ICE, HML, ALL.
  - Their cards only use common, uncommon and rare, so the rarity check is not the obstacle.
- **Mirage to Tenth Edition:** all 33 expansions and core sets checked, from MIR to 10E, load as
  downloaded and ran alone for 90 days. Portal, Chronicles and promo sets were not checked.
- **How the runs below were made:** the loader did not read `default` yet, so each pre-Mirage file
  was copied with `default` exposed as `draft`, in a directory outside the repository. The content
  is the same as what the loader now reads.

### What was run

Every figure is the mean of seeds 1 to 5, over 90 days, at 100 coins per dollar, from TCGplayer
snapshots downloaded with the `tools/fetchdata` downloader on 2026-10-11.

- **Booster value:** the engine's expected value of one booster, in dollars.
- **Tier:** the energy cost the set gets in a directory holding the five 2005-2007 sets (RAV, GPT,
  DIS, CSP, TSP) and that set. In a directory holding only the five, they get 5 or 8. The tiers
  follow the median of the directory, see
  [ADR-0008](adr/0008-energy-cost-reference-is-the-catalogue-directory.md), so a different
  directory moves them.
- **Coins per booster:** Counter sales of the `cash-out` player divided by boosters opened.
- **Solo runs at the tier above:** each set ran alone with its energy cost fixed to that tier, so
  the numbers do not depend on how a policy shares energy between sets.

| Set | Booster value | Tier (longest wait) | Coins per booster | Coins per energy | Per energy vs 2005-2007 |
|-----|--------------:|---------------------|------------------:|-----------------:|------------------------:|
| LEA Alpha | $1,079.81 | 12 (160 min) | 61,786 | 5,149 | ×65 |
| LEB Beta | $663.44 | 12 (160 min) | 36,440 | 3,037 | ×38 |
| ARN Arabian Nights | $635.92 | 12 (160 min) | 37,116 | 3,093 | ×39 |
| 2ED Unlimited | $449.94 | 12 (160 min) | 21,708 | 1,809 | ×23 |
| ATQ Antiquities | $223.06 | 12 (160 min) | 12,553 | 1,046 | ×13 |
| LEG Legends | $162.21 | 12 (160 min) | 7,793 | 649 | ×8.2 |
| 3ED Revised | $69.98 | 12 (160 min) | 2,988 | 249 | ×3.1 |
| STH Stronghold, the priciest of Mirage to 10E | $48.34 | 12 (160 min) | 2,617 | 218 | ×2.7 |
| DRK The Dark | $19.47 | 12 (160 min) | 1,041 | 87 | ×1.1 |
| ICE Ice Age | $12.82 | 8 (80 min) | 713 | 89 | ×1.1 |
| 4ED Fourth Edition | $8.79 | 8 (80 min) | 480 | 60 | ×0.8 |
| HML Homelands | $3.31 | 3 (30 min) | 191 | 64 | ×0.8 |
| FEM Fallen Empires | $3.02 | 3 (30 min) | 177 | 59 | ×0.7 |
| 2005-2007, five sets | $5.12 to $12.82 | 5 or 8 (50 or 80 min) | 291 to 704, mean 492 | 58 to 92, mean 79.5 | ×1 |

- Seed to seed, Alpha's ratio stays between ×64 and ×66.
- One Alpha booster pays about 618 daily rewards (100 coins each).
- Over 90 days the `cash-out` player ends with 31.4 million coins on Alpha and 0.36 to 0.57 million
  on a 2005-2007 set.
- The cap is flat: a $70 Revised booster and a $1,080 Alpha booster both cost 12 energy.
- Matching the 2005-2007 coins per energy would take about 780 energy for Alpha
  (61,786 ÷ 79.5), against the 210 that the reserve and savings can hold. A longer tier table
  cannot close that gap.

### What it does to the pace

The `collector` player, with the five 2005-2007 sets plus one old set, spends the Counter's coins on
missing cards. Days until the five 2005-2007 binders are complete, across 5 seeds and 5 sets:

| Added set | Median days | Range | Coins spent at the Counter |
|-----------|------------:|-------|---------------------------:|
| None | 32 | 22 to 39 | 73,749 |
| LEA | 6 | 5 to 8 | 1,403,542 |
| LEB | 7 | 6 to 9 | 1,209,854 |
| ARN | 13 | 11 to 15 | 291,913 |
| 2ED | 22 | 18 to 30 | 948,461 |
| ATQ | 26 | 21 to 28 | 144,674 |
| LEG | 30 | 25 to 56 | 437,837 |

- LEA, LEB and ARN cut the median by 59 to 81 percent, and 2ED by about a third. ATQ and LEG cut it
  by 19 and 6 percent, inside the baseline's range.
- With DRK, ICE, HML or FEM added the median is 35 to 42 days, and with 3ED or 4ED it is 49 to 52.
  Those runs are slower than the baseline because the extra set shares the same energy.
- Alone at the tier above, the same player has `coins_after_full_binder` of 25.1 million on
  Alpha, 16.0 million on Arabian Nights and 13.5 million on Beta. On a 2005-2007 set it is
  0.32 to 0.50 million.

### Limits of this test

- **Prices:** Alpha, Beta and Unlimited have rares with no TCGplayer price. The simulation prices
  them at `fallback_price` and the Counter refuses to buy them (`sub_coin_cards` equals
  `unpriced_cards`): 101 cards on Alpha (96 are rares), 62 on Beta, and 4 rares on Unlimited, among
  them Black Lotus, Mox Jet and Mox Sapphire. Their booster values are lower bounds.
- **Layouts:** the pre-Mirage `default` definitions are taken as MTGJSON gives them, and their
  fidelity to the real boosters was not checked. ARN, ATQ, DRK, FEM and HML boosters hold 8 cards,
  and V1 is limited to 15-card boosters, see [Contradictions to fix](#contradictions-to-fix).
- **Not covered:** coin rates other than 100, NPC trades (not built), any remedy, sets outside the
  46 checked here, and the V1 sets Planar Chaos and Future Sight, which the 2005-2007 data lacks.

## What the NPC crowd and trades change

A sensitivity run of the NPC crowd and of trades, one parameter at a time. It tells which settings
are worth tuning and which can stay at their defaults. The defaults it starts from are guesses, see
[NPC crowd](simulation.md#npc-crowd).

### How the sensitivity run was set up

- **Data:** the RAV and TSP sets with TSB as a supplementary pool, from the TCGplayer snapshot
  downloaded on 2026-10-13 with `tools/fetchdata`, over 90 days at 100 coins per dollar.
- **Baseline:** the `trader` player, 300 NPCs, trades enabled, every other setting at its default.
- **Seeds:** every figure is the mean of seeds 1 to 8. A setting changes the random draws that
  follow it, so one seed per setting cannot tell an effect from noise.
- **Noise:** across seeds the day of the full RAV binder moves by about 2 days (standard deviation)
  and the final balance by about 3 percent. Read a difference below that as no effect.
- **Pace metrics:** days until every common and uncommon of RAV is owned, and days until its whole
  binder is. Both are reported for RAV. The player shares its energy with TSP.

### What trading does

| Player | Days to all RAV commons and uncommons | Days to the RAV binder | Final balance |
|--------|--------------------------------------:|-----------------------:|--------------:|
| `collector` | 10.6 | 16.8 | 435k |
| `trader` | 7.5 | 14.5 | 425k |

- Trading saves about 3 days on commons and uncommons and about 2 on the binder.
- It costs coins: the player's top-ups average about 7k a run. Over 4 seeds, coins plus collection
  value ends about 2 percent below the `collector`'s.
- The `exploiter` finds no gain. Its coins plus collection value ends about 20 percent below the
  `collector`'s at the default margins and about 6 percent below with 10 points added to every
  margin. This agrees with the property test described in [Trades](simulation.md#trades).

### What each parameter does

A parameter is a lever when moving it across the range tested changes the outcome by more than the
noise.

| Parameter | Range tested | Effect |
|-----------|--------------|--------|
| `sessions_per_day` | 1 to 12 | The strongest pace lever. The RAV binder takes 20, 14.5, 11.6 and 8.6 days at 1, 3, 6 and 12 sessions. The balance goes from 302k to 919k. |
| NPC `margin_pct`, points added to every archetype | 0 to 100 | The only trade setting that bites. Accepted trades fall from 332 to 252 at +30 and 175 at +100. Commons and uncommons then take 9.5 and 10.3 days, close to the 10.6 without trades. |
| `trades.max_cards_per_side` | 1 to 30 | At 1 no counter-offer happens, so accepted trades fall from 332 to 133 and the binder takes 16.5 days. From 10 up the result is flat. |
| `energy.recharge_per_tick` | 1 to 4 | Doubling it adds 10 percent to the balance and saves about 1 day on the binder. A second doubling adds little. |
| `coin_rate` | 25 to 500 | Balance and trade volume scale with it. Below 100 sub-coin cards stay unsold: 347 duplicate copies at 50 and 2,400 at 25 on two sets, and far fewer on a bigger pool, see [Do the parameter findings hold on 10 sets](#do-the-parameter-findings-hold-on-10-sets). See [ADR-0006](adr/0006-coin-rate-is-100-per-dollar.md). |
| `counter_sell_pct` | 30 to 100 | The balance scales with it. At 30 percent about 330 duplicate copies stay unsold. |
| `daily_reward` | 0 to 300 | A small effect: commons and uncommons take 9.4 days at 0 and 7.5 at the default 100, and the balance rises by about 8 percent from 0 to 300. |
| `counter_buy_pct` | 100 to 200 | Top-ups grow from 2.4k to 16k. The pace shows no clear change. |
| `trades.duplicate_keep` | 1 to 4 | At 4 no NPC makes an offer. Values 1 to 3 are close to each other. |

### What does not matter

- **`npcs.population`:** 50, 150, 300, 600 and 1,200 give the same results. On two sets NPC offers
  reach the daily cap from 150 NPCs, and at 50 they are about a third lower. A bigger pool needs
  more NPCs, see [Do the parameter findings hold on 10 sets](#do-the-parameter-findings-hold-on-10-sets).
- **`trades.max_open_offers_per_day`:** NPC offers follow the cap, with about 90 over the run at a
  cap of 1, 1,790 at 20 and 5,700 at 200. Accepted trades hardly move: 279, 332 and 347. More
  offers do not mean more trades. What bounds acceptance was not tested.
- **`offer_chance_pct`:** one half to ten times the defaults changes nothing. At zero the player's
  own proposals still reach about 240 accepted trades.
- **`npcs.surplus_hold_days` and `trades.offer_expiry_days`:** 1 to 14 days leave the pace
  unchanged.
- **`energy.savings_cap`:** 50 to 1,000 gives identical runs, because the player never fills the
  savings at 3 sessions a day. A cap of 0 is rejected, since the dearest booster could never be
  paid for.

### What the crowd does to the Counter

- **At the default stock the crowd changes nothing for a `collector`.** Runs with and without NPCs
  are identical. `initial_stock` applies per card, so even 1 copy of every card hides the crowd.
- **With an empty Counter the crowd is what feeds it.** With `initial_stock` at 0 a `collector`
  needs 49 days for the RAV binder alone and about 17 with a crowd.
- **A small crowd is enough.** At an empty stock, 25 NPCs already give 17 days, and 600 give the
  same.

### Limits of this test

- **Two sets:** RAV and TSP. Other sets have other prices, and so other top-ups and tiers.
- **One policy:** the `trader`, plus the `collector` and the `exploiter` as references. A player who
  trades differently may weigh the margin differently.
- **One axis at a time:** no interaction was tested, such as a high margin with more sessions a day.
- **Margins:** the margin test adds the same points to all five archetypes, which keeps their
  differences. It does not say which archetype matters.
- **Exploit check:** the net worth comparison used 4 seeds.
- **Offline play:** catch-up and persistence are not built, so the run assumes the player is
  present every day.

## What a bigger pool changes

The same sensitivity run, now over a growing number of sets. It answers how much a set adds to the
pace and to the economy. It also loads the core sets Magic 2010 to Magic 2015, see
[ADR-0013](adr/0013-mythic-is-a-rarity-of-its-own.md).

### How the pool run was set up

- **Method:** as in [How the sensitivity run was set up](#how-the-sensitivity-run-was-set-up), with
  these differences.
- **Data:** 18 openable sets and TSB, from TCGplayer snapshots downloaded on 2026-10-13 in several
  `tools/fetchdata` calls, merged into one `prices.json`.
- **Directory:** every run used the whole catalogue directory and varied only `--sets`. Energy costs
  follow the median of the directory, see
  [ADR-0008](adr/0008-energy-cost-reference-is-the-catalogue-directory.md), so they stayed fixed
  while the pool grew.
- **Horizon and seeds:** 365 days and seeds 1 to 5. 90 days is too short, since nothing but the
  smallest pools finishes in it.
- **Ladder:** RAV, then adding TSP, MRD, DST, 5DN, ONS, LGN, SCG, MOR, LRW, M10 to M15, and last 9ED
  and 10E. The pools hold 1, 2, 5, 9, 10, 16 and 18 sets.

### What a set adds to the pace

The `collector`, with 3 sessions a day and no crowd:

| Sets | RAV commons and uncommons (days) | RAV binder (days) | Sets with commons and uncommons done |
|-----:|---------------------------------:|------------------:|-------------------------------------:|
| 1 | 4.4 | 8.6 | 1 of 1 |
| 2 | 10.6 | 16.4 | 2 of 2 |
| 5 | 22.4 | 40.6 | 5 of 5 |
| 9 | 39.2 | 79.4 | 8.4 of 9 |
| 16 | 60.6 | 120.6 | 15.4 of 16 |
| 18 | 57.8 | 132.4 | 15.2 of 18 |

- Each added set costs about 3.7 days of commons and uncommons and about 7.5 days of binder,
  averaged from 1 to 16 sets. The 18-set row is not slower than the 16-set one for RAV's commons and
  uncommons, which is within the seed noise.
- The player opens the sets in turn, so the sets of a pool take longer in step. Energy is the likely
  limit, but this run did not test it.
- RAV with TSP took the same time in the full directory as in the earlier directory of three files,
  10.6 days for commons and uncommons and 16.4 against 16.8 for the binder. The sets added to the
  median did not move their tiers enough to show.
- Sets of a pool differ. In one 18-set `trader` run, commons and uncommons finished on day 36 to 75
  for M10 to M15, on day 50 for RAV, and on day 95 and 110 for SCG and MOR. The tiers may explain
  it, this was not checked.
- The final balance does not grow with the pool: 2.27M with one set and 1.79M with sixteen.

### Trading in a bigger pool

The `trader` with 300 NPCs and trades enabled, against the `collector` of the table above:

| Sets | Commons and uncommons, collector then trader (days) | Binder, collector then trader (days) | Accepted trades |
|-----:|----------------------------------------------------:|-------------------------------------:|----------------:|
| 2 | 10.6, 8.2 | 16.4, 14.6 | 342 |
| 5 | 22.4, 21.0 | 40.6, 37.8 | 624 |
| 10 | 44.6, 40.4 | 90.2, 86.2 | 1,087 |
| 16 | 60.6, 52.4 | 120.6, 105.8 | 1,737 |

- Trading saves time at every size, between 5 and 25 percent. The seed noise is 2 to 7 days, so the
  runs cannot tell whether the saving grows or shrinks with the pool.
- Accepted trades grow with the pool, because the run lasts longer and more cards are in play.
- NPC offers stay at the daily cap, as in
  [What does not matter](#what-does-not-matter).

### Mythics and the Counter

- The Counter starts with 180 mythic copies, 2 of each of the 90 mythic cards. A `collector` without
  a crowd draws that down to 109 over a year, so buying a mythic works.
- With 300 NPCs the Counter's stock only grows. After a year it holds 7.1 million commons, 1.8
  million uncommons, 446 thousand rares and 18 thousand mythics, in one 16-set `trader` run. The
  Counter does not run dry, which is the same finding as in
  [What the crowd does to the Counter](#what-the-crowd-does-to-the-counter).

### The star printings of 10E, 9ED and ONS

Some catalogue files hold a second printing of a card, numbered with a star, that exists as a foil
only and has its own UUID. `1★` sits next to `1`, with the same name.

- Those printings sit on the foil sheets only. They count towards the binder, and a booster almost
  never opens one.
- 10E has 85 commons and uncommons of this kind and 9ED has 220. ONS has one, Embermage Goblin
  200★.
- 10E and 9ED did not finish their commons and uncommons in any of the five 365-day runs of the
  18-set pool, although the player ended with more than a million coins. ONS did not finish in four
  of five, stuck at 220 of 221.
- Their commons-and-uncommons totals are inflated. 10E counts 329 cards for 242 distinct names.
- [ADR-0007](adr/0007-catalogues-count-only-obtainable-cards.md) drops the cards no sheet holds. These
  are on a sheet, so it keeps them.
- A rule for these printings is not decided, see [TODO](todo.md#calibration-and-decisions).

### Do the parameter findings hold on 10 sets

The first-round sweeps ran on two sets. The key ones were repeated on a pool of RAV, TSP, MRD, DST,
5DN, ONS, LGN, SCG, MOR and LRW, with the `trader`, 300 NPCs, trades enabled, 180 days and seeds 1
to 5. On this pool the `collector` takes a median of 47.6 days for its commons and uncommons and
the `trader` 39.4, with 1,064 accepted trades. The median is over the sets of the pool, so a set that
does not finish, such as ONS with its star printing, does not move it. See
[The star printings of 10E, 9ED and ONS](#the-star-printings-of-10e-9ed-and-ons).

What holds:

| Finding on two sets | On 10 sets |
|---------------------|------------|
| NPC margin is the one trade lever | Accepted trades fall from 1,064 to 996, 767 and 433 at +10, +30 and +100 points. |
| The daily offer cap does not change accepted trades | 1,050, 1,064 and 1,105 at caps of 5, 20 and 200, while NPC offers go from 888 to 9,082. |
| Population above 50 does not change accepted trades | 1,008 at 50 NPCs and 1,065 at 1,200. |
| Trading saves time | The median set finishes 17 percent sooner than with the `collector`. |
| `sessions_per_day` is the strongest pace lever | A median of 50.8, 39.4 and 32.4 days at 1, 3 and 6 sessions, in the same ratios as on two sets. |
| Doubling `energy.recharge_per_tick` is a small gain | 40.0 days against 39.4, inside the noise. |

What changes with the pool:

- **Per-side cap:** a cap of 1 cuts accepted trades by 77 percent, from 1,064 to 240, and a cap of 3
  by 66 percent. On two sets the cuts were 60 and 46 percent. A bigger pool needs bigger trades.
- **Who trades:** with no NPC offers at all accepted trades fall only 9 percent, to 963, where they
  fell 27 percent on two sets. The player's own proposals carry more of the trading.
- **Crowd size:** at 50 NPCs offers are 7.5 a day, against 18.8 at 300, the daily cap. The crowd
  that saturates the cap grows with the pool.
- **Sub-coin cards at low rates:** at a coin rate of 50 unsold duplicate copies fall from 347 on two
  sets to 9 on ten, and at 25 from 2,400 to 1,555. Rate 25 is still clearly bad. A likely reason
  is that boosters spread over more sets leave fewer duplicates, which was not tested.
- **Pace across rates:** the median is 34.2 days at rate 25 and 44.2 at rate 250, with 41.0 at 50 and
  39.4 at 100. The fixed daily reward buying fewer cards at a higher rate would explain it, see
  [What the sweep answers about the coin rate](#what-the-sweep-answers-about-the-coin-rate). The
  first round showed the same direction.
- **Counter sell percentage:** at 30 percent the balance falls to 234k, and the unsold duplicates
  vary too much between seeds, 105 with a deviation of 206, to give one figure.

### Limits of the pool run

- **Parameter sweeps on one larger pool:** the 10-set pool above, without mythics. They were not
  repeated on 16 or 18 sets.
- **One player policy at a time:** the `collector` and the `trader`. The `exploiter` was not run.
- **Five seeds, one sessions rate:** 3 sessions a day only.
- **Crowd size:** 300 NPCs. A larger crowd on a large pool was not tested.
- **Cause of the linear pace:** energy is assumed, not tested.

## What the archetype mix changes

The five archetypes differ in how many boosters they open, how many copies they keep, what they wish
for and what margin they demand, see [NPC crowd](simulation.md#npc-crowd). This run changes the share
of each in a crowd of 300 and leaves the archetypes themselves as they are.

### How the archetype run was set up

- **Method:** as in [How the sensitivity run was set up](#how-the-sensitivity-run-was-set-up), with
  these differences.
- **Pool:** RAV, TSP, MRD, DST and 5DN, with the full catalogue directory, so the energy tiers match
  [the pool run](#how-the-pool-run-was-set-up).
- **Players and horizon:** the `trader`, 300 NPCs, trades enabled, 120 days and seeds 1 to 5.
- **Mix:** the shares of the five archetypes. A share of zero removes an archetype. The default is
  50, 15, 20, 10 and 5 percent for casual, playset hunter, targeted collector, speculator and whale.

### What the mix does

| Mix | Median days, commons and uncommons | Accepted trades | NPC offers | Boosters the NPCs opened | Final coins |
|-----|-----------------------------------:|----------------:|-----------:|-------------------------:|------------:|
| No crowd | 23.8 | 0 | 0 | 0 | 573k |
| Default | 20.4 | 593 | 2,348 | 239k | 583k |
| Uniform, 20 percent each | 21.2 | 579 | 2,280 | 410k | 569k |
| No whales | 21.4 | 607 | 2,342 | 178k | 564k |
| 40 percent whales | 23.2 | 598 | 2,287 | 687k | 586k |
| Casual only | 19.2 | 619 | 2,373 | 216k | 581k |
| Targeted collector only | 22.6 | 551 | 1,701 | 125k | 588k |
| Whale only | 20.6 | 586 | 466 | 1,441k | 588k |
| Speculator only | 23.0 | 275 | 443 | 109k | 555k |
| Playset hunter only | 24.0 | 48 | 2,323 | 163k | 587k |

- The mix barely moves the pace or the balance. Medians fall between 19 and 24 days, and the seed
  noise is 2 to 4 days.
- The boosters the NPCs open range from 109 thousand to 1.44 million, and the cards they send to the
  Counter from 1.0 million to 20.8 million. The player's outcome does not change, which agrees with
  [What the crowd does to the Counter](#what-the-crowd-does-to-the-counter).
- Accepted trades stay between 550 and 620 for most mixes. Two archetypes trade less.

### Why hunters and speculators trade less

The two archetypes also differ in margin, offer chance and keep threshold, so each was changed
alone:

| Variant | Accepted trades |
|---------|----------------:|
| Hunters, keep threshold 4 (default) | 48 |
| Hunters, keep threshold 1 | 602 |
| Speculators, keep threshold 2 (default) | 275 |
| Speculators, margin 0 instead of 15 | 292 |
| Speculators, offer chance 2 instead of 1 | 274 |
| Speculators, keep threshold 1 | 509 |

- **The keep threshold is the lever.** An NPC shelves only the copies above it, see
  [ADR-0011](adr/0011-npcs-hold-surplus-on-a-shelf.md), so a hunter keeping four copies has almost
  nothing to trade. At a threshold of 1 hunters trade as much as the other archetypes.
- **Margin and offer chance do nothing for speculators.** Doubling their offer chance leaves their
  offers at 438 against 443. Their offers follow what they wish for, which is the dearest cards, and
  the player rarely holds those twice. This reading of the offers was not tested.
- **The hunter explanation is partly inferred** from the shelf rule and the result at a threshold of
  1. The shelf code was not read for this run.

### Limits of the archetype run

- **Shares only:** boosters per day and wish size were not varied one at a time.
- **One pool:** five sets, without mythics, 9ED or 10E.
- **One policy and five seeds:** the `trader` over 120 days.
- **A default to review:** in the defaults a playset hunter is nearly a non-trader, see
  [TODO](todo.md#calibration-and-decisions).

## Simulating the economy

Some risks only show after weeks of play: the collection's value growing through trades, or the
Counter's stock running dry. Playing is too slow to tune them. The project needs a way to measure the
impact of a change before it ships.

- **Headless:** it runs the real domain without UI, API or real time. Time advances by ticks, which
  the domain allows because it has no clock and no global randomness, see
  [Architecture](architecture.md#layers).
- **Deterministic:** a run is fully set by the seed, the configuration and the price snapshot. Two
  runs then differ only by what we changed.
- **Configuration-driven:** every tunable value is configuration, not code. That covers the NPC
  archetype margins, the Counter's rates, the NPC population and activity rhythm, the energy costs
  and the rewards. The format and location of the configuration are not decided yet. The starting
  values in the other docs are the defaults.
- **Simulated player:** coin flows and trades need a player, so the simulation plays one. Player
  behaviours are interchangeable, to cover different play styles. One of them hunts for exploits,
  for instance by chaining trades across NPCs.
- **Comparable output:** a run reports metrics over time, and two runs on the same seed can be
  compared, typically a baseline against a changed configuration.
  - The player's money supply, by flow.
  - The Counter's stock by rarity.
  - Trades proposed and accepted.
  - The player's collection value over time.
  - Days to reach each milestone.
  - The size of the NPC collections.
- **Run without a player:** over 90 days it checks supply only, meaning the Counter's stock and the
  NPC collections. No coins move without a player.

Questions left to simulation:

- The margin of each NPC archetype.
- The coin ↔ USD rate.
- The default number of NPCs and the initial Counter stock.
- Quest, milestone and mini-game rewards.
- Whether a new basic land counts for the "discover 10 new cards" quest.

## Legal and data

Without a server or personal data, only the Fan Content framework and the data licences remain.

- [ ] **Scryfall terms:** check that a local tool that downloads images and prices is allowed.
- [x] **MTGJSON booster coverage:** all seven V1 sets have a draft booster definition, see
  [Core features](core-features.md#boosters-and-draw-rates).
- [ ] **MTGJSON licence:** check it.
- [ ] **Code licence** to choose.
- [ ] **README:** Fan Content Policy notice, and nothing from Wizards, Scryfall or MTGJSON in the
  repository.
- [ ] **Delivery:** how the client gets the catalogue and the price snapshot, directly from MTGJSON
  and Scryfall or through a published copy. A published copy would need the Scryfall redistribution
  terms checked first.
- [x] GDPR, Law 25, minimum age, terms of service: moot (no data collected).
- [x] Accounts and password recovery: moot (no account).

## Technical

The most uncertain item is the NPC simulation's behaviour at scale. Hosting no longer exists.

| Topic | Need |
| --- | --- |
| Simulation | 10-minute step, catch-up capped at 48 h, batch processing, runs in the same process while the app is open |
| NPC collections | Stored as quantities; index on (printing, NPC) for "who has it as a duplicate" |
| Coin ledger | Double-entry, immutable journal, player and Counter, explicit accounts for sources and sinks |
| Data sync | The sync adapter, see [Architecture](architecture.md#layers); handle ID changes |
| Images | Local cache, downloaded on demand or in a batch for the 7 sets |
| Database | SQLite |
| Saves | Copy of the SQLite file, export/import |
| Reproducibility | Stored random seed and the price snapshot in use |

## Open questions

- [ ] Ninth and Tenth Edition in V1? Both hold star printings that block their binders, see
  [The star printings of 10E, 9ED and ONS](#the-star-printings-of-10e-9ed-and-ons).
- [ ] Reference value of a card with no TCGplayer price.
- [ ] Pacing target: a set's binder in about two weeks at 3 visits a day and about one week at 6.
  The simulation gives 8 days for RAV and TSP at 3 visits and 6 days at 6. With about 150 sets, the
  target is per set, since finishing every set takes years.
- [x] Energy cost tiers are relative to the median booster value of the whole catalogue, not of the
  sets in a run or what the player has unlocked; see
  [ADR-0008](adr/0008-energy-cost-reference-is-the-catalogue-directory.md).
- [x] Booster energy cost stops at 12 while booster values vary widely. Do very expensive sets create
  too many coins through the Counter? Yes for the priciest sets: ×2.7 per energy at $48 a booster,
  ×65 for Alpha, and negligible below about $20. See
  [Expensive old sets and the 12-energy cap](#expensive-old-sets-and-the-12-energy-cap). Those sets
  are left out of V1 from $60 a booster, see
  [ADR-0029](adr/0029-a-set-with-a-booster-worth-60-or-more-is-not-loaded.md).
- [ ] Default number of NPCs and archetype split. Above 50 NPCs the population does not change the
  pace, see [What the NPC crowd and trades change](#what-the-npc-crowd-and-trades-change).
- [ ] Initial Counter stock: which cards? At 1 copy of every card the crowd no longer matters to a
  `collector`, see [What the crowd does to the Counter](#what-the-crowd-does-to-the-counter).
- [x] Format and location of the configuration file (YAML, passed via `--config`; see [ADR-0002](adr/0002-yaml-config-format.md)).
- [ ] Jokers and duplicate protection.
- [ ] When do live prices replace the snapshot, if ever?
