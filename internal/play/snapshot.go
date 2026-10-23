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
}

// OpenerSnapshot is the seed of the game and the opaque state of its [Stream].
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
}

// OpenedCard is one card of an [OpeningSnapshot].
type OpenedCard struct {
	ID     domain.CardID `json:"id"`
	Finish domain.Finish `json:"finish"`
	New    bool          `json:"new"`
}
