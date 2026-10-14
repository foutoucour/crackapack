package demo

// demSeed is the fixed random seed for the demo dataset. All random draws use
// this seed via a PCG generator so the dataset is byte-identical on every platform.
const demSeed uint64 = 42

// demoConfigYAML is the YAML configuration for the crackapack demo sweep.
//
// Calibration targets are pacing targets set by the maintainers. They were
// chosen after running the demo sweep on the synthetic dataset (seed 42, 90 days)
// and observing which metrics vary with the coin rate:
//
//	rate | sub_coin_cards | daily_reward_commons | unsold_duplicate_copies
//	-----+----------------+----------------------+------------------------
//	  50 |             44 |                   33 |                    4368
//	 100 |              2 |                   14 |                     888
//	 200 |              0 |                    7 |                       0
//	 400 |              0 |                    3 |                       0
//
// The three targets below split the grid: rates 50 and 100 fail at least one
// target; rates 200 and 400 pass all three.
//
// The Counter opens 40 boosters a day and pays from a treasury of 5 million coins plus 20 thousand
// a day, large enough that the sweep outcome above does not change: the demo exercises the
// Counter as a special player without moving its calibration. A smaller treasury runs the Counter
// out of coins at the higher rates, which removes the passing window.
//
// Energy costs 300 coins a point, so the cap's 12 energy is worth 3,600 coins. At the dearest rate
// of the sweep the demo's boosters are worth more than that and the player pays the difference as
// a fee; at the other rates no booster is. The sweep outcome above does not change. A price of 200
// would drain the player to a few hundred coins at rate 400, and a price of 100 would do so at
// rate 200 as well.
//
// Note: days_to_100pct_cu.dem = 4 at all rates (the collector's buy-all-cheapest
// strategy is rate-insensitive on this dataset). That metric is informative but
// not a calibration target here.
const demoConfigYAML = `
run_days: 90
seed: 42
sweep_rates: [50, 100, 200, 400]
npcs:
  population: 60
  surplus_hold_days: 2
trades:
  enabled: true
energy:
  coins_per_energy: 300
counter_supply:
  mode: fixed
  boosters_per_day: 40
counter_budget:
  enabled: true
  initial: 5000000
  daily_float: 20000

targets:
  - metric: sub_coin_cards
    comparison: at_most
    max: 5.0
  - metric: daily_reward_commons
    comparison: between
    min: 3.0
    max: 15.0
  - metric: unsold_duplicate_copies
    comparison: at_most
    max: 0.0
`
