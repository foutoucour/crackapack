// Package sim runs the headless, deterministic economy simulation.
//
// It composes internal/domain objects with internal/config values and records
// per-day metrics. The domain never imports sim; sim is the composition layer.
//
// Determinism guarantees:
//   - A single *rand.Rand seeded from config.Seed drives all randomness.
//   - All map iterations over domain types sort keys before acting.
//   - encoding/json sorts string-kinded map keys automatically.
//   - CSV columns use a fixed declared order.
//   - Same seed + config + price snapshot → byte-identical JSON and CSV output.
package sim

// TicksPerDay is the number of 10-minute ticks in one simulated day (24 h ÷ 10 min).
const TicksPerDay = 144
