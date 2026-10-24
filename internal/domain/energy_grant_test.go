package domain

import "testing"

func TestGrantAddsToTheSavingsAndLeavesTheReserve(t *testing.T) {
	e := mustNewEnergy(t, defaultEnergyParams())
	if err := e.Spend(4); err != nil {
		t.Fatal(err)
	}

	added, err := e.Grant(20)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	if added != 20 || e.Savings() != 20 || e.Reserve() != 6 {
		t.Errorf("Grant(20) = %d, savings %d, reserve %d; want 20, 20, 6", added, e.Savings(), e.Reserve())
	}
}

func TestGrantStopsAtTheSavingsCap(t *testing.T) {
	p := defaultEnergyParams()
	e := mustNewEnergy(t, p)
	if _, err := e.Grant(p.SavingsCap - 3); err != nil {
		t.Fatal(err)
	}

	added, err := e.Grant(10)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	if added != 3 || e.Savings() != p.SavingsCap {
		t.Errorf("Grant(10) near the cap = %d, savings %d; want 3, %d", added, e.Savings(), p.SavingsCap)
	}
}

func TestGrantRefusesAnAmountBelowOne(t *testing.T) {
	for _, amount := range []int{0, -5} {
		e := mustNewEnergy(t, defaultEnergyParams())
		if _, err := e.Grant(amount); err == nil {
			t.Errorf("Grant(%d) = nil error, want one", amount)
		}
		if e.Savings() != 0 {
			t.Errorf("Grant(%d) changed the savings to %d", amount, e.Savings())
		}
	}
}
