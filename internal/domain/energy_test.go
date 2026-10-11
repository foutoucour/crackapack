package domain

import (
	"errors"
	"testing"
)

// defaultEnergyParams returns a baseline EnergyParams for tests.
// ReserveCap=10, RechargePerTick=1, SavingsIntervalTicks=3, SavingsCap=200.
func defaultEnergyParams() EnergyParams {
	return EnergyParams{
		RechargePerTick:      1,
		ReserveCap:           10,
		SavingsIntervalTicks: 3,
		SavingsCap:           200,
	}
}

func mustNewEnergy(t *testing.T, p EnergyParams) *Energy {
	t.Helper()
	e, err := NewEnergy(p)
	if err != nil {
		t.Fatalf("NewEnergy: unexpected error: %v", err)
	}
	return e
}

// TestNewEnergyStartingState verifies the documented starting state.
func TestNewEnergyStartingState(t *testing.T) {
	p := defaultEnergyParams()
	e := mustNewEnergy(t, p)
	if e.Reserve() != p.ReserveCap {
		t.Errorf("Reserve() = %d, want %d (full reserve)", e.Reserve(), p.ReserveCap)
	}
	if e.Savings() != 0 {
		t.Errorf("Savings() = %d, want 0 (empty savings)", e.Savings())
	}
}

// TestNewEnergyValidation checks that each invalid param produces a named error.
func TestNewEnergyValidation(t *testing.T) {
	base := defaultEnergyParams()
	cases := []struct {
		name   string
		params EnergyParams
	}{
		{"RechargePerTick zero", EnergyParams{RechargePerTick: 0, ReserveCap: base.ReserveCap, SavingsIntervalTicks: base.SavingsIntervalTicks, SavingsCap: base.SavingsCap}},
		{"RechargePerTick negative", EnergyParams{RechargePerTick: -1, ReserveCap: base.ReserveCap, SavingsIntervalTicks: base.SavingsIntervalTicks, SavingsCap: base.SavingsCap}},
		{"ReserveCap zero", EnergyParams{RechargePerTick: base.RechargePerTick, ReserveCap: 0, SavingsIntervalTicks: base.SavingsIntervalTicks, SavingsCap: base.SavingsCap}},
		{"ReserveCap negative", EnergyParams{RechargePerTick: base.RechargePerTick, ReserveCap: -5, SavingsIntervalTicks: base.SavingsIntervalTicks, SavingsCap: base.SavingsCap}},
		{"SavingsIntervalTicks zero", EnergyParams{RechargePerTick: base.RechargePerTick, ReserveCap: base.ReserveCap, SavingsIntervalTicks: 0, SavingsCap: base.SavingsCap}},
		{"SavingsIntervalTicks negative", EnergyParams{RechargePerTick: base.RechargePerTick, ReserveCap: base.ReserveCap, SavingsIntervalTicks: -1, SavingsCap: base.SavingsCap}},
		{"SavingsCap negative", EnergyParams{RechargePerTick: base.RechargePerTick, ReserveCap: base.ReserveCap, SavingsIntervalTicks: base.SavingsIntervalTicks, SavingsCap: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEnergy(tc.params)
			if err == nil {
				t.Error("NewEnergy: expected error, got nil")
			}
		})
	}
}

// TestTickZeroIsNoOp confirms Tick(0) leaves state unchanged.
func TestTickZeroIsNoOp(t *testing.T) {
	p := defaultEnergyParams()
	e := mustNewEnergy(t, p)
	// Spend to create a non-trivial state.
	if err := e.Spend(3); err != nil {
		t.Fatal(err)
	}
	wantReserve := e.Reserve()
	wantSavings := e.Savings()

	if err := e.Tick(0); err != nil {
		t.Fatalf("Tick(0): unexpected error: %v", err)
	}
	if e.Reserve() != wantReserve {
		t.Errorf("Reserve() = %d, want %d after Tick(0)", e.Reserve(), wantReserve)
	}
	if e.Savings() != wantSavings {
		t.Errorf("Savings() = %d, want %d after Tick(0)", e.Savings(), wantSavings)
	}
}

// TestTickNegativeReturnsError ensures Tick with n<0 returns an error and leaves state unchanged.
func TestTickNegativeReturnsError(t *testing.T) {
	e := mustNewEnergy(t, defaultEnergyParams())
	if err := e.Spend(1); err != nil {
		t.Fatal(err)
	}
	wantReserve := e.Reserve()

	if err := e.Tick(-1); err == nil {
		t.Error("Tick(-1): expected error, got nil")
	}
	if e.Reserve() != wantReserve {
		t.Errorf("Reserve() = %d after Tick(-1), want %d (state must be unchanged)", e.Reserve(), wantReserve)
	}
}

