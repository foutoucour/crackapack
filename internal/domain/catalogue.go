package domain

import "fmt"

// Finish is a card printing finish as defined by MTGJSON.
type Finish string

const (
	// FinishNonFoil is the standard non-foil printing.
	FinishNonFoil Finish = "nonfoil"
	// FinishFoil is the foil printing.
	FinishFoil Finish = "foil"
)

// SheetCard is one entry in a Sheet: a card identifier and its relative draw weight.
type SheetCard struct {
	ID     CardID
	Weight int
}

// Card is one printing of a Magic card within a set catalogue.
type Card struct {
	ID          CardID
	Name        string
	Number      string
	Rarity      Rarity
	Finishes    []Finish
	IsBasicLand bool
	ScryfallID  string   // empty when absent from the source file
	Colors      []string // MTGJSON "colors", e.g. ["W","U"]; empty means colourless or data absent
}

// Sheet is one named card pool within a booster definition.
// Cards is sorted by ID ascending so that weighted draws over an injected
// seeded *rand.Rand are reproducible across runs regardless of map iteration order.
type Sheet struct {
	Name          string
	Cards         []SheetCard
	TotalWeight   int
	Foil          bool
	BalanceColors bool
}

// BoosterOption is one possible booster layout with its relative frequency weight.
type BoosterOption struct {
	Contents map[string]int // sheet name -> number of cards drawn from it
	Weight   int
}

// SetCatalogue is the complete card and booster data parsed from one MTGJSON set file.
type SetCatalogue struct {
	Code           string
	Name           string
	Cards          map[CardID]Card  // keyed by MTGJSON UUID for O(1) lookup
	Sheets         map[string]Sheet // keyed by sheet name
	BoosterOptions []BoosterOption
}

// CatalogueError is returned when a set file is missing or contains an invalid value.
// File is the fs.FS path as given to LoadSetCatalogue.
// Field is the dotted JSON path that failed, e.g. "data.cards[3].rarity".
// Err is the underlying cause; it may be nil when the field is merely absent.
type CatalogueError struct {
	File  string
	Field string
	Err   error
}

// Error implements the error interface.
func (e *CatalogueError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("catalogue %s: field %q: %v", e.File, e.Field, e.Err)
	}
	return fmt.Sprintf("catalogue %s: field %q: missing or empty", e.File, e.Field)
}

// Unwrap returns the underlying cause so callers can use errors.Is and errors.As.
func (e *CatalogueError) Unwrap() error { return e.Err }
