package demo

// demSeed is the fixed random seed for the demo dataset. All random draws use
// this seed via a PCG generator so the dataset is byte-identical on every platform.
const demSeed uint64 = 42

// playConfigYAML cuts the two demo sets into two stages of one, and lets a quarter of a binder
// unlock the other set, so that the welcome boosters of the first reach it: a binder at 90% takes a
// few dozen boosters.
const playConfigYAML = `
unlock:
  start_sets: 1
  stages: 1
  sets_per_unlock: 1
  binder_threshold: 0.25
`

// demoConfigYAML is the YAML configuration for the crackapack demo sweep.
//
// Calibration targets are pacing targets set by the maintainers. They were
// chosen after running the demo sweep on the synthetic dataset (seed 42, 90 days)
// and observing which metrics vary with the coin rate:
//
//	rate | sub_coin_cards | daily_reward_commons | unsold_duplicate_copies
//	-----+----------------+----------------------+------------------------
//	  50 |              0 |                   33 |                       0
//	 100 |              0 |                   14 |                       0
//	 200 |              0 |                    7 |                       0
//	 400 |              0 |                    3 |                       0
//
// The Counter pays one coin for a card priced at one cent, see ADR-0044, so no rate leaves a
// sub-coin card here, and the table above is what the sweep gave after that floor: before it, rates
// 50 and 100 had 44 and 2 such cards. The targets below now split the grid on the daily reward alone:
// rate 50 fails it, and rates 100 to 400 pass all three.
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
// DemoCoinTopupMinEnergyYAML is a sweep config that enables the min_energy coin top-up variant.
// Players pay coins for any energy shortfall when opening a booster; they always need at least
// min_energy_units real energy (opening with coins only is never allowed).
// Use with --player=converting-collector so the policy actually uses top-up each session.
const DemoCoinTopupMinEnergyYAML = `
run_days: 90
seed: 42
sweep_rates: [50, 100, 200, 400]
npcs:
  population: 70
  surplus_hold_days: 2
trades:
  enabled: true
energy:
  coins_per_energy: 300
  coin_topup:
    enabled: true
    variant: min_energy
    min_energy_units: 1
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

// DemoCoinTopupEscalatingYAML is a sweep config that enables the escalating coin top-up variant.
// The price for the k-th energy unit bought today starts at coins_per_energy and rises by
// escalation_step per unit; it resets to the base price at the start of each day.
// Use with --player=converting-collector so the policy actually uses top-up each session.
const DemoCoinTopupEscalatingYAML = `
run_days: 90
seed: 42
sweep_rates: [50, 100, 200, 400]
npcs:
  population: 70
  surplus_hold_days: 2
trades:
  enabled: true
energy:
  coins_per_energy: 300
  coin_topup:
    enabled: true
    variant: escalating
    min_energy_units: 1
    escalation_step: 30
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

// DemoCoinTopupBothYAML is a sweep config that enables both coin top-up limits simultaneously:
// min_energy_units guards the real-energy floor, and the price also escalates within each day.
// Use with --player=converting-collector so the policy actually uses top-up each session.
const DemoCoinTopupBothYAML = `
run_days: 90
seed: 42
sweep_rates: [50, 100, 200, 400]
npcs:
  population: 70
  surplus_hold_days: 2
trades:
  enabled: true
energy:
  coins_per_energy: 300
  coin_topup:
    enabled: true
    variant: both
    min_energy_units: 1
    escalation_step: 30
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

const demoConfigYAML = `
run_days: 90
seed: 42
sweep_rates: [50, 100, 200, 400]
npcs:
  population: 70
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

// TuneConfigYAML is the base config of the demo search: a small crowd, the Counter opening
// boosters from a treasury, and no coin top-up. The search varies it, see TuneSpaceYAML.
const TuneConfigYAML = `
run_days: 30
seed: 42
npcs:
  population: 20
  surplus_hold_days: 2
energy:
  coins_per_energy: 300
counter_supply:
  mode: fixed
  boosters_per_day: 40
counter_budget:
  enabled: true
  initial: 5000000
  daily_float: 20000
`

// TuneSpaceYAML is the demo search space: the Counter's supply and stock, its prices and the
// energy that sets the pace.
const TuneSpaceYAML = `
params:
  - {path: counter_supply.boosters_per_day, type: int, min: 1, max: 40, scale: log}
  - {path: counter_shelf_days, type: int, min: 0, max: 30}
  - {path: npcs.population, type: int, min: 0, max: 30}
  - {path: initial_stock.rare, type: int, min: 0, max: 6}
  - {path: initial_stock.mythic, type: int, min: 0, max: 6}
  - {path: daily_reward, type: int, min: 5, max: 200, scale: log}
  - {path: counter_buy_pct, type: int, min: 100, max: 200}
  - {path: counter_sell_pct, type: int, min: 40, max: 120}
  - {path: energy.recharge_per_tick, type: int, min: 1, max: 3}
  - {path: energy.reserve_cap, type: int, min: 4, max: 20}
  - {path: "energy.cost_table[1].cost", type: int, min: 4, max: 7}
`

// TuneGoalsYAML is the demo goals: the pace of each set played alone, at three and at six visits a
// day, and a Counter whose rares are scarce but not gone. The bands are the product's, see
// ADR-0051 and ADR-0049. The pace is a bell over the two demo sets, scaled to the demo's 30 days.
const TuneGoalsYAML = `
scenarios:
  - {name: pace-3-visits, player: collector, sessions_per_day: 3, run_days: 30, sets: [DEM, DSP], supplementary: [DST], per_set: true}
  - {name: pace-6-visits, player: collector, sessions_per_day: 6, run_days: 15, sets: [DEM, DSP], supplementary: [DST], per_set: true}
goals:
  - {scenario: pace-3-visits, metric: rare_out_of_stock_pct, comparison: between, min: 10, max: 50}
  - {scenario: pace-3-visits, metric: rare_stock_per_printing, comparison: between, min: 0.5, max: 3}
  - {scenario: pace-3-visits, metric: sub_coin_cards, comparison: at_most, max: 0}
  - {scenario: pace-3-visits, metric: daily_reward_commons, comparison: between, min: 3, max: 15}
shapes:
  - {scenario: pace-3-visits, metric: "days_to_100pct.{set}", center: 10, spread: 3}
  - {scenario: pace-6-visits, metric: "days_to_100pct.{set}", center: 5, spread: 2}
`
