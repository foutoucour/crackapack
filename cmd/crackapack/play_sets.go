package main

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// allSets is the --sets value that plays every set in scope of the catalogue directory.
const allSets = "all"

// loadScope loads the sets of the catalogue directory that are in the product's scope, see
// docs/adr/0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md: openable,
// released from Mirage on, and with a booster worth under the expensive limit. pools are the card
// pools of the directory that their boosters draw from, and reference is the common bundle that
// the energy costs are relative to, as for a list of sets.
func loadScope(catFS fs.FS, prices domain.PriceSnapshot, cfg config.Config) (sets, pools, reference []domain.SetCatalogue, err error) {
	openable, err := loadReferenceCatalogues(catFS, nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if reference, err = leaveOutExpensiveSets(nil, openable, prices, cfg); err != nil {
		return nil, nil, nil, err
	}
	for _, cat := range reference {
		if cat.ReleaseDate >= domain.FirstReleaseInScope {
			sets = append(sets, cat)
		}
	}
	if len(sets) == 0 {
		return nil, nil, nil, errors.New("no set of the catalogue directory is in scope: openable, released from Mirage on and under the expensive booster value")
	}
	if pools, err = loadDrawnPools(catFS, openable, sets); err != nil {
		return nil, nil, nil, err
	}
	return sets, pools, reference, nil
}

// loadDrawnPools loads each file of the catalogue directory that is not an openable set and holds a
// card that the booster sheets of sets draw and that no set holds, such as TSB for Time Spiral. The
// other files, such as the sets that MTGJSON gives no booster the loader reads, are left out.
func loadDrawnPools(catFS fs.FS, openable, sets []domain.SetCatalogue) ([]domain.SetCatalogue, error) {
	isOpenable := make(map[string]bool, len(openable))
	for _, cat := range openable {
		isOpenable[cat.Code+".json"] = true
	}
	drawn := make(map[domain.CardID]bool)
	for _, cat := range sets {
		for _, sheet := range cat.Sheets {
			for _, sc := range sheet.Cards {
				if _, held := cat.Cards[sc.ID]; !held {
					drawn[sc.ID] = true
				}
			}
		}
	}
	entries, err := fs.ReadDir(catFS, ".")
	if err != nil {
		return nil, fmt.Errorf("catalogue directory: %w", err)
	}
	var pools []domain.SetCatalogue
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !setFilePattern.MatchString(name) || isOpenable[name] {
			continue
		}
		pool, err := domain.LoadCardPool(catFS, name)
		if err != nil {
			return nil, fileError("supplementary", name, err)
		}
		for id := range pool.Cards {
			if drawn[id] {
				pools = append(pools, pool)
				break
			}
		}
	}
	return pools, nil
}
