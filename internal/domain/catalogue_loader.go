package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"time"
)

// mtgSetFile is the top-level MTGJSON v5 set file envelope.
type mtgSetFile struct {
	Data mtgSetData `json:"data"`
}

type mtgSetData struct {
	Code        string                   `json:"code"`
	Name        string                   `json:"name"`
	ReleaseDate string                   `json:"releaseDate"`
	Cards       []mtgCard                `json:"cards"`
	Booster     map[string]mtgBoosterDef `json:"booster"`
}

// parseReleaseDate parses an ISO date string in YYYY-MM-DD format.
// It returns an error whose message is "release date %q is not an ISO date such as 2005-10-07".
func parseReleaseDate(s string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("release date %q is not an ISO date such as 2005-10-07", s)
	}
	return t, nil
}

type mtgCard struct {
	UUID       string   `json:"uuid"`
	Name       string   `json:"name"`
	Number     string   `json:"number"`
	Rarity     string   `json:"rarity"`
	Finishes   []string `json:"finishes"`
	Types      []string `json:"types"`
	Supertypes []string `json:"supertypes"`
	Colors     []string `json:"colors"`
	// Identifiers is an inline struct matching the MTGJSON identifiers object.
	Identifiers struct {
		ScryfallID string `json:"scryfallId"`
	} `json:"identifiers"`
}

type mtgBoosterDef struct {
	Boosters []mtgBoosterOption  `json:"boosters"`
	Sheets   map[string]mtgSheet `json:"sheets"`
}

type mtgBoosterOption struct {
	Contents map[string]int `json:"contents"`
	Weight   int            `json:"weight"`
}

type mtgSheet struct {
	Cards         map[string]int `json:"cards"`
	TotalWeight   int            `json:"totalWeight"`
	Foil          bool           `json:"foil"`
	BalanceColors bool           `json:"balanceColors"`
}

// isValidRarity reports whether r is a recognised rarity.
// Unknown strings (bonus, empty) are rejected by the caller.
func isValidRarity(r Rarity) bool {
	switch r {
	case RarityCommon, RarityUncommon, RarityRare, RaritySpecial, RarityMythic:
		return true
	default:
		return false
	}
}

// isValidColor reports whether c is a recognised Magic colour letter (WUBRG).
func isValidColor(c string) bool {
	switch c {
	case "W", "U", "B", "R", "G":
		return true
	default:
		return false
	}
}

// normalizeFinish maps MTGJSON finish strings to the two canonical Finish values used by the
// simulation. "etched" (introduced ~2021 for etched-foil treatments) is treated as FinishFoil:
// the two foil variants are economically equivalent in the simulation, and treating etched as foil
// allows sets like NEO, SNC, and VOW to load without changes to collection counting or pricing.
func normalizeFinish(raw string) (Finish, bool) {
	switch raw {
	case string(FinishNonFoil):
		return FinishNonFoil, true
	case string(FinishFoil):
		return FinishFoil, true
	case "etched":
		return FinishFoil, true // etched foil ≈ regular foil for economy purposes
	default:
		return "", false
	}
}

// readAndCheckMetadata reads name from fsys, unmarshals the MTGJSON envelope, and
// validates that data.code and data.name are non-empty. It returns the raw mtgSetData.
// All errors are wrapped in *CatalogueError with the given name.
func readAndCheckMetadata(fsys fs.FS, name string) (mtgSetData, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return mtgSetData{}, &CatalogueError{File: name, Field: "(file)", Err: err}
	}

	var raw mtgSetFile
	if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
		var te *json.UnmarshalTypeError
		if errors.As(jsonErr, &te) {
			// te.Field is not the same on every Go release, so locate the value from the offset.
			field, ok := jsonPathAt(data, te.Offset)
			if !ok {
				field = te.Field
			}
			return mtgSetData{}, &CatalogueError{File: name, Field: field, Err: jsonErr}
		}
		// Syntax errors carry no field information.
		return mtgSetData{}, &CatalogueError{File: name, Field: "(json)", Err: jsonErr}
	}

	d := raw.Data
	if d.Code == "" {
		return mtgSetData{}, &CatalogueError{File: name, Field: "data.code"}
	}
	if d.Name == "" {
		return mtgSetData{}, &CatalogueError{File: name, Field: "data.name"}
	}
	return d, nil
}