// TestTickRechargesReserve verifies reserve recharges by RechargePerTick per tick,
// is capped at ReserveCap and does not overflow.
func TestTickRechargesReserve(t *testing.T) {
	cases := []struct {
		name            string
		spend           int
		ticks           int
		wantReserve     int
		wantSavingsTick bool // whether savings should have started (none expected here)
	}{
		{"one tick adds one", 1, 1, 10, false},
		{"partial refill", 5, 3, 8, false},
		{"caps at ReserveCap", 10, 15, 10, false},
		{"large RechargePerTick caps correctly", 0, 0, 10, false}, // no spend, stays full
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := defaultEnergyParams()
			e := mustNewEnergy(t, p)
			if tc.spend > 0 {
				if err := e.Spend(tc.spend); err != nil {
					t.Fatalf("Spend: %v", err)
				}
			}
			if err := e.Tick(tc.ticks); err != nil {
				t.Fatalf("Tick: %v", err)
			}
			if e.Reserve() != tc.wantReserve {
				t.Errorf("Reserve() = %d, want %d", e.Reserve(), tc.wantReserve)
			}
		})
	}
}

// TestTickLargeRechargePerTickCaps ensures a RechargePerTick larger than the gap
// does not push the reserve past ReserveCap.
func TestTickLargeRechargePerTickCaps(t *testing.T) {
	p := EnergyParams{RechargePerTick: 5, ReserveCap: 10, SavingsIntervalTicks: 3, SavingsCap: 200}
	e := mustNewEnergy(t, p)
	if err := e.Spend(3); err != nil {
		t.Fatal(err)
	}
	// One tick should add 5 but cap at 10 (gap is only 3).
	if err := e.Tick(1); err != nil {
		t.Fatal(err)
	}
	if e.Reserve() != 10 {
		t.Errorf("Reserve() = %d, want 10 after large RechargePerTick refills", e.Reserve())
	}
}

// TestTickSavingsRechargesWhenReserveFull checks savings recharges every SavingsIntervalTicks
// ticks only while reserve is full.
func TestTickSavingsRechargesWhenReserveFull(t *testing.T) {
	p := defaultEnergyParams() // SavingsIntervalTicks=3
	e := mustNewEnergy(t, p)

	// After 2 ticks, savingsTick=2 but savings hasn't triggered yet.
	if err := e.Tick(2); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 0 {
		t.Errorf("Savings() = %d after 2 ticks, want 0 (counter not yet reached)", e.Savings())
	}

	// 3rd tick: savingsTick reaches 3 → savings becomes 1, counter resets.
	if err := e.Tick(1); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 1 {
		t.Errorf("Savings() = %d after 3 ticks, want 1", e.Savings())
	}

	// 6 total ticks → savings = 2.
	if err := e.Tick(3); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 2 {
		t.Errorf("Savings() = %d after 6 ticks, want 2", e.Savings())
	}
}

// TestTickSavingsDoesNotAdvanceDuringReserveRecharge verifies savingsTick stays
// frozen while the reserve is below cap.
func TestTickSavingsDoesNotAdvanceDuringReserveRecharge(t *testing.T) {
	p := defaultEnergyParams() // SavingsIntervalTicks=3
	e := mustNewEnergy(t, p)

	// Drain the entire reserve.
	if err := e.Spend(10); err != nil {
		t.Fatal(err)
	}
	// Tick 9 times while reserve recharges (1 per tick). 9 ticks → reserve=9, still below cap.
	if err := e.Tick(9); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 0 {
		t.Errorf("Savings() = %d after recharging reserve, want 0 (savingsTick must not advance)", e.Savings())
	}

	// One more tick: reserve reaches 10 (full), but that tick was spent recharging → no savings yet.
	if err := e.Tick(1); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 0 {
		t.Errorf("Savings() = %d on tick reserve reaches full, want 0", e.Savings())
	}

	// 3 more ticks: reserve full all three, savingsTick goes 1→2→3 → savings=1.
	if err := e.Tick(3); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 1 {
		t.Errorf("Savings() = %d, want 1 after first full-reserve interval", e.Savings())
	}
}

// TestTickSavingsCapEnforced checks savings cannot exceed SavingsCap.
func TestTickSavingsCapEnforced(t *testing.T) {
	p := EnergyParams{RechargePerTick: 1, ReserveCap: 1, SavingsIntervalTicks: 1, SavingsCap: 3}
	e := mustNewEnergy(t, p)

	// With SavingsIntervalTicks=1, every tick while full adds 1 savings. After 10 ticks savings
	// must be capped at 3.
	if err := e.Tick(10); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 3 {
		t.Errorf("Savings() = %d, want 3 (SavingsCap)", e.Savings())
	}
}

// TestTickSavingsCounterRetainsProgressAcrossSpendRefill documents and verifies that
// the savings-interval counter keeps its value when the reserve is spent and then refills.
// This prevents losing partial progress from brief spending dips.
func TestTickSavingsCounterRetainsProgressAcrossSpendRefill(t *testing.T) {
	p := defaultEnergyParams() // SavingsIntervalTicks=3
	e := mustNewEnergy(t, p)

	// Advance savingsTick to 2 (one short of triggering savings).
	if err := e.Tick(2); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 0 {
		t.Fatalf("Savings() = %d after 2 ticks, want 0", e.Savings())
	}

	// Spend 1 energy: reserve drops to 9, savings-interval counter stays at 2.
	if err := e.Spend(1); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	// One tick refills the reserve (9→10). This tick is spent recharging, not advancing savingsTick.
	if err := e.Tick(1); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 0 {
		t.Errorf("Savings() = %d after refill tick, want 0 (counter at 2, not yet 3)", e.Savings())
	}

	// One more tick with reserve full: savingsTick goes from 2 to 3 → savings becomes 1.
	if err := e.Tick(1); err != nil {
		t.Fatal(err)
	}
	if e.Savings() != 1 {
		t.Errorf("Savings() = %d, want 1 (counter resumed from retained value 2)", e.Savings())
	}
}

