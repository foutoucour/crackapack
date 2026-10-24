# Tuning log

The economy's config changes often. This log records each change to its values, or each search that
proposed one, with what the change did to the goals. It is a history: an ADR records a decision that
holds until it is superseded, and this log records values that the next search will replace.

An ADR names a goal and its metric and leaves out the numbers of its band. The ADRs accepted before
this rule, such as ADR-0041 and ADR-0043, still carry the numbers they were accepted with. The current bands are in
[`tune/goals.yaml`](../tune/goals.yaml), and every change to one is an entry here.

How the search works is in [The tuning system](simulation.md#the-tuning-system). The goals are the bands
of ADR-0051, ADR-0041, ADR-0049 and ADR-0043, and the search files are in [`tune/`](../tune).

**A change to `initial_stock` reaches existing saves.** The game saves the Counter's stock as its
difference from the configured initial stock, see
[ADR-0052](adr/0052-the-game-is-saved-in-one-versioned-json-file.md). Raising or lowering an initial
stock moves the Counter of every save already written by the same amount.

**Add an entry** whenever a default of the config changes, a search runs on the product's files, or a
goal band changes. Newest first. Each entry says:
- what triggered it and whether the values were applied to `config.defaults()`;
- the keys that changed, before and after;
- how the goals read before and after, on the same scenarios, so the entries compare;
- what the run does not tell.

## 2026-10-24 – Milestones pay energy, no search run

**Trigger:** the game gets milestones, see [Milestones](core-features.md#milestones). The values below
are the maintainers' choice, in `config.defaults()`. No simulation plays milestones, so the goals are
unchanged and unmeasured.

Each reward is a share of `energy.reserve_cap`, which is 10, so the energy is the share divided by ten.

| Key | Share of the bar | Energy at a bar of 10 |
|-----|-----------------:|----------------------:|
| `milestones.common_pct` | 50 | 5, a booster of a regular set |
| `milestones.uncommon_pct` | 100 | 10, a full reserve |
| `milestones.rare_pct` | 200 | 20 |
| `milestones.mythic_pct` | 400 | 40 |
| `milestones.colour_pct` | 50 | 5 |

**What it does not show:** how much the rewards shorten the pace of a binder. The rarities of a set
without a mythic pay 35 energy in all, which is seven boosters of a regular set, and 75 with a mythic.
The colour groups add 5 each on top. The pace goal of
[ADR-0051](adr/0051-the-pace-of-the-scope-is-a-bell-curve-around-two-weeks.md) was tuned without
them. A run with milestones is in the [TODO](todo.md).

## 2026-10-24 – Energy has a price in the game, and a smaller save, no search run

**Trigger:** the game gets the coin fee above the energy cap and the coin top-up, see
[ADR-0068](adr/0068-the-game-charges-a-sets-coin-fee-when-a-booster-is-opened.md) and
[ADR-0069](adr/0069-the-player-asks-for-a-coin-top-up-and-the-game-plays-one-price.md), and the daily
reward, see [ADR-0067](adr/0067-the-daily-reward-is-paid-for-each-day-the-game-catches-up.md).
`tune/play.yaml` prices energy and turns the top-up on, which needed a switch to be a key that an
override can set, as a 0 or a 1. The demo's own config prices energy too. `config.defaults()`,
`tune/base.yaml` and the search are unchanged.

| Key | Before | `tune/play.yaml` | Demo before | Demo now |
|-----|-------:|-----------------:|------------:|---------:|
| `energy.coins_per_energy` | 0 | 100 | 0 | 70 |
| `energy.coin_topup.enabled` | off | on | off | on |

- **A price of 100** is the start the [todo](todo.md) names, the best coins per energy of the tiered sets.
  On the 107 sets of the scope with `prices.json` it gives 7 sets a fee, from 56 to 1,699 coins. At 70,
  16 sets pay one, at 150 four, at 200 three.
- **The demo's 70** puts the cap's energy at 840 coins, between the cash-out value of a DEM booster, about
  740, and of a DSP booster, about 930. DSP asks a fee and DEM does not.
- The daily reward is `daily_reward`, 100 by default, unchanged.

**The save is smaller.** A copy of a real save, 12 NPCs, went from 359 KB to 134 KB when the game loaded
it and saved it again with the card dictionary of
[ADR-0066](adr/0066-a-save-names-the-cards-of-its-lists-by-a-dictionary.md): 63% less. The NPCs' lists
were 334 KB of the 359. The table of the 2026-10-23 entry below was measured before the dictionary.

**What it does not show:** what the fee and the top-up do to the pace and to the coin pile. No search
runs them, and the price was not tuned. The goals of ADR-0041 and ADR-0051 were read without them.

## 2026-10-24 – What makes a binder fast: the Counter, the crowd and the market, two values applied to the game

**Trigger:** the market of the game was thought too generous, see
[ADR-0060](adr/0060-an-npc-pays-coins-for-a-card-on-the-market.md), and no simulation ran it. The
maintainers' aim is that trading is the fast road: with boosters alone as speed 1, the Counter should
make a binder about a third faster (1.2 to 1.5), the market at least twice as fast and 1.5 times the
Counter, and no set should be done with it in under 5 days. `crackapack market-study` measured it, see
[The market study](simulation.md#the-market-study). The variants are in
[`tune/market-study.yaml`](../tune/market-study.yaml).

**Applied** to the game only, in [`tune/play.yaml`](../tune/play.yaml), the stack `supply-1-hold-28`
below. `config.defaults()`, `tune/base.yaml` and the search are unchanged, so the goals of
[`tune/goals.yaml`](../tune/goals.yaml) read as before.

| Key | Before | Now |
|-----|-------:|----:|
| `npcs.surplus_hold_days` | 7 | 28 |
| `counter_supply.boosters_per_day` | 13 | 1 |

**Measured** on the 107 sets of the scope played one at a time, `pace-3-visits` (3 visits a day, 120
days), seeds 42, 43 and 44 pooled: 321 runs of each player per variant. The speeds are the median
days of boosters alone over the median of the player, and 24% of the sets were not finished by
boosters alone in 120 days. This first table is the game before energy had a price in coins.

| Variant | Days: boosters | Counter | Market | Counter speed | Market speed | Fastest set |
|---------|---------------:|--------:|-------:|--------------:|-------------:|------------:|
| `baseline` (the game today) | 42 | 14 | 11 | 3.00 | 3.82 | 5 days |
| `supply-1-hold-28` | 42 | 33 | 12 | 1.27 | 3.50 | 5 days |
| `crowd-24-supply-1-hold-28-buy-400` | 42 | 32 | 11 | 1.31 | 3.82 | 5 days |
| `supply-1-hold-21-buy-400` | 42 | 28 | 12 | 1.50 | 3.50 | 5 days |
| `crowd-6-supply-1-hold-28-buy-400` | 42 | 36 | 23 | 1.17 | 1.83 | 7 days |
| `crowd-48-supply-1-hold-28-buy-400` | 42 | 32 | 9 | 1.31 | 4.67 | 4 days |

- **The Counter is the generous one, not the market.** Today the Counter makes a binder 3 times
  faster than boosters alone, where 1.33 is wanted. The market is already above its floor, at 3.82,
  but only 1.27 times the Counter: it is a better road than the Counter, and not by the margin wanted.
- **The NPCs are the Counter's main supply.** In a run of one set the crowd sells its surplus to the
  Counter. Alone, the Counter's own supply (13 boosters a day down to 1) brings its speed from 3.00 to
  2.47, and an NPC hold of 28 days, not 7, to 2.33. Together they bring it to 1.27: the NPCs keep the
  surplus until the Counter's own supply is all that is left. The price (`counter_buy_pct` 400)
  alone moves it as far as the hold does, to 2.33, and the Counter's sale price, its shelf life and
  the daily reward less, to 2.62, 2.80 and 2.80. Once the supply and the hold are set the price adds
  nothing: 1.27 with it and without it.
**Measured again** with the stack applied, in the game as it is now, with energy priced in coins and
the top-up on, the same scenario and seeds. 30% of the sets were not finished by boosters alone.

| Variant | Days: boosters | Counter | Market | Counter speed | Market speed | Fastest set |
|---------|---------------:|--------:|-------:|--------------:|-------------:|------------:|
| baseline (the game before this entry) | 50 | 15 | 12 | 3.33 | 4.17 | 5 days |
| `supply-1` | 50 | 18 | 12 | 2.78 | 4.17 | 5 days |
| `hold-28` | 50 | 19 | 12 | 2.63 | 4.17 | 5 days |
| `supply-1-hold-28` (applied) | 50 | 34 | 12 | 1.47 | 4.17 | 5 days |
| `crowd-24-supply-1-hold-28-buy-400` | 50 | 33 | 11 | 1.52 | 4.55 | 5 days |

The applied stack still meets every target, with the Counter's speed at 1.47 in a band that ends at
1.5, so it has little room: a game a little more generous would leave the band. The coin price of
energy moved the baseline's Counter from 3.00 to 3.33.

**The cost of the hold.** A new game warms its NPCs up for the days of the hold but one, see
[ADR-0064](adr/0064-a-new-games-npcs-have-most-of-a-hold-behind-them.md). Measured on the real binary
with every set in scope and seed 42, a new game answers its first request in 1.81 s, against 1.12 s,
and its save weighs 351 KB, against 108 KB. The save of a game weeks old was not measured: its
shelves hold four times the cards, and the 1.6 MB of a month at a hold of 7, in the entry below, will
grow.

- **Six variants meet every target, and two keys are enough.** `supply-1-hold-28` sets
  `counter_supply.boosters_per_day` to 1 and `npcs.surplus_hold_days` to 28. The stack with a crowd of
  24 has the most room on its tightest speed, 9%, and the one with a hold of 21 days sits on the edge
  of the Counter's band.
- **The market's speed follows the crowd, not the Counter.** Two NPCs give it 2.33 and six 2.80,
  against 3.82 for twelve and 5.25 for forty-eight, which is also where the fastest set falls to 3
  days, under the floor. The delays of the archetypes barely matter: answering at once gives 4.20.
- **One seed misleads.** The baseline's Counter speed was 3.50, 2.33 and 2.80 on the three seeds, and
  `supply-1-hold-28` met the Counter's band on two of them. Read the pooled numbers.
- **A first version of the study compared two worlds.** The crowd kept its surplus on a shelf only
  for the player that used the market, so its Counter was fed a week later than the others'. That
  made the market look slower than the Counter in a small crowd. All the players now play in the
  crowd of the game, see `sim.WithTradingCrowd`, and the earlier numbers were discarded.

**What it does not show:**
- **A set alone is not the game.** The crowd and the Counter's supply go into the one set, where the
  game spreads them over every set the player can open, so each is faster here than there. The
  pace bands of the product are judged the same way, see
  [ADR-0051](adr/0051-the-pace-of-the-scope-is-a-bell-curve-around-two-weeks.md).
- **The other goals.** The coin pile, the Counter's stock and the small prices, the goals of
  ADR-0041, ADR-0049 and ADR-0043, were not read with any of these variants. They are judged on
  `tune/base.yaml`, which does not carry the stack, so they read as before, and the game now plays in
  a world they do not judge. A Counter fed less by the crowd stocks fewer rares, which ADR-0049 wants
  between 10% and 50% out of stock.
- **The size of an older save.** Only a new game's was measured, see above.
- **One way of trading.** The market player asks around for the rares it lacks, lists every card it
  holds twice and takes the cheapest asking price it can pay, and never counters. A player who
  counters, or who hoards, would find another speed. Its calls are listed in
  [The market](simulation.md#the-market).
- **The fastest set.** It is a whole number of days that the pace of the energy fixes, and it sat on
  its floor of 5 days in almost every variant, so it separates few of them.

## 2026-10-23 – A trading crowd and a market for the game, no search run

**Trigger:** the game gets a market, see
[ADR-0058](adr/0058-the-player-trades-on-a-market-of-requests-and-listings.md), and the real game had
no NPCs to trade with. `tune/play.yaml` gives it a crowd, the demo's own config a bigger one, and
every archetype a response delay. `config.defaults()`, `tune/base.yaml` and the search are unchanged,
and the simulation does not read the delay.

| Key | Before | `tune/play.yaml` | Demo before | Demo now |
|-----|-------:|-----------------:|------------:|---------:|
| `npcs.population` | 0 | 12 | 3 | 12 |
| `npcs.surplus_hold_days` | 2 | 7 | 2 | 3 |

- **12 NPCs** is the smallest crowd whose archetype shares give a whale: five casuals, two targeted
  collectors, two playset hunters, a speculator, a whale and an archivist. The whale's many boosters
  fill the shelves, and the speculator makes an offer on any listed card it lacks.
- **A hold of a week** keeps a card on an NPC's shelf long enough to be asked for. At 4 days the save is
  10% smaller and the cards held at the start fall from 120 to 117 of 156.
- **The demo's 12** give the same mix, so that asking around for a common of DEM gets an answer at
  once and another later, and a listed rare draws the speculator's offer at once. A test of the `play`
  command pins it.

| Archetype | `response_delay_hours` | Why |
|-----------|-----------------------:|-----|
| `casual` | 0 | The main source of supply, answers at once |
| `speculator` | 0 | Watches the market and pounces |
| `fish` | 0 | Passive and friendly, eager to trade |
| `gambler` | 1 | Impulsive |
| `playset_hunter` | 4 | Checks its binders in the evening |
| `targeted_collector` | 8 | Answers once a day, at its own pace |
| `whale` | 12 | Busy opening boosters |
| `stingy` | 12 | A miser takes its time |
| `ego`, `ego_stingy`, `archivist` | 24 | Proud or old-fashioned: the next day |
| `archivist_ego_stingy` | 36 | Both at once |

**Measured** on the 107 sets of the scope with `prices.json`, seed 42, the clock moved by hand, through
the game's start-up code. "Held" is how many of 156 cards of the first open set some NPC has on its
shelf, so that asking around for it gets an answer.

| Days played | Save | Held | Opening a booster and saving |
|------------:|-----:|-----:|-----------------------------:|
| 0 | 306 KB | 120 | 13 ms |
| 3 | 477 KB | 135 | 13 ms |
| 30 | 1.6 MB | 143 | 24 ms |
| 90, caught up 30 days at a time | 2.0 MB | 147 | 24 ms |

- The binary starts a new game in 1.80 s, against 1.67 s without NPCs, and loads it in 1.23 s, against
  1.01 s. Catching up 30 days takes about 0.15 s.
- At 30 days, 8 NPCs save 1.4 MB and 16 save 2.0 MB.
- The save is now written without indentation. That changes little: 1,605 KB at 30 days and 1,981 KB
  at 90, against 1,609 KB and 2,047 KB, since the NPCs' cards and the Counter's shelf were compact
  strings already.
- Before the NPCs opened only the sets the player can open, see
  [ADR-0063](adr/0063-the-npcs-of-the-game-open-the-sets-the-player-can-open.md), the same crowd held 0
  of the 156 cards on the fourth day and 28 after a month, and its save was 3.0 MB after 30 days, half
  of it the Counter's shelf in its older form.

**What it does not show:** what the market does to the economy. No simulation runs it. NPCs pay up to
a card's value less their margin, against the Counter's 60%, and a casual sells at the reference value,
against the Counter's 120%, so the market is a better source of coins and a cheaper shop than the
Counter for most cards, see [ADR-0060](adr/0060-an-npc-pays-coins-for-a-card-on-the-market.md). The
coin goals of ADR-0041 and the stock goals of ADR-0049 were never read with it. Nor how the save grows
once many sets are unlocked: the NPCs then open more sets, and their collections tend toward every
printing of those sets.

## 2026-10-23 – The demo's Counter shows an item out of stock, no search run

**Trigger:** the Counter's pages say when a card was last in stock, and the demo had no way to reach
that state: a card leaves the Counter's stock only when a limited shelf lets it go, and the demo's
shelf kept cards forever. The demo's own config (`demo/config.go`) changed. `config.defaults()`, the
search and the game's files did not.

| Key | Demo before | Demo now |
|-----|------------:|---------:|
| `counter_shelf_days` | -1 | 1 |
| `initial_stock.uncommon` | 8 | 0 |

- Sell an uncommon of the demo and the Counter has it for a day, then the slot says when it last had it.
  The mythics already started out of stock, so they read "never seen".
- The goals, which the demo does not run, are unaffected. A test of the `play` command sells the
  uncommons of a booster, moves the clock two days and reads the line.

## 2026-10-23 – A scarce Counter for the game, no search run

**Trigger:** with `tune/base.yaml` alone the game's Counter holds every printing from the first day and
never loses one, so no rare is ever out of stock. The game now applies `tune/play.yaml` on top of the
base, see [ADR-0057](adr/0057-the-game-plays-the-products-base-config.md). The search and
`config.defaults()` are unchanged.

| Key | Default and `tune/base.yaml` | `tune/play.yaml` |
|-----|-----------------------------:|-----------------:|
| `counter_shelf_days` | -1 | 28 |
| `initial_stock.rare` | 2 | 0 |
| `initial_stock.mythic` | 2 | 0 |

- **The shelf of 28 days** is the value of candidate 17 in the bell-goal entry below. It is long enough
  that a card the Counter opens stays a while, and short enough that its stock does not grow forever.
- **No rare or mythic at the start:** they come only from the Counter's 13 boosters a day of
  `tune/base.yaml` and from the player's sales, so a rare is something to watch for. Commons and
  uncommons keep their initial stock, and initial stock never vanishes.
- On the demo sets with this config, no rare is in stock at the start, and some are after three days
  of the Counter's boosters. That is pinned by a test of the `play` command.
- On the 107 sets with these values, a new game starts in 1.67 s and a reload in 1.01 s.

**What it does not show:** how scarce the rares are on the 107 sets after weeks of play. With 13
boosters a day across 107 sets a given set gets about one booster in eight days, so most rares of the
scope stay out of stock for long. Nor how this Counter reads against the stock goals of
[ADR-0049](adr/0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md): no simulation ran
with the overlay.

## 2026-10-23 – Catch-up cap added for the game's Counter, no search run

**Trigger:** the game catches up the Counter's supply and NPCs on the days it was closed, see
[ADR-0055](adr/0055-the-counter-catches-up-on-whole-days-up-to-a-configured-cap.md). One key is new in
`config.defaults()`. It does not change a simulation.

| Key | Default |
|-----|--------:|
| `catch_up_days` | 30 |

The value is the maintainers' choice: a month away still shows. On the 107 sets the 30 days take 5 ms
with the supply of `tune/base.yaml`, 13 boosters a day, and 34 ms with 9 NPCs as well. The game starts
in about one second either way, most of it loading the catalogues.

The demo of `play` sets its own Counter: a fixed supply of 5 boosters a day, 3 NPCs, and no mythic in
the initial stock, so that a new demo has a card the Counter never had.

**What it does not show:** how the stock a player meets after a long absence compares with one who
plays every day. No simulation plays absences.

## 2026-10-23 – Unlock keys added for the stages of the game, no search run

**Trigger:** the game opens its sets in stages, see [Stages](core-features.md#stages). Four keys are
new in `config.defaults()`, under `unlock`. They do not change a simulation.

| Key | Default |
|-----|--------:|
| `unlock.binder_threshold` | 0.9 |
| `unlock.sets_per_unlock` | 3 |
| `unlock.stages` | 4 |
| `unlock.start_sets` | 5 |

The values are the maintainers' choice. On the 107 sets of the scope they make a first stage of 5 sets,
then four stages of 26, 26, 26 and 24. The demo of `play` has its own: two stages of one set and a
threshold of 0.25, so the welcome boosters unlock the second set.

**What it does not show:** how long a player takes to unlock a stage. No simulation plays the stages.

## 2026-10-23 – Bell goal on the 97 sets that can finish, not applied

**Trigger:** a check that the tuner moves toward the bell once the ten sets of the entry below are out of
the way. A scratch search, with a goals file of its own that is not in the repo: the 3-visit shape alone,
on the 97 sets of the scope that can finish a binder. 48 candidates over the keys of `tune/space.yaml`,
the best 12 scored on 2 seeds and the best 3 validated on 2 fresh ones.

**Status:** not applied. `config.defaults()` is unchanged, and the maintainers prefer the baseline curve.

The best candidate, number 17, would change these defaults:

| Key | Default | Candidate 17 |
|-----|--------:|-------------:|
| `counter_sell_pct` | 60 | 77 |
| `counter_buy_pct` | 120 | 103 |
| `counter_foil_pct` | 500 | 781 |
| `counter_shelf_days` | -1 | 28 |
| `counter_supply` | none | fixed, 31 a day |
| `npcs.population` | 0 | 37 |
| `initial_stock.rare` | 2 | 2 |
| `daily_reward` | 100 | 93 |
| `energy.recharge_per_tick` | 1 | 2 |
| `energy.reserve_cap` | 10 | 9 |
| `energy.cost_table[1].cost` | 5 | 7 |
| `energy.cost_table[2].cost` | 8 | 10 |

**Impact**, the `collector` at 3 visits a day for 40 days on the 97 sets, seed 0. The baseline is the
defaults with the Counter's fixed supply on, 13 boosters a day, as `tune/base.yaml` sets it.

| Reading | Baseline | Candidate 17 |
|---------|---------:|-------------:|
| Search loss, the distance from the bell | 92.3 | 80.8 (86.9 on fresh seeds) |
| Distance from the bell at seed 0 | 106.2 | 91.5 |
| Sets finishing in 10 to 18 days | 34 | 43 |
| Sets finishing under 10 days | 42 | 32 |
| Mean days, sets that finish | 12.3 | 13.2 |
| Spread (sd), sets that finish | 7.0 | 7.2 |
| Sets not finished in 40 days | 6 | 5 |

**What it shows**
- The tuner moves toward the bell. The sets move from under 10 days into the middle, and the mean goes
  from 12.3 toward 14.
- It does not reach a bell. The spread stays at 7 days against a target of 4, and five sets still take
  longer than the 40-day run: FRF, MIR, OGW, ONS and SHM on the candidate, and USG on the baseline.
- The gap between the search seeds and the fresh seeds is 6 points, so part of the gain is the seeds'
  luck.

**What it does not show:** a search with more candidates, an adaptive search, or a longer run that
would let the five slow sets finish and so read their real time.

## 2026-10-23 – Why ten sets never finish, no config change

**Trigger:** the bell curve of the entry below is dominated by the sets that do not finish.

**Finding:** ten sets, 10E, 7ED, 8ED, 9ED, PLS, AER, AKH, BFZ, HOU and KLD, hold cards that a booster gives
once in thousands, the star printings and the Masterpiece series. No config can finish them in 400 days,
see [Why ten sets never finish their binder](specs-review.md#why-ten-sets-never-finish-their-binder).

**What it changes:** nothing in the config or the goals. It says the fix is a binder rule, not a tuning
value, and that no search under the bell goal can reach a bell while those sets count.

## 2026-10-23 – Pace judged as a bell curve, no search run

**Trigger:** the pace goal is the shape of the times across the sets, a bell around two weeks, see
[ADR-0051](adr/0051-the-pace-of-the-scope-is-a-bell-curve-around-two-weeks.md). The per-set floor and
ceiling of the entry below are retired.

**Goals:** a shape on `days_to_100pct` across the 107 sets, each alone.

| Visits a day | Center, days | Spread, days | Run length, days |
|-------------:|-------------:|-------------:|-----------------:|
| 3 | 14 | 4 | 40 |
| 6 | 7 | 2 | 20 |

The center at 3 visits is the two weeks asked for. The center at 6 visits is the old goal's one week. The
spreads are a first guess: the give or take of the old bands.

**Impact on the two configs of the entry below**, 3 visits a day for 40 days, seed 0, with no new search.
The distance counts each set as one per spread from its place in the bell.

| Reading | Baseline | Candidate 23 |
|---------|---------:|-------------:|
| Distance from the bell, all 107 sets | 160.2 | 159.1 |
| Distance of the sets that finish, 91 and 92 of them | 75.9 | 87.8 |
| Mean days of the sets that finish | 12.3 | 10.8 |

**What it shows**
- About 84 of the baseline's 160 comes from the 16 sets that do not finish, which sit far out in the
  tail. They are the largest piece of the distance on both configs.
- Among the sets that finish, the baseline is closer to the bell than candidate 23, and both are quicker
  than two weeks on average.
- The two distances are close only because the unfinished sets dominate them.

**What it does not show:** the six-visit readings, which were not taken, and any config tuned for this
goal. Losses under it do not compare with those under bands.

## 2026-10-23 – Counter stock goals read with trades, one new goal

**Trigger:** the Counter is not the only route to a card, so the stock goals now count trades, see
[ADR-0049](adr/0049-a-rare-is-scarce-at-the-counter-and-still-reachable-by-trade.md). A goal band is
added, none of the existing ones moves.

**Bands:**

| Metric | Band | Source |
|--------|------|--------|
| `rare_out_of_stock_pct` | 10% to 50% | unchanged, set 2026-10-16 |
| `rare_stock_per_printing` | 0.5 to 3 | unchanged, set 2026-10-16 |
| `rare_unreachable_pct` | at most 10% | new, a first guess with no run behind it |

**Status:** the metric `rare_unreachable_pct` is not built, so it is not in `tune/goals.yaml`. The two
Counter goals there are still judged without trades.

**What it does not show:** a reading of any of the three with trades on. The readings of the entry below
were taken with trades off, and they do not carry over.

## 2026-10-23 – Pace ceiling loosened, no search run

**Trigger:** slow sets are welcome, so the pace goal keeps its floor and gives up its flat ceiling, see
[ADR-0050](adr/0050-a-binder-is-never-fast-and-some-sets-are-slow-on-purpose.md). A goal band changed,
not a default of the config.

**Bands:** per set alone at 3 visits a day, 10 to 18 days became 10 to 40, the length of the run. At 6
visits, 5 to 9 became 5 to 20.

**Impact on the two configs of the entry below**, re-read at 3 visits a day for 40 days, seed 0, with no
new search:

| Sets of the 107 | Baseline | Candidate 23 |
|-----------------|---------:|-------------:|
| Under 10 days | 42 | 49 |
| In the new band, 10 to 40 days | 49 | 43 |
| Not finished in 40 days | 16 | 15 |

**What it shows**
- The fast sets are what is left to fix: 42 sets finish under the floor on the baseline.
- Candidate 23 is worse than the baseline under the new bands as well, with more sets under the floor.
- A loss under the new bands does not compare with one under the old: a wider band makes a set
  outside it weigh less. The next search starts a new series.

**What it does not show:** the six-visit readings, which were not taken, and any config tuned for the
new bands.

The run lengths of 40 and 20 days, and so the ceilings, are a first guess: about twice the old ceiling.

## 2026-10-22 – First search on the 107 sets, not applied

**Trigger:** the first run of the files in `tune/`, after the energy cost table had to rise, see
[ADR-0048](adr/0048-a-dearer-energy-tier-costs-more-energy.md). Price snapshot of 2026-10-15.

**Run:** 48 candidates over 13 keys, none refused. The best 12 scored on 2 seeds and the best 3
validated on 2 fresh ones. About 45 minutes niced on 3 workers.

**Status:** not applied. `config.defaults()` is unchanged.

The best candidate, number 23, would change these defaults:

| Key | Default | Candidate 23 |
|-----|--------:|-------------:|
| `counter_sell_pct` | 60 | 95 |
| `counter_buy_pct` | 120 | 159 |
| `counter_foil_pct` | 500 | 593 |
| `counter_shelf_days` | -1 | 23 |
| `counter_supply` | none | fixed, 60 a day |
| `npcs.population` | 0 | 9 |
| `initial_stock.rare` | 2 | 6 |
| `daily_reward` | 100 | 25 |
| `energy.recharge_per_tick` | 1 | 3 |
| `energy.reserve_cap` | 10 | 18 |
| `energy.cost_table[1].cost` | 5 | 7 |
| `energy.cost_table[2].cost` | 8 | 10 |

**Impact.** The baseline is the defaults with the Counter's fixed supply on, 13 boosters a day, as
`tune/base.yaml` sets it. Pace is the `collector` at 3 visits a day for 40 days, each set alone. The
other rows are the `master-collector` on the 107 sets for 90 days. Seed 0.

| Reading | Band | Baseline | Candidate 23 |
|---------|------|---------:|-------------:|
| Search loss | 0 | 158.3 | 134.5 (137.2 on fresh seeds) |
| Sets with a binder in 10 to 18 days | all 107 | 34 | 33 |
| Sets faster than 10 days | none | 42 | 49 |
| Sets slower than 18 days | none | 31 | 25 |
| `final_balance` | at most 3,000 | 110 | 27 |
| `rare_stock_per_printing` | 0.5 to 3 | 2.05 | 5.4 |
| `rare_out_of_stock_pct` | 10 to 50 | 4.5 | 4.5 |
| `daily_reward_commons` | 3 to 15 | 5 | 1 |
| `sub_coin_cards` | 0 | 0 | 0 |

**What it shows**
- The baseline already meets every whole-scope band but the out-of-stock share. Candidate 23 loses two
  of them, the copies per rare printing and the daily reward in commons, and gains nothing there.
- Its lower loss comes from the pace of the slow sets. The loss weighs a set that never finishes, which
  reads as 41 days, far above one that finishes too early, so the search leans toward speed.
- The number of sets inside the pace band does not change.
- Fourteen sets never finish in 40 days on either config: 10E, 7ED, 8ED, 9ED, AER, AKH, BFZ, FRF, HOU,
  KLD, OGW, ONS, PLS and SHM. The baseline also leaves MIR and USG unfinished, and candidate 23 leaves
  MIR. The causes were not studied.
- Of the 12 scored candidates, 9 leave 4.5% of rare printings out of stock and 3 leave 65% to 77%. None
  lands in the band, and twelve are too few to tell whether it can.

**What it does not show:** a search with more candidates, a wider range for the three keys at the edge
of theirs (`energy.recharge_per_tick`, `counter_supply.boosters_per_day`, `initial_stock.rare`), or a
loss that does not favour speed so strongly.

## 2026-10-16 – First search on 106 sets, not merged

**Trigger:** the first tuning system, on the sets in scope before All Will Be One loaded. 40 candidates
over 12 keys, on goals of its own, so its losses do not compare with the entry above.

**Status:** not merged. It sat on the local branch `parked-tuned-defaults`, because the energy cost table
it drew, 3, 9, 8 and 12, did not rise and broke the energy tests. That is what
[ADR-0048](adr/0048-a-dearer-energy-tier-costs-more-energy.md) forbids.

**Impact:** the loss went from 13.4 for the defaults of the time to 4.9, and 7.9 on fresh seeds. The
pace of the old sets, such as Mirage and Urza's Saga, still missed its bands.

Its candidate set the Counter to 62% and 142%, a daily reward of 23, a shelf of 22 days, a supply of 13
boosters a day, 27 NPCs, an energy reserve of 19 recharging 2 a tick, and 9 energy for the middle tier.
