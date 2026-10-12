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
catalogue.

**Build order**, so that each step is playable and can be tested before the next:

1. Sync, boosters, energy and opening.
2. Collection: binders, progress, milestones, playsets.
3. Coins: ledger, Counter, daily reward, quests.
4. NPCs and trades.

**What we measure (for ourselves):** does the economy hold? The player's money supply, the
Counter's stock levels, the trade volume per day. The simulation MVP already runs the boosters,
energy, collection, ledger, Counter and daily reward headlessly and sweeps them across coin
rates; sync, milestones, quests, NPCs and trades are not built. See
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

Once the 12-energy tier binds, the coins a booster pays per unit of energy grow with its value:
×1.1 at $19 (The Dark), ×2.7 at $48 (Stronghold), ×3.1 at $70 (Revised), ×8.2 at $162 (Legends)
and ×65 at $1,080 (Alpha), each against the 2005-2007 sets.

The five V1 sets tested are worth $5 to $13 per booster and get a tier of 5 or 8.

### Which sets can be simulated

- **Before Mirage:** none loads as downloaded. MTGJSON gives these sets a `booster.default`
  definition and no `booster.draft`, so the loader stops with `data.booster.draft: missing or empty`.
  - Checked: LEA, LEB, 2ED, 3ED, 4ED, ARN, ATQ, LEG, DRK, FEM, ICE, HML, ALL.
  - Their cards only use common, uncommon and rare, so the rarity check is not the obstacle.
- **Mirage to Tenth Edition:** all 33 expansions and core sets checked, from MIR to 10E, load as
  downloaded and ran alone for 90 days. Portal, Chronicles and promo sets were not checked.
- **Test workaround, no code change:** copy the file and expose `default` as `draft`. The 12
  pre-Mirage sets other than ALL ran this way, in a directory outside the repository.

```
jq '.data.booster = {draft: .data.booster.default}' LEA.json > old-sets/LEA.json
```

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
- **Loader support:** the least invasive way to make pre-Mirage sets loadable is to accept
  `booster.default` when `booster.draft` is absent. It is not built, and no ADR is written until a
  remedy for the faucet is chosen.

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

- [ ] Ninth and Tenth Edition in V1?
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
  [Expensive old sets and the 12-energy cap](#expensive-old-sets-and-the-12-energy-cap).
- [ ] Default number of NPCs and archetype split.
- [ ] Initial Counter stock: which cards?
- [x] Format and location of the configuration file (YAML, passed via `--config`; see [ADR-0002](adr/0002-yaml-config-format.md)).
- [ ] Jokers and duplicate protection.
- [ ] When do live prices replace the snapshot, if ever?