// TestSpendWithinReserve verifies spending leaves savings untouched.
func TestSpendWithinReserve(t *testing.T) {
	e := mustNewEnergy(t, defaultEnergyParams()) // reserve=10

	if err := e.Spend(4); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if e.Reserve() != 6 {
		t.Errorf("Reserve() = %d, want 6", e.Reserve())
	}
	if e.Savings() != 0 {
		t.Errorf("Savings() = %d, want 0 (savings untouched)", e.Savings())
	}
}

// TestSpendCrossesIntoSavings verifies reserve empties first then savings covers the rest.
func TestSpendCrossesIntoSavings(t *testing.T) {
	p := defaultEnergyParams()
	e := mustNewEnergy(t, p)
	// Add 5 savings.
	if err := e.Tick(15); err != nil { // 15 ticks at SavingsIntervalTicks=3 → 5 savings
		t.Fatal(err)
	}
	if e.Savings() != 5 {
		t.Fatalf("Savings() = %d after 15 ticks, want 5", e.Savings())
	}

	// Spend 13: reserve(10) is consumed first, then 3 from savings.
	if err := e.Spend(13); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if e.Reserve() != 0 {
		t.Errorf("Reserve() = %d, want 0", e.Reserve())
	}
	if e.Savings() != 2 {
		t.Errorf("Savings() = %d, want 2 (5 - 3 = 2)", e.Savings())
	}
}

// TestSpendExactTotal confirms spending exactly reserve+savings leaves both at zero.
func TestSpendExactTotal(t *testing.T) {
	p := defaultEnergyParams()
	e := mustNewEnergy(t, p)
	// Add 2 savings (6 ticks at interval=3).
	if err := e.Tick(6); err != nil {
		t.Fatal(err)
	}
	total := e.Reserve() + e.Savings() // 10 + 2 = 12

	if err := e.Spend(total); err != nil {
		t.Fatalf("Spend(%d): %v", total, err)
	}
	if e.Reserve() != 0 {
		t.Errorf("Reserve() = %d, want 0", e.Reserve())
	}
	if e.Savings() != 0 {
		t.Errorf("Savings() = %d, want 0", e.Savings())
	}
}

// TestSpendInsufficientReturnsTypedErrorAndLeavesStateUnchanged checks that
// *InsufficientEnergyError is returned and state is not modified.
func TestSpendInsufficientReturnsTypedErrorAndLeavesStateUnchanged(t *testing.T) {
	e := mustNewEnergy(t, defaultEnergyParams()) // reserve=10, savings=0, total=10

	wantReserve := e.Reserve()
	wantSavings := e.Savings()

	err := e.Spend(11)
	if err == nil {
		t.Fatal("Spend(11): expected error, got nil")
	}
	var insErr *InsufficientEnergyError
	if !errors.As(err, &insErr) {
		t.Fatalf("Spend(11): error type = %T, want *InsufficientEnergyError", err)
	}
	if insErr.Have != 10 {
		t.Errorf("InsufficientEnergyError.Have = %d, want 10", insErr.Have)
	}
	if insErr.Need != 11 {
		t.Errorf("InsufficientEnergyError.Need = %d, want 11", insErr.Need)
	}
	if e.Reserve() != wantReserve {
		t.Errorf("Reserve() = %d after failed Spend, want %d (state unchanged)", e.Reserve(), wantReserve)
	}
	if e.Savings() != wantSavings {
		t.Errorf("Savings() = %d after failed Spend, want %d (state unchanged)", e.Savings(), wantSavings)
	}
}

// TestSpendInvalidAmountReturnsTypedError checks that amount <= 0 returns
// *InvalidSpendAmountError and leaves state unchanged.
func TestSpendInvalidAmountReturnsTypedError(t *testing.T) {
	cases := []struct{ amount int }{
		{0},
		{-1},
		{-100},
	}
	for _, tc := range cases {
		e := mustNewEnergy(t, defaultEnergyParams())
		wantReserve := e.Reserve()

		err := e.Spend(tc.amount)
		if err == nil {
			t.Errorf("Spend(%d): expected error, got nil", tc.amount)
			continue
		}
		var invErr *InvalidSpendAmountError
		if !errors.As(err, &invErr) {
			t.Errorf("Spend(%d): error type = %T, want *InvalidSpendAmountError", tc.amount, err)
		}
		if e.Reserve() != wantReserve {
			t.Errorf("Spend(%d): Reserve() = %d, want %d (state unchanged)", tc.amount, e.Reserve(), wantReserve)
		}
	}
}
