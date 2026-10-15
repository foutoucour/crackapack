package sim

import "github.com/foutoucour/crackapack/internal/domain"

// Session is the action API available to a PlayerPolicy during one session.
// The engine's concrete port type implements this interface.
//
// View() returns a fresh WorldView computed at the moment of the call, so the
// policy sees updated state after each action. The other methods act on the
// simulation state and return results or errors.
//
// A run without trades still offers the Trading methods, which refuse or return nothing, so a
// policy needs no check.
//
// Only ErrSubCoin (from BuyFromPlayer or SellToPlayer) and ErrCounterBroke (from BuyFromPlayer
// when the Counter has a treasury) are treated as normal refusals; all other domain errors abort
// the simulation run.
type Session interface {
	Trading

	// View returns the current simulation state. Cheap to call; recomputed each time.
	View() WorldView

	// OpenBooster spends energy and adds one booster from setCode to the collection.
	// Returns an error if the set is unknown or energy is insufficient.
	OpenBooster(setCode string) error

	// SellDuplicates sells all copies above keep to the Counter. keep must be in [1,4].
	// Sub-coin trades are counted as refused and the card is restored to the collection. A
	// Counter that cannot pay refuses once per card, not once per copy.
	// Returns the number of individual cards sold and the number refused.
	SellDuplicates(keep int) (sold, refused int)

	// BuyCard purchases one copy of key from the Counter and adds it to the collection.
	// Returns an error if the card is out of stock, unknown, or the player cannot afford it.
	BuyCard(key domain.CardKey) error

	// OpenBoosterTopUp opens one booster from setCode, spending whatever real energy the player
	// has (up to the booster's full cost) and paying coins for any shortfall. The player must
	// have at least the configured minimum real energy (min_energy_units); opening with coins
	// only is never allowed. Returns ErrInsufficientEnergy when real energy is below the
	// minimum, and ErrInsufficientBalance when the coin balance cannot cover both the top-up
	// and the mandatory fee. When top-up is disabled, it behaves identically to OpenBooster.
	OpenBoosterTopUp(setCode string) error
}

// PlayerPolicy decides what happens during one session. The engine calls RunSession
// once per session tick, passing the live Session handle.
//
// Implementations must not retain the Session reference beyond the call.
type PlayerPolicy interface {
	RunSession(s Session)
}