// parseSetCards validates and converts the raw card array in d into a typed Cards map.
// All card-level validation (uuid, rarity, finish, color, duplicates) is performed here.
// Errors are wrapped in *CatalogueError with the given file name.
func parseSetCards(d mtgSetData, name string) (map[CardID]Card, error) {
	cards := make(map[CardID]Card, len(d.Cards))
	seen := make(map[CardID]int, len(d.Cards)) // uuid -> first index, for duplicate detection
	for i, mc := range d.Cards {
		if mc.UUID == "" {
			return nil, &CatalogueError{
				File:  name,
				Field: fmt.Sprintf("data.cards[%d].uuid", i),
				Err:   fmt.Errorf("uuid must not be empty"),
			}
		}
		id := CardID(mc.UUID)
		if first, dup := seen[id]; dup {
			return nil, &CatalogueError{
				File:  name,
				Field: fmt.Sprintf("data.cards[%d].uuid", i),
				Err:   fmt.Errorf("duplicate uuid %q (first at index %d)", mc.UUID, first),
			}
		}
		seen[id] = i

		r := Rarity(mc.Rarity)
		if !isValidRarity(r) {
			return nil, &CatalogueError{
				File:  name,
				Field: fmt.Sprintf("data.cards[%d].rarity", i),
				Err:   fmt.Errorf("unknown rarity %q", mc.Rarity),
			}
		}

		finishes := make([]Finish, 0, len(mc.Finishes))
		for j, fin := range mc.Finishes {
			f, ok := normalizeFinish(fin)
			if !ok {
				return nil, &CatalogueError{
					File:  name,
					Field: fmt.Sprintf("data.cards[%d].finishes[%d]", i, j),
					Err:   fmt.Errorf("unknown finish %q", fin),
				}
			}
			finishes = append(finishes, f)
		}

		colors := make([]string, 0, len(mc.Colors))
		for j, col := range mc.Colors {
			if !isValidColor(col) {
				return nil, &CatalogueError{
					File:  name,
					Field: fmt.Sprintf("data.cards[%d].colors[%d]", i, j),
					Err:   fmt.Errorf("unknown color %q", col),
				}
			}
			colors = append(colors, col)
		}

		isBasic := false
		for _, st := range mc.Supertypes {
			if st == "Basic" {
				isBasic = true
				break
			}
		}

		cards[id] = Card{
			ID:          id,
			Name:        mc.Name,
			Number:      mc.Number,
			Rarity:      r,
			Finishes:    finishes,
			IsBasicLand: isBasic,
			IsLand:      slices.Contains(mc.Types, "Land"),
			ScryfallID:  mc.Identifiers.ScryfallID,
			Colors:      colors,
		}
	}
	return cards, nil
}

// Field paths of the two booster definitions MTGJSON uses. A CatalogueError for a file with
// neither names draftField, which callers match to tell a card pool from a set file.
const (
	draftField   = "data.booster.draft"
	defaultField = "data.booster.default"
)

// boosterDefinition returns the booster definition of a set and the field path it was read from,
// which names it in errors. MTGJSON gives most sets a "draft" definition, but the sets before
// Mirage only a "default" one; "draft" wins when a file has both.
func boosterDefinition(boosters map[string]mtgBoosterDef) (def mtgBoosterDef, field string, ok bool) {
	if def, ok = boosters["draft"]; ok {
		return def, draftField, true
	}
	if def, ok = boosters["default"]; ok {
		return def, defaultField, true
	}
	return mtgBoosterDef{}, "", false
}

