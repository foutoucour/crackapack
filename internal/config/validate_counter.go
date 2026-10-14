package config

import "fmt"

// validateCounterSupply checks the Counter's supply block. It returns an error naming the
// offending key.
func validateCounterSupply(s CounterSupplyConfig) error {
	switch s.Mode {
	case CounterSupplyNone:
		return nil
	case CounterSupplyFixed:
		if s.BoostersPerDay < 1 {
			return fmt.Errorf("config: counter_supply.boosters_per_day must be >= 1 in %s mode, got %d", CounterSupplyFixed, s.BoostersPerDay)
		}
		return nil
	default:
		return fmt.Errorf("config: counter_supply.mode must be %q or %q, got %q", CounterSupplyNone, CounterSupplyFixed, s.Mode)
	}
}
