package config_test

// Tests for defects A and B:
//   A – Milestone targets can pass on an unreached milestone
//   B – Energy costs the player can never afford

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

// ---------------------------------------------------------------------------
// A – Milestone targets: days_to_ max must be <= run_days
// ---------------------------------------------------------------------------

// TestLoad_daysToTarget_atMost_boundary verifies that a days_to_ metric with
// at_most and max == run_days is accepted (exact boundary is valid).
func TestLoad_daysToTarget_atMost_boundary(t *testing.T) {
	t.Parallel()
	const yaml = `
run_days: 90
targets:
  - metric: days_to_100pct_cu.rav
    comparison: at_most
    max: 90
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load: unexpected error at boundary: %v", err)
	}
}

// TestLoad_daysToTarget_atMost_aboveRunDays verifies that a days_to_ metric
// with at_most and max == run_days+1 is rejected. The error must name targets[i].max.
func TestLoad_daysToTarget_atMost_aboveRunDays(t *testing.T) {
	t.Parallel()
	const yaml = `
run_days: 90
targets:
  - metric: days_to_100pct_cu.rav
    comparison: at_most
    max: 91
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("Load: expected error for max above run_days, got nil")
	}
	if !strings.Contains(err.Error(), "targets[0].max") {
		t.Errorf("error %q does not contain %q", err.Error(), "targets[0].max")
	}
}

// TestLoad_daysToTarget_between_boundary verifies that a between target with
// max == run_days is accepted.
func TestLoad_daysToTarget_between_boundary(t *testing.T) {
	t.Parallel()
	const yaml = `
run_days: 60
targets:
  - metric: days_to_complete_commons
    comparison: between
    min: 10
    max: 60
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load: unexpected error at boundary: %v", err)
	}
}

// TestLoad_daysToTarget_between_aboveRunDays verifies that a between target
// with max > run_days is rejected and names targets[i].max.
func TestLoad_daysToTarget_between_aboveRunDays(t *testing.T) {
	t.Parallel()
	const yaml = `
run_days: 60
targets:
  - metric: days_to_complete_commons
    comparison: between
    min: 10
    max: 61
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("Load: expected error for max above run_days, got nil")
	}
	if !strings.Contains(err.Error(), "targets[0].max") {
		t.Errorf("error %q does not contain %q", err.Error(), "targets[0].max")
	}
}

// TestLoad_nonDaysToTarget_atMost_aboveRunDays verifies that a non-days_to_
// metric with at_most and a max well above run_days is accepted (no new check).
func TestLoad_nonDaysToTarget_atMost_aboveRunDays(t *testing.T) {
	t.Parallel()
	const yaml = `
run_days: 30
targets:
  - metric: daily_coins_earned
    comparison: at_most
    max: 9999
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load: unexpected error for non-days_to_ metric: %v", err)
	}
}

// TestLoad_daysToTarget_checksLoadedRunDays verifies that the check uses the
// run_days from the same file, not a hard-coded default.
func TestLoad_daysToTarget_checksLoadedRunDays(t *testing.T) {
	t.Parallel()

	// max == run_days in the same file → accepted
	const yamlOK = `
run_days: 30
targets:
  - metric: days_to_100pct_cu.rav
    comparison: at_most
    max: 30
`
	if _, err := config.Load(strings.NewReader(yamlOK)); err != nil {
		t.Fatalf("Load: unexpected error (max == run_days in file): %v", err)
	}

	// max == default run_days (90) but run_days in file is 30 → rejected
	const yamlBad = `
run_days: 30
targets:
  - metric: days_to_100pct_cu.rav
    comparison: at_most
    max: 90
`
	_, err := config.Load(strings.NewReader(yamlBad))
	if err == nil {
		t.Fatal("Load: expected error when max exceeds loaded run_days, got nil")
	}
	if !strings.Contains(err.Error(), "targets[0].max") {
		t.Errorf("error %q does not contain %q", err.Error(), "targets[0].max")
	}
}

// ---------------------------------------------------------------------------
// B – Energy costs the player can never afford
// ---------------------------------------------------------------------------

// TestLoad_energyCost_boundary verifies that cost == reserve_cap + savings_cap
// is accepted (the player can just barely afford it).
func TestLoad_energyCost_boundary(t *testing.T) {
	t.Parallel()
	const yaml = `
energy:
  reserve_cap: 10
  savings_cap: 0
  cost_table:
    - threshold: 100
      cost: 10
`
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load: unexpected error at boundary (cost == reserve_cap + savings_cap): %v", err)
	}
}

// TestLoad_energyCost_aboveCap verifies that cost == reserve_cap + savings_cap + 1
// is rejected, and the error names energy.cost_table[i].cost.
func TestLoad_energyCost_aboveCap(t *testing.T) {
	t.Parallel()
	const yaml = `
energy:
  reserve_cap: 10
  savings_cap: 0
  cost_table:
    - threshold: 100
      cost: 11
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("Load: expected error when cost exceeds reserve_cap + savings_cap, got nil")
	}
	if !strings.Contains(err.Error(), "energy.cost_table[0].cost") {
		t.Errorf("error %q does not contain %q", err.Error(), "energy.cost_table[0].cost")
	}
}

// TestLoad_energyCost_savingsCapZero_costAboveReserve is the concrete scenario
// from the defect report: savings_cap=0, cost=12, reserve_cap=10.
func TestLoad_energyCost_savingsCapZero_costAboveReserve(t *testing.T) {
	t.Parallel()
	const yaml = `
energy:
  reserve_cap: 10
  savings_cap: 0
  cost_table:
    - threshold: 400
      cost: 12
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("Load: expected error (savings_cap=0, cost=12 > reserve_cap=10), got nil")
	}
	if !strings.Contains(err.Error(), "energy.cost_table[0].cost") {
		t.Errorf("error %q does not contain %q", err.Error(), "energy.cost_table[0].cost")
	}
	if !strings.Contains(err.Error(), "reserve_cap") {
		t.Errorf("error %q does not contain %q", err.Error(), "reserve_cap")
	}
	if !strings.Contains(err.Error(), "savings_cap") {
		t.Errorf("error %q does not contain %q", err.Error(), "savings_cap")
	}
}

// TestLoad_energyCost_firstOffendingEntry verifies that when multiple cost-table
// entries are unaffordable, the error names the first offending index (stable order).
func TestLoad_energyCost_firstOffendingEntry(t *testing.T) {
	t.Parallel()
	const yaml = `
energy:
  reserve_cap: 5
  savings_cap: 0
  cost_table:
    - threshold: 100
      cost: 4
    - threshold: 200
      cost: 6
    - threshold: 400
      cost: 8
`
	_, err := config.Load(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "energy.cost_table[1].cost") {
		t.Errorf("error %q: want first offending index [1], got something else", err.Error())
	}
}
