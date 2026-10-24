package domain

import "fmt"

// Grant adds amount of bonus energy, as a reward pays it, and returns what it added. It goes to the
// savings, not the reserve: the reserve keeps recharging by itself, so a bonus there would only
// cost the player the recharge it replaces. Energy above SavingsCap is lost. Grant returns an
// error, and changes nothing, when amount is not positive. A grant is never paid for with coins,
// see [Energy].
func (e *Energy) Grant(amount int) (int, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("energy: grant amount must be > 0, got %d", amount)
	}
	added := min(amount, e.params.SavingsCap-e.savings)
	e.savings += added
	return added, nil
}
