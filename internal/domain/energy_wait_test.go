package domain

import (
	"errors"
	"testing"
)

// spentDown returns an Energy whose reserve was emptied by one spend of the full cap.
func spentDown(t *testing.T) *Energy {
	t.Helper()
	e, err := NewEnergy(defaultEnergyParams())
	if err != nil {
		t.Fatalf("NewEnergy: %v", err)
	}
	if err := e.Spend(e.Reserve()); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	return e
}

func TestTicksUntil_isZeroWhenTheCostCanBePaidNow(t *testing.T) {
	t.Parallel()
	e, err := NewEnergy(defaultEnergyParams())
	if err != nil {
		t.Fatalf("NewEnergy: %v", err)
	}

	got, err := e.TicksUntil(5)

	if err != nil || got != 0 {
		t.Errorf("TicksUntil(5) on a full reserve = %d, %v, want 0, nil", got, err)
	}
}

func TestTicksUntil_countsTheTicksOfTheReserveRecharge(t *testing.T) {
	t.Parallel()
	e := spentDown(t)

	for cost, want := range map[int]int{1: 1, 5: 5, 10: 10} {
		got, err := e.TicksUntil(cost)
		if err != nil || got != want {
			t.Errorf("TicksUntil(%d) from an empty reserve = %d, %v, want %d", cost, got, err, want)
		}
	}
}

func TestTicksUntil_aCostAboveTheReserveCapWaitsForSavings(t *testing.T) {
	t.Parallel()
	e, err := NewEnergy(defaultEnergyParams())
	if err != nil {
		t.Fatalf("NewEnergy: %v", err)
	}

	// The reserve is full at 10 and each savings point takes 3 ticks, so a cost of 12 needs 2 points.
	got, err := e.TicksUntil(12)

	if err != nil || got != 6 {
		t.Errorf("TicksUntil(12) = %d, %v, want 6", got, err)
	}
}

func TestTicksUntil_isExactForEveryStateAndCost(t *testing.T) {
	t.Parallel()
	for spend := 0; spend <= 10; spend++ {
		for cost := 1; cost <= 12; cost++ {
			e, err := NewEnergy(defaultEnergyParams())
			if err != nil {
				t.Fatalf("NewEnergy: %v", err)
			}
			if spend > 0 {
				if err := e.Spend(spend); err != nil {
					t.Fatalf("Spend(%d): %v", spend, err)
				}
			}
			n, err := e.TicksUntil(cost)
			if err != nil {
				t.Fatalf("spend %d cost %d: %v", spend, cost, err)
			}

			if n > 0 {
				early := *e
				if err := early.Tick(n - 1); err != nil {
					t.Fatal(err)
				}
				if err := early.Spend(cost); err == nil {
					t.Errorf("spend %d cost %d: affordable after %d ticks, TicksUntil said %d", spend, cost, n-1, n)
				}
			}
			ready := *e
			if err := ready.Tick(n); err != nil {
				t.Fatal(err)
			}
			if err := ready.Spend(cost); err != nil {
				t.Errorf("spend %d cost %d: still not affordable after the %d ticks TicksUntil promised: %v", spend, cost, n, err)
			}
		}
	}
}

func TestTicksUntil_leavesTheEnergyAsItWas(t *testing.T) {
	t.Parallel()
	e := spentDown(t)
	reference := *e

	if _, err := e.TicksUntil(12); err != nil {
		t.Fatalf("TicksUntil: %v", err)
	}

	if *e != reference {
		t.Errorf("energy changed by asking: %+v, was %+v", *e, reference)
	}
}

func TestTicksUntil_refusesACostThatCanNeverBePaidOrIsNotPositive(t *testing.T) {
	t.Parallel()
	e, err := NewEnergy(defaultEnergyParams())
	if err != nil {
		t.Fatalf("NewEnergy: %v", err)
	}

	if _, err := e.TicksUntil(10 + 200 + 1); err == nil {
		t.Error("a cost above the reserve cap plus the savings cap returned no error")
	}
	var invalid *InvalidSpendAmountError
	if _, err := e.TicksUntil(0); !errors.As(err, &invalid) {
		t.Errorf("TicksUntil(0) error = %v, want an InvalidSpendAmountError", err)
	}
}
