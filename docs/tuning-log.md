# Tuning log

The economy's config changes often. This log records each change to its values, or each search that
proposed one, with what the change did to the goals. It is a history: an ADR records a decision that
holds until it is superseded, and this log records values that the next search will replace.

An ADR names a goal and its metric and leaves out the numbers of its band. The ADRs accepted before
this rule, such as ADR-0041 and ADR-0043, still carry the numbers they were accepted with. The current bands are in
[`tune/goals.yaml`](../tune/goals.yaml), and every change to one is an entry here.

How the search works is in [The tuning system](simulation.md#the-tuning-system). The goals are the bands
of ADR-0051, ADR-0041, ADR-0049 and ADR-0043, and the search files are in [`tune/`](../tune).

**Add an entry** whenever a default of the config changes, a search runs on the product's files, or a
goal band changes. Newest first. Each entry says:
- what triggered it and whether the values were applied to `config.defaults()`;
- the keys that changed, before and after;
- how the goals read before and after, on the same scenarios, so the entries compare;
- what the run does not tell.

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
