package domain

import "testing"

func TestEnergyRestore_resumesWhereTheStateWasTaken(t *testing.T) {
	p := defaultEnergyParams()
	src := mustNewEnergy(t, p)
	if err := src.Spend(10); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if err := src.Tick(10 + 3*5 + 1); err != nil { // refill, five savings, one tick into the next
		t.Fatalf("Tick: %v", err)
	}

	dst := mustNewEnergy(t, p)
	if err := dst.Restore(src.State()); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Two more savings points need the same ticks from both only if the interval counter came along.
	for _, e := range []*Energy{src, dst} {
		if err := e.Tick(5); err != nil {
			t.Fatalf("Tick: %v", err)
		}
	}
	if dst.State() != src.State() {
		t.Errorf("restored energy state = %+v after 5 ticks, want %+v", dst.State(), src.State())
	}
}

func TestEnergyRestore_rejectsAStateOutsideTheCaps(t *testing.T) {
	tests := []struct {
		name  string
		state EnergyState
	}{
		{"negative reserve", EnergyState{Reserve: -1}},
		{"reserve above the cap", EnergyState{Reserve: 11}},
		{"negative savings", EnergyState{Reserve: 10, Savings: -1}},
		{"savings above the cap", EnergyState{Reserve: 10, Savings: 201}},
		{"negative savings tick", EnergyState{Reserve: 10, SavingsTick: -1}},
		{"savings tick past the interval", EnergyState{Reserve: 10, SavingsTick: 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := mustNewEnergy(t, defaultEnergyParams())
			before := e.State()

			if err := e.Restore(tc.state); err == nil {
				t.Fatalf("Restore(%+v) returned nil error", tc.state)
			}
			if e.State() != before {
				t.Errorf("state after a refused Restore = %+v, want %+v", e.State(), before)
			}
		})
	}
}
