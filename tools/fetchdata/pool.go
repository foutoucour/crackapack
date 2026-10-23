package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// borrowedPool describes a card pool that fetchdata derives for a set whose booster sheets hold
// cards that MTGJSON lists in the files of other sets. Those sets are openable and keep only the
// cards of their own sheets, so a donor file cannot be passed whole as a supplementary pool: its
// cards would be in two catalogues. The pool holds only the borrowed cards.
type borrowedPool struct {
	host   string   // the set whose sheets borrow
	code   string   // the code of the pool, and the name of its file
	name   string   // the display name of the pool
	donors []string // the sets whose files hold the borrowed cards
}

// borrowedPools returns the pools the real MTGJSON data needs. Phyrexia: All Will Be One draws the
// Phyrexian Praetor concept cards of Dominaria United, Kamigawa: Neon Dynasty, Kaldheim and
// Streets of New Capenna, see ADR-0047.
func borrowedPools() []borrowedPool {
	return []borrowedPool{{
		host:   "ONE",
		code:   "PRAETORS",
		name:   "Phyrexian Praetors (borrowed by ONE)",
		donors: []string{"DMU", "NEO", "KHM", "SNC"},
	}}
}

// withDonors returns codes followed by the donors of every requested host that are not already
// requested, so that asking for a host fetches what its pool is made of.
func withDonors(codes []string, pools []borrowedPool) []string {
	all := slices.Clone(codes)
	for _, pool := range pools {
		if !slices.Contains(codes, pool.host) {
			continue
		}
		for _, donor := range pool.donors {
			if !slices.Contains(all, donor) {
				all = append(all, donor)
			}
		}
	}
	return all
}

// writeBorrowedPool writes the file of the pool next to the set files in dir, and returns how many
// cards it holds. The pool is the cards that the booster sheets of the host name and the host file
// does not hold, taken as MTGJSON gives them from the donor files.
func writeBorrowedPool(dir string, pool borrowedPool) (int, error) {
	borrowed, err := borrowedIDs(filepath.Join(dir, pool.host+".json"))
	if err != nil {
		return 0, fmt.Errorf("pool %s: %w", pool.code, err)
	}

	found := make(map[string]json.RawMessage, len(borrowed))
	for _, donor := range pool.donors {
		cards, err := donorCards(filepath.Join(dir, donor+".json"))
		if err != nil {
			return 0, fmt.Errorf("pool %s: %w", pool.code, err)
		}
		for id := range borrowed {
			if card, ok := cards[id]; ok {
				found[id] = card
			}
		}
	}
	if missing := missingIDs(borrowed, found); len(missing) > 0 {
		return 0, fmt.Errorf("pool %s: the booster sheets of %s hold %d cards that no donor (%s) has: %s",
			pool.code, pool.host, len(missing), strings.Join(pool.donors, ", "), strings.Join(missing, ", "))
	}

	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cards := make([]json.RawMessage, len(ids))
	for i, id := range ids {
		cards[i] = found[id]
	}

	path := filepath.Join(dir, pool.code+".json")
	err = writeFileAtomic(path, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(poolDocument{Data: poolData{Code: pool.code, Name: pool.name, Cards: cards}})
	})
	return len(cards), err
}

type poolDocument struct {
	Data poolData `json:"data"`
}

type poolData struct {
	Code  string            `json:"code"`
	Name  string            `json:"name"`
	Cards []json.RawMessage `json:"cards"`
}

// borrowedIDs returns the cards on the booster sheets of the set file at path that the file does
// not hold. Like the loader, it reads the draft booster, or the default one when there is no draft.
func borrowedIDs(path string) (map[string]struct{}, error) {
	var set struct {
		Data struct {
			Cards []struct {
				UUID string `json:"uuid"`
			} `json:"cards"`
			Booster map[string]struct {
				Sheets map[string]struct {
					Cards map[string]int `json:"cards"`
				} `json:"sheets"`
			} `json:"booster"`
		} `json:"data"`
	}
	if err := readJSON(path, &set); err != nil {
		return nil, err
	}

	own := make(map[string]struct{}, len(set.Data.Cards))
	for _, c := range set.Data.Cards {
		own[c.UUID] = struct{}{}
	}
	definition, ok := set.Data.Booster["draft"]
	if !ok {
		definition = set.Data.Booster["default"]
	}
	borrowed := make(map[string]struct{})
	for _, sheet := range definition.Sheets {
		for id := range sheet.Cards {
			if _, isOwn := own[id]; !isOwn {
				borrowed[id] = struct{}{}
			}
		}
	}
	return borrowed, nil
}

// donorCards returns the cards of the set file at path by UUID, each as the raw MTGJSON object.
func donorCards(path string) (map[string]json.RawMessage, error) {
	var set struct {
		Data struct {
			Cards []json.RawMessage `json:"cards"`
		} `json:"data"`
	}
	if err := readJSON(path, &set); err != nil {
		return nil, err
	}
	byID := make(map[string]json.RawMessage, len(set.Data.Cards))
	for _, raw := range set.Data.Cards {
		var card struct {
			UUID string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &card); err != nil {
			return nil, fmt.Errorf("decode a card of %s: %w", path, err)
		}
		byID[card.UUID] = raw
	}
	return byID, nil
}

func readJSON(path string, into any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read catalogue: %w", err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("decode catalogue %s: %w", path, err)
	}
	return nil
}

// missingIDs returns the borrowed cards that were not found, sorted.
func missingIDs(borrowed map[string]struct{}, found map[string]json.RawMessage) []string {
	var missing []string
	for id := range borrowed {
		if _, ok := found[id]; !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}
