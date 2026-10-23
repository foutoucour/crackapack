package play

import (
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

// SaveVersion is the version of the save file's layout. It changes only when a save written by an
// earlier version could no longer be read as it is. Adding a field does not change it: a field a
// save lacks loads as a new game's value, see [Game.Persist].
const SaveVersion = 1

// Snapshot is the state of a [Game] as a save file holds it, see
// docs/adr/0052-the-game-is-saved-in-one-versioned-json-file.md. Every state that the game gains is
// a field added here, a pointer or a slice or a map so that a save without it can be told apart
// from one that holds the empty value, and it is left out of a restore when it is absent.
type Snapshot struct {
	Version int `json:"version"`
	// Opener is where the booster random stream stood, so that a loaded game does not open the
	// boosters it opened before.
	Opener *OpenerSnapshot `json:"opener"`
	// Welcome is the welcome boosters left, by set code. A set missing from it has none.
	Welcome    map[string]int  `json:"welcome"`
	Collection []CardCount     `json:"collection"`
	Energy     *EnergySnapshot `json:"energy"`
	// LastOpening is the booster the pack page shows, null before the first opening.
	LastOpening *OpeningSnapshot `json:"last_opening"`
	// Unlocks is how far the player is through the stages, see [Game.Stage].
	Unlocks *UnlocksSnapshot `json:"unlocks"`
	// Counter is the coins and the Counter, see [Game.OpenCounter].
	Counter *CounterSnapshot `json:"counter"`
}

// OpenerSnapshot is a seed and the opaque state of the [Stream] it started.
type OpenerSnapshot struct {
	Seed  uint64 `json:"seed"`
	State []byte `json:"state"`
}

// CardCount is the copies of one printing in one finish that the collection holds.
type CardCount struct {
	ID       domain.CardID `json:"id"`
	Finish   domain.Finish `json:"finish"`
	Quantity int           `json:"quantity"`
}

// EnergySnapshot is the energy and the instant it was brought up to date at: the time the game was
// closed is recharged from SyncedAt when it is loaded.
type EnergySnapshot struct {
	Reserve     int       `json:"reserve"`
	Savings     int       `json:"savings"`
	SavingsTick int       `json:"savings_tick"`
	SyncedAt    time.Time `json:"synced_at"`
}

// OpeningSnapshot is an opening by the cards' keys, whose names and rarities come from the
// catalogues again when it is loaded.
type OpeningSnapshot struct {
	SetCode string       `json:"set"`
	Cards   []OpenedCard `json:"cards"`
	// Unlocked are the codes of the sets the opening unlocked.
	Unlocked []string `json:"unlocked,omitempty"`
}

// OpenedCard is one card of an [OpeningSnapshot].
type OpenedCard struct {
	ID     domain.CardID `json:"id"`
	Finish domain.Finish `json:"finish"`
	New    bool          `json:"new"`
}

// UnlocksSnapshot is the sets unlocked, the sets whose binder has unlocked sets already, and where
// the draw of the unlocks stands, so that a reload does not draw them again. The first stage is open
// whatever it holds.
type UnlocksSnapshot struct {
	Unlocked  []string        `json:"unlocked"`
	Triggered []string        `json:"triggered"`
	Rand      *OpenerSnapshot `json:"rand"`
}

// CounterSnapshot is the balances of the ledger, what the Counter holds and remembers, and the
// supply and the crowd that stock it, caught up to SyncedAt. The entries of the ledger are not
// kept: the balances are what the game reads.
type CounterSnapshot struct {
	// SyncedAt is the start of the first day of the supply and the crowd not caught up yet.
	SyncedAt time.Time                       `json:"synced_at"`
	Balances map[domain.Account]domain.Coins `json:"balances"`
	// Tick is the ticks the Counter has lived, which the shelf is stamped with.
	Tick int `json:"tick"`
	// Stock is the copies the Counter holds of the cards whose count differs from its initial
	// stock, which the catalogues and the rules give, so that a save stays small.
	Stock []CardCount `json:"stock"`
	// Shelf is the copies on a limited shelf, see domain.ShelfLife.
	Shelf []ShelfCount `json:"shelf,omitempty"`
	// LastInStock is, for the cards out of stock that the Counter had, when the last copy left.
	LastInStock []LastInStock   `json:"last_in_stock,omitempty"`
	Supply      *SupplySnapshot `json:"supply,omitempty"`
	Crowd       *CrowdSnapshot  `json:"crowd,omitempty"`
}

// ShelfCount is the copies of a card that arrived on the Counter's limited shelf at one tick.
type ShelfCount struct {
	ID       domain.CardID `json:"id"`
	Finish   domain.Finish `json:"finish"`
	Tick     int           `json:"tick"`
	Quantity int           `json:"quantity"`
}

// LastInStock is the instant the Counter's last copy of a card left.
type LastInStock struct {
	ID     domain.CardID `json:"id"`
	Finish domain.Finish `json:"finish"`
	At     time.Time     `json:"at"`
}

// SupplySnapshot is where the Counter's supply stands: its stream, the credit of a fixed supply,
// and the energy and the set picked of an energy supply.
type SupplySnapshot struct {
	Rand        OpenerSnapshot `json:"rand"`
	Credit      int            `json:"credit,omitempty"`
	Reserve     int            `json:"reserve,omitempty"`
	Savings     int            `json:"savings,omitempty"`
	SavingsTick int            `json:"savings_tick,omitempty"`
	Next        string         `json:"next,omitempty"`
}

// CrowdSnapshot is the NPCs' stream and what each NPC holds. Who the NPCs are is drawn again from
// the seed of Rand when the game loads.
type CrowdSnapshot struct {
	Rand OpenerSnapshot `json:"rand"`
	Tick int            `json:"tick"`
	NPCs []NPCSnapshot  `json:"npcs"`
}

// NPCSnapshot is what one NPC holds, in the order of the NPCs.
type NPCSnapshot struct {
	Credit int         `json:"credit"`
	Cards  []CardCount `json:"cards"`
}
