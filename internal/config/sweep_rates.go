package config

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// validateSweepRates checks that rates is non-empty, every entry is > 0,
// and entries are strictly ascending (duplicates are rejected).
func validateSweepRates(rates []domain.CoinRate) error {
	if len(rates) == 0 {
		return fmt.Errorf("config: sweep_rates must not be empty")
	}
	for i, r := range rates {
		if r <= 0 {
			return fmt.Errorf("config: sweep_rates[%d] must be > 0, got %d", i, r)
		}
	}
	for i := 1; i < len(rates); i++ {
		if rates[i] <= rates[i-1] {
			return fmt.Errorf(
				"config: sweep_rates[%d] (%d) must be > sweep_rates[%d] (%d); rates must be strictly ascending",
				i, rates[i], i-1, rates[i-1],
			)
		}
	}
	return nil
}
