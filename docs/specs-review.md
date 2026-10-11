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

1. [x] Sync, boosters, energy and opening.
2. [x] Collection: binders, progress, milestones, playsets.
3. [x] Coins: ledger, Counter, daily reward, quests.
4. [ ] NPCs and trades.

**What we measure (for ourselves):** does the economy hold? The player's money supply, the
Counter's stock levels, the trade volume per day. Steps 1–3 are now covered by the simulation
MVP; the economy can be run headlessly and swept across coin rates. See
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
| Arbitrage | Both Counter prices come from one reference, so a round trip loses half. NPC trades never have a negative margin |
| Commons worth nothing | Playsets, milestones, playset hunters among the NPCs |

- **Coin ↔ USD rate:** to be set, the whole economy depends on it.
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

What the sweep leaves open (see [Open questions](#open-questions)):

- The actual coin ↔ USD rate to ship: the targets use a synthetic price snapshot;
  real prices will change which rates pass.
- Stock assumptions: the initial Counter stock and NPC opening pace are not yet
  calibrated; they affect how quickly the Counter runs out of rare copies.
- The collector's reserve rule: `collectorReserveDays = 1` is an assumption; a
  higher value slows buying and may change which rates pass.

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
- [ ] Fixed coin ↔ USD rate (the sweep narrows the range; real prices needed to confirm).
- [ ] Reference value of a card with no TCGplayer price.
- [ ] Default number of NPCs and archetype split.
- [ ] Initial Counter stock: which cards?
- [x] Format and location of the configuration file (YAML, passed via `--config`; see [ADR-0002](adr/0002-yaml-config-format.md)).
- [ ] Jokers and duplicate protection.
- [ ] When do live prices replace the snapshot, if ever?
