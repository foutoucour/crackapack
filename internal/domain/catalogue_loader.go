package domain

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
)

// mtgSetFile is the top-level MTGJSON v5 set file envelope.
type mtgSetFile struct {
	Data mtgSetData `json:"data"`
}

type mtgSetData struct {
	Code    string                   `json:"code"`
	Name    string                   `json:"name"`
	Cards   []mtgCard                `json:"cards"`
	Booster map[string]mtgBoosterDef `json:"booster"`
}

type mtgCard struct {
	UUID       string   `json:"uuid"`
	Name       string   `json:"name"`
	Number     string   `json:"number"`
	Rarity     string   `json:"rarity"`
	Finishes   []string `json:"finishes"`
	Supertypes []string `json:"supertypes"`
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

// isValidRarity reports whether r is a recognised rarity for V1 sets.
// Unknown strings (mythic, bonus, empty) are rejected by the caller.
func isValidRarity(r Rarity) bool {
	switch r {
	case RarityCommon, RarityUncommon, RarityRare, RaritySpecial:
		return true
	default:
		return false
	}
}

// LoadSetCatalogue reads one MTGJSON-shaped set file from fsys and returns the
// parsed SetCatalogue. name is used in error messages and must be a valid fs.FS path.
// Sheet.Cards slices are sorted by ID ascending so that weighted draws over an
// injected seeded *rand.Rand are reproducible across runs.
func LoadSetCatalogue(fsys fs.FS, name string) (SetCatalogue, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return SetCatalogue{}, &CatalogueError{File: name, Field: "(file)", Err: err}
	}

	var raw mtgSetFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return SetCatalogue{}, &CatalogueError{File: name, Field: "(json)", Err: err}
	}

	d := raw.Data
	if d.Code == "" {
		return SetCatalogue{}, &CatalogueError{File: name, Field: "data.code"}
	}
	if d.Name == "" {
		return SetCatalogue{}, &CatalogueError{File: name, Field: "data.name"}
	}
	draftDef, ok := d.Booster["draft"]
	if !ok {
		return SetCatalogue{}, &CatalogueError{File: name, Field: "data.booster.draft"}
	}

	cards := make(map[CardID]Card, len(d.Cards))
	for i, mc := range d.Cards {
		r := Rarity(mc.Rarity)
		if !isValidRarity(r) {
			return SetCatalogue{}, &CatalogueError{
				File:  name,
				Field: fmt.Sprintf("data.cards[%d].rarity", i),
				Err:   fmt.Errorf("unknown rarity %q", mc.Rarity),
			}
		}
		isBasic := false
		for _, st := range mc.Supertypes {
			if st == "Basic" {
				isBasic = true
				break
			}
		}
		finishes := make([]Finish, len(mc.Finishes))
		for j, fin := range mc.Finishes {
			finishes[j] = Finish(fin)
		}
		id := CardID(mc.UUID)
		cards[id] = Card{
			ID:          id,
			Name:        mc.Name,
			Number:      mc.Number,
			Rarity:      r,
			Finishes:    finishes,
			IsBasicLand: isBasic,
			ScryfallID:  mc.Identifiers.ScryfallID,
		}
	}

	sheets := make(map[string]Sheet, len(draftDef.Sheets))
	for sname, ms := range draftDef.Sheets {
		// Sheet card IDs are not validated against the Cards map: in Time Spiral,
		// booster sheets reference timeshifted cards that live in the separate TSB
		// set file and are absent from the TSP cards array.
		sc := make([]SheetCard, 0, len(ms.Cards))
		for uid, w := range ms.Cards {
			sc = append(sc, SheetCard{ID: CardID(uid), Weight: w})
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

	opts := make([]BoosterOption, len(draftDef.Boosters))
	for i, bo := range draftDef.Boosters {
		contents := make(map[string]int, len(bo.Contents))
		for k, v := range bo.Contents {
			contents[k] = v
		}
		opts[i] = BoosterOption{Contents: contents, Weight: bo.Weight}
	}

	return SetCatalogue{
		Code:           d.Code,
		Name:           d.Name,
		Cards:          cards,
		Sheets:         sheets,
		BoosterOptions: opts,
	}, nil
}
