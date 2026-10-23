package config

import "fmt"

// validateUnlock checks the lower bounds of the unlock block; validateBounds checks the upper ones.
func validateUnlock(u UnlockConfig) error {
	switch {
	case u.BinderThreshold <= 0 || u.BinderThreshold > 1:
		return fmt.Errorf("config: unlock.binder_threshold must be in (0, 1], got %v", u.BinderThreshold)
	case u.SetsPerUnlock < 1:
		return fmt.Errorf("config: unlock.sets_per_unlock must be >= 1, got %d", u.SetsPerUnlock)
	case u.Stages < 1:
		return fmt.Errorf("config: unlock.stages must be >= 1, got %d", u.Stages)
	case u.StartSets < 1:
		return fmt.Errorf("config: unlock.start_sets must be >= 1, got %d", u.StartSets)
	}
	return nil
}
