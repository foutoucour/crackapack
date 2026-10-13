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
// Note: days_to_100pct_cu.dem = 4 at all rates (the collector's buy-all-cheapest
// strategy is rate-insensitive on this dataset). That metric is informative but
// not a calibration target here.
const demoConfigYAML = `
run_days: 90
seed: 42
sweep_rates: [50, 100, 200, 400]
npcs:
  population: 60

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
