package config

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ValidateSweepRates checks that rates is non-empty, every entry is > 0,
// and entries are strictly ascending (duplicates are rejected).
// Errors name the offending index using the same key-naming style as the
// other config validators ("config: sweep_rates[N]: …").
//
// Callers outside the package (e.g. the sweep command's --rates flag) may
// call this to apply the same rule to a flag-supplied grid.
func ValidateSweepRates(rates []domain.CoinRate) error {
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
