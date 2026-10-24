package domaintest

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
)

// exportFile is the top-level MTGJSON v5 envelope used by the serialisers in
// this package. It mirrors the unexported mtgSetFile in catalogue_loader.go.
type exportFile struct {
	Data exportData `json:"data"`
}

type exportData struct {
	Code        string                      `json:"code"`
	Name        string                      `json:"name"`
	ReleaseDate string                      `json:"releaseDate,omitempty"`
	KeyruneCode string                      `json:"keyruneCode,omitempty"`
	Cards       []exportCard                `json:"cards"`
	Booster     map[string]exportBoosterDef `json:"booster,omitempty"`
}

type exportCard struct {
	UUID        string            `json:"uuid"`
	Name        string            `json:"name"`
	Number      string            `json:"number"`
	Rarity      string            `json:"rarity"`
	Finishes    []string          `json:"finishes"`
	Types       []string          `json:"types,omitempty"`
	Supertypes  []string          `json:"supertypes"`
	Colors      []string          `json:"colors,omitempty"`
	Identifiers exportIdentifiers `json:"identifiers"`
}

type exportIdentifiers struct {
	ScryfallID string `json:"scryfallId,omitempty"`
}

type exportBoosterDef struct {
	Boosters []exportBoosterOption  `json:"boosters"`
	Sheets   map[string]exportSheet `json:"sheets"`
}

type exportBoosterOption struct {
	Contents map[string]int `json:"contents"`
	Weight   int            `json:"weight"`
}

type exportSheet struct {
	Cards         map[string]int `json:"cards"`
	TotalWeight   int            `json:"totalWeight"`
	Foil          bool           `json:"foil"`
	BalanceColors bool           `json:"balanceColors"`
}

// CatalogueToJSON serialises cat to MTGJSON v5-shaped JSON that
// domain.LoadSetCatalogue can parse back to a functionally equivalent
// SetCatalogue. Cards are emitted in ascending UUID order for determinism.
// The booster section is built from cat.Sheets and cat.BoosterOptions.
func CatalogueToJSON(cat domain.SetCatalogue) ([]byte, error) {
	cards := buildExportCards(cat)
	sheets := buildExportSheets(cat.Sheets)
	boosters := buildExportBoosters(cat.BoosterOptions)

	f := exportFile{
		Data: exportData{
			Code:        cat.Code,
			Name:        cat.Name,
			ReleaseDate: cat.ReleaseDate,
			KeyruneCode: cat.IconCode,
			Cards:       cards,
			Booster: map[string]exportBoosterDef{
				"draft": {
					Boosters: boosters,
					Sheets:   sheets,
				},
			},
		},
	}
	data, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("domaintest: CatalogueToJSON %q: %w", cat.Code, err)
	}
	return data, nil
}

// CardPoolToJSON serialises cat to MTGJSON v5-shaped JSON without a booster
// section. domain.LoadCardPool can parse the result back to a SetCatalogue
// whose Code, Name, and Cards match cat. Sheets and BoosterOptions are not
// preserved because the format does not include them for a card pool.
func CardPoolToJSON(cat domain.SetCatalogue) ([]byte, error) {
	cards := buildExportCards(cat)

	f := exportFile{
		Data: exportData{
			Code:  cat.Code,
			Name:  cat.Name,
			Cards: cards,
			// Booster omitted (omitempty) — LoadCardPool does not require it.
		},
	}
	data, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("domaintest: CardPoolToJSON %q: %w", cat.Code, err)
	}
	return data, nil
}

// PriceSnapshotToJSON serialises snap to the JSON format that
// domain.LoadPriceSnapshot accepts: a JSON object mapping string UUID to
// integer USDCents.
func PriceSnapshotToJSON(snap domain.PriceSnapshot) ([]byte, error) {
	m := make(map[string]int64, len(snap))
	for id, cents := range snap {
		m[string(id)] = int64(cents)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("domaintest: PriceSnapshotToJSON: %w", err)
	}
	return data, nil
}