// LoadSetCatalogue reads one MTGJSON-shaped set file from fsys and returns the
// parsed SetCatalogue. name is used in error messages and must be a valid fs.FS path.
// The booster comes from the file's "draft" definition, or its "default" one when it has no draft.
// Cards that no booster sheet contains are left out of Cards: they cannot be opened, so the
// economy and the collection totals only count cards a player can get.
// Sheet.Cards slices are sorted by ID ascending so that weighted draws over an
// injected seeded *rand.Rand are reproducible across runs.
func LoadSetCatalogue(fsys fs.FS, name string) (SetCatalogue, error) {
	d, err := readAndCheckMetadata(fsys, name)
	if err != nil {
		return SetCatalogue{}, err
	}

	// Validate the release date when present; an empty or absent date loads with ReleaseDate "".
	if d.ReleaseDate != "" {
		if _, err := parseReleaseDate(d.ReleaseDate); err != nil {
			return SetCatalogue{}, &CatalogueError{File: name, Field: "data.releaseDate", Err: err}
		}
	}

	draftDef, boosterField, ok := boosterDefinition(d.Booster)
	if !ok {
		return SetCatalogue{}, &CatalogueError{File: name, Field: draftField}
	}

	cards, err := parseSetCards(d, name)
	if err != nil {
		return SetCatalogue{}, err
	}

	sheets := make(map[string]Sheet, len(draftDef.Sheets))
	for sname, ms := range draftDef.Sheets {
		// Sheet card IDs are not validated against the Cards map: in Time Spiral,
		// booster sheets reference timeshifted cards that live in the separate TSB
		// set file and are absent from the TSP cards array.
		sc := make([]SheetCard, 0, len(ms.Cards))
		weightSum := 0
		for uid, w := range ms.Cards {
			if w <= 0 {
				return SetCatalogue{}, &CatalogueError{
					File:  name,
					Field: fmt.Sprintf("%s.sheets.%s.cards.%s", boosterField, sname, uid),
					Err:   fmt.Errorf("weight must be strictly positive, got %d", w),
				}
			}
			weightSum += w
			sc = append(sc, SheetCard{ID: CardID(uid), Weight: w})
		}
		if weightSum != ms.TotalWeight {
			return SetCatalogue{}, &CatalogueError{
				File:  name,
				Field: fmt.Sprintf("%s.sheets.%s.totalWeight", boosterField, sname),
				Err: fmt.Errorf("totalWeight %d does not match sum of card weights %d",
					ms.TotalWeight, weightSum),
			}
		}
		sort.Slice(sc, func(i, j int) bool { return sc[i].ID < sc[j].ID })
		sheets[sname] = Sheet{
			Name:          sname,
			Cards:         sc,
			TotalWeight:   ms.TotalWeight,
			Foil:          ms.Foil,
			BalanceColors: ms.BalanceColors,
		}
	}

	cards = keepCardsInSheets(cards, sheets)
	addFoilFinishToFoilSheetCards(cards, sheets)

	opts := make([]BoosterOption, len(draftDef.Boosters))
	for i, bo := range draftDef.Boosters {
		if bo.Weight <= 0 {
			return SetCatalogue{}, &CatalogueError{
				File:  name,
				Field: fmt.Sprintf("%s.boosters[%d].weight", boosterField, i),
				Err:   fmt.Errorf("weight must be strictly positive, got %d", bo.Weight),
			}
		}

		// Sort content keys so the first error in alphabetical sheet-name order
		// is always reported; map iteration is random and would make errors
		// nondeterministic.
		contentKeys := make([]string, 0, len(bo.Contents))
		for k := range bo.Contents {
			contentKeys = append(contentKeys, k)
		}
		sort.Strings(contentKeys)

		for _, sheetName := range contentKeys {
			count := bo.Contents[sheetName]
			if count <= 0 {
				return SetCatalogue{}, &CatalogueError{
					File:  name,
					Field: fmt.Sprintf("%s.boosters[%d].contents.%s", boosterField, i, sheetName),
					Err:   fmt.Errorf("count must be strictly positive, got %d", count),
				}
			}
			if _, ok := sheets[sheetName]; !ok {
				return SetCatalogue{}, &CatalogueError{
					File:  name,
					Field: fmt.Sprintf("%s.boosters[%d].contents.%s", boosterField, i, sheetName),
					Err:   fmt.Errorf("references unknown sheet %q", sheetName),
				}
			}
		}

		contents := make(map[string]int, len(bo.Contents))
		for k, v := range bo.Contents {
			contents[k] = v
		}
		opts[i] = BoosterOption{Contents: contents, Weight: bo.Weight}
	}

	return SetCatalogue{
		Code:           d.Code,
		Name:           d.Name,
		ReleaseDate:    d.ReleaseDate,
		Cards:          cards,
		Sheets:         sheets,
		BoosterOptions: opts,
	}, nil
}

// keepCardsInSheets returns the cards that appear in at least one booster sheet. MTGJSON set
// files also list cards no booster holds, such as collector-numbered promos.
func keepCardsInSheets(cards map[CardID]Card, sheets map[string]Sheet) map[CardID]Card {
	inSheet := make(map[CardID]struct{})
	for _, sheet := range sheets {
		for _, sc := range sheet.Cards {
			inSheet[sc.ID] = struct{}{}
		}
	}
	kept := make(map[CardID]Card, len(cards))
	for id, card := range cards {
		if _, ok := inSheet[id]; ok {
			kept[id] = card
		}
	}
	return kept
}

// addFoilFinishToFoilSheetCards gives FinishFoil to every card of cards that a foil sheet holds
// and whose finishes lack it. MTGJSON sometimes lists such a card as nonfoil only, for example
// Tamiyo's Journal in Shadows over Innistrad. The sheet deals it as a foil all the same, so the
// catalogue has to know the finish or the Counter refuses that copy, see ADR-0028.
func addFoilFinishToFoilSheetCards(cards map[CardID]Card, sheets map[string]Sheet) {
	for _, sheet := range sheets {
		if !sheet.Foil {
			continue
		}
		for _, sc := range sheet.Cards {
			card, ok := cards[sc.ID]
			if !ok || hasFinish(card.Finishes, FinishFoil) {
				continue
			}
			card.Finishes = append(card.Finishes, FinishFoil)
			cards[sc.ID] = card
		}
	}
}

// LoadCardPool reads a MTGJSON-shaped set file from fsys and returns a SetCatalogue
// with only Code, Name, and Cards populated. The file need not contain a
// data.booster section. All card-level validation (uuid, rarity, finish, color,
// duplicates) is identical to LoadSetCatalogue. The returned SetCatalogue has
// nil Sheets and nil BoosterOptions.
//
// Use this for supplementary card pools (e.g. TSB) that extend a main set's
// booster sheets but are not directly openable.
func LoadCardPool(fsys fs.FS, name string) (SetCatalogue, error) {
	d, err := readAndCheckMetadata(fsys, name)
	if err != nil {
		return SetCatalogue{}, err
	}

	cards, err := parseSetCards(d, name)
	if err != nil {
		return SetCatalogue{}, err
	}

	return SetCatalogue{
		Code:  d.Code,
		Name:  d.Name,
		Cards: cards,
	}, nil
}
