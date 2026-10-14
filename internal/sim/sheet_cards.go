package sim

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// checkSheetCards returns an error when a booster sheet of an openable set holds a card that no
// loaded catalogue has, see ADR-0016. Time Spiral's sheets, for instance, reference the timeshifted
// cards of TSB: without that pool the Counter does not know them, and a run with a supply or a
// crowd fails late with "card not in catalogue". The error names the set, the number of missing
// cards and the smallest missing ID, so that it does not vary from run to run.
func checkSheetCards(openable, supplementary []domain.SetCatalogue) error {
	known := make(map[domain.CardID]struct{})
	for _, group := range [][]domain.SetCatalogue{openable, supplementary} {
		for _, cat := range group {
			for id := range cat.Cards {
				known[id] = struct{}{}
			}
		}
	}
	for _, cat := range openable {
		missing := make(map[domain.CardID]struct{})
		for _, sheet := range cat.Sheets {
			for _, sc := range sheet.Cards {
				if _, ok := known[sc.ID]; !ok {
					missing[sc.ID] = struct{}{}
				}
			}
		}
		if len(missing) == 0 {
			continue
		}
		var example domain.CardID
		for id := range missing {
			if example == "" || id < example {
				example = id
			}
		}
		return fmt.Errorf("sim: the booster sheets of %q hold %d cards that no loaded catalogue has, for example %q: pass the pool that holds them with --supplementary",
			cat.Code, len(missing), example)
	}
	return nil
}