// MergePrices merges multiple PriceSnapshots into one. If a CardID appears in
// more than one snapshot the last occurrence (rightmost argument) wins. Card IDs
// across the domaintest builders are distinct, so this is typically safe to call
// with one snapshot per catalogue.
func MergePrices(snaps ...domain.PriceSnapshot) domain.PriceSnapshot {
	result := make(domain.PriceSnapshot)
	for _, snap := range snaps {
		for id, price := range snap {
			result[id] = price
		}
	}
	return result
}

// NewMapFS builds an fstest.MapFS suitable for use with domain.LoadSetCatalogue,
// domain.LoadCardPool, and domain.LoadPriceSnapshot:
//
//   - Each catalogue in cats is written as "<CODE>.json" via CatalogueToJSON.
//   - Each catalogue in supps is written as "<CODE>.json" via CardPoolToJSON
//     (no booster section required in the file).
//   - snap is written as "prices.json" via PriceSnapshotToJSON.
//
// If a code appears in both cats and supps the supplementary entry wins.
// Returns an error if any serialisation step fails.
func NewMapFS(cats []domain.SetCatalogue, supps []domain.SetCatalogue, snap domain.PriceSnapshot) (fstest.MapFS, error) {
	mapFS := make(fstest.MapFS)

	for _, cat := range cats {
		data, err := CatalogueToJSON(cat)
		if err != nil {
			return nil, err
		}
		mapFS[cat.Code+".json"] = &fstest.MapFile{Data: data}
	}

	for _, cat := range supps {
		data, err := CardPoolToJSON(cat)
		if err != nil {
			return nil, err
		}
		mapFS[cat.Code+".json"] = &fstest.MapFile{Data: data}
	}

	pricesData, err := PriceSnapshotToJSON(snap)
	if err != nil {
		return nil, err
	}
	mapFS["prices.json"] = &fstest.MapFile{Data: pricesData}

	return mapFS, nil
}

// buildExportCards converts the Cards map of a SetCatalogue to a sorted slice
// of exportCard. Cards are sorted by ascending UUID for deterministic JSON output.
func buildExportCards(cat domain.SetCatalogue) []exportCard {
	ids := make([]domain.CardID, 0, len(cat.Cards))
	for id := range cat.Cards {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := make([]exportCard, 0, len(cat.Cards))
	for _, id := range ids {
		c := cat.Cards[id]

		// supertypes: always output the array; "Basic" for basic lands, empty for others.
		supertypes := []string{}
		if c.IsBasicLand {
			supertypes = []string{"Basic"}
		}

		var types []string
		if c.IsLand {
			types = []string{"Land"}
		}

		finishes := make([]string, len(c.Finishes))
		for i, f := range c.Finishes {
			finishes[i] = string(f)
		}

		out = append(out, exportCard{
			UUID:       string(id),
			Name:       c.Name,
			Number:     c.Number,
			Rarity:     string(c.Rarity),
			Finishes:   finishes,
			Types:      types,
			Supertypes: supertypes,
			Colors:     c.Colors, // omitempty handles nil/empty
			Identifiers: exportIdentifiers{
				ScryfallID: c.ScryfallID,
			},
		})
	}
	return out
}

// buildExportSheets converts the Sheets map of a SetCatalogue to a map of
// exportSheet, suitable for inclusion in the booster.draft section.
func buildExportSheets(sheets map[string]domain.Sheet) map[string]exportSheet {
	out := make(map[string]exportSheet, len(sheets))
	for name, s := range sheets {
		cards := make(map[string]int, len(s.Cards))
		for _, sc := range s.Cards {
			cards[string(sc.ID)] = sc.Weight
		}
		out[name] = exportSheet{
			Cards:         cards,
			TotalWeight:   s.TotalWeight,
			Foil:          s.Foil,
			BalanceColors: s.BalanceColors,
		}
	}
	return out
}

// buildExportBoosters converts a slice of domain.BoosterOption to the export form.
func buildExportBoosters(opts []domain.BoosterOption) []exportBoosterOption {
	out := make([]exportBoosterOption, len(opts))
	for i, opt := range opts {
		contents := make(map[string]int, len(opt.Contents))
		for k, v := range opt.Contents {
			contents[k] = v
		}
		out[i] = exportBoosterOption{
			Contents: contents,
			Weight:   opt.Weight,
		}
	}
	return out
}
