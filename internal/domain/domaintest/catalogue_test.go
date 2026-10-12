package domaintest_test

import (
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func TestNewCatalogue_allRarities(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)

	found := map[domain.Rarity]bool{}
	for _, c := range cat.Cards {
		found[c.Rarity] = true
	}
	for _, r := range []domain.Rarity{domain.RarityCommon, domain.RarityUncommon, domain.RarityRare} {
		if !found[r] {
			t.Errorf("no card with rarity %q", r)
		}
	}
}

func TestNewCatalogue_hasBasicLands(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)

	for _, c := range cat.Cards {
		if c.IsBasicLand {
			return
		}
	}
	t.Error("no basic land card found")
}

func TestNewCatalogue_hasFoilCards(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)

	for _, c := range cat.Cards {
		for _, f := range c.Finishes {
			if f == domain.FinishFoil {
				return
			}
		}
	}
	t.Error("no foil finish found on any card")
}

func TestNewCatalogue_hasColorshiftedSheet(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)

	sheet, ok := cat.Sheets["colorshifted"]
	if !ok {
		t.Fatal("colorshifted sheet missing")
	}
	// Sheet must mix uncommons (weight 30) and rares (weight 10).
	hasUncommonWeight := false
	hasRareWeight := false
	for _, sc := range sheet.Cards {
		c, exists := cat.Cards[sc.ID]
		if !exists {
			t.Errorf("colorshifted sheet card %q not in Cards map", sc.ID)
			continue
		}
		switch c.Rarity {
		case domain.RarityUncommon:
			if sc.Weight == 30 {
				hasUncommonWeight = true
			}
		case domain.RarityRare:
			if sc.Weight == 10 {
				hasRareWeight = true
			}
		case domain.RarityCommon, domain.RaritySpecial:
			// colorshifted sheet contains only uncommons and rares; other rarities are ignored here
		}
	}
	if !hasUncommonWeight {
		t.Error("colorshifted sheet: no uncommon with weight 30")
	}
	if !hasRareWeight {
		t.Error("colorshifted sheet: no rare with weight 10")
	}
}

func TestNewCatalogue_sheetCardsAreSortedByID(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)

	for sname, sheet := range cat.Sheets {
		for i := 1; i < len(sheet.Cards); i++ {
			if sheet.Cards[i].ID < sheet.Cards[i-1].ID {
				t.Errorf("sheet %q not sorted: Cards[%d].ID %q < Cards[%d].ID %q",
					sname, i, sheet.Cards[i].ID, i-1, sheet.Cards[i-1].ID)
			}
		}
	}
}

func TestNewCatalogue_deterministic(t *testing.T) {
	t.Parallel()
	cat1 := domaintest.NewCatalogue(7)
	cat2 := domaintest.NewCatalogue(7)

	if !reflect.DeepEqual(cat1, cat2) {
		t.Error("two calls with the same seed produced different catalogues")
	}
}

func TestNewCatalogue_differentSeedsDiffer(t *testing.T) {
	t.Parallel()
	cat1 := domaintest.NewCatalogue(1)
	cat2 := domaintest.NewCatalogue(2)

	// The sheets use rng-generated weights; different seeds must produce different weights.
	equal := true
	for sname, s1 := range cat1.Sheets {
		s2, ok := cat2.Sheets[sname]
		if !ok {
			equal = false
			break
		}
		for i := range s1.Cards {
			if i >= len(s2.Cards) || s1.Cards[i].Weight != s2.Cards[i].Weight {
				equal = false
				break
			}
		}
	}
	if equal {
		t.Error("different seeds produced identical catalogues")
	}
}

func TestNewTimeshiftedCatalogue_onlySpecialRarity(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewTimeshiftedCatalogue(42)

	if len(cat.Cards) == 0 {
		t.Fatal("timeshifted catalogue has no cards")
	}
	for _, c := range cat.Cards {
		if c.Rarity != domain.RaritySpecial {
			t.Errorf("card %q: Rarity = %q, want %q", c.ID, c.Rarity, domain.RaritySpecial)
		}
	}
}

func TestNewTimeshiftedCatalogue_deterministic(t *testing.T) {
	t.Parallel()
	cat1 := domaintest.NewTimeshiftedCatalogue(99)
	cat2 := domaintest.NewTimeshiftedCatalogue(99)

	if !reflect.DeepEqual(cat1, cat2) {
		t.Error("two calls with the same seed produced different timeshifted catalogues")
	}
}

func TestNewPrices_hasBelowTenCent(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	prices := domaintest.NewPrices(cat, 42)

	for _, p := range prices {
		if p < 10 {
			return
		}
	}
	t.Error("no card priced below 10 USDCents")
}

func TestNewPrices_hasMissingCard(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	prices := domaintest.NewPrices(cat, 42)

	missing := 0
	for id := range cat.Cards {
		if _, ok := prices[id]; !ok {
			missing++
		}
	}
	if missing == 0 {
		t.Error("every card has a price; want at least one with no entry")
	}
}

func TestNewPrices_deterministic(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	p1 := domaintest.NewPrices(cat, 42)
	p2 := domaintest.NewPrices(cat, 42)

	if !reflect.DeepEqual(p1, p2) {
		t.Error("two calls with the same seed produced different price snapshots")
	}
}

func TestNewCatalogue_boostersHaveFifteenCards(t *testing.T) {
	t.Parallel()
	const boosterSize = 15
	cat := domaintest.NewCatalogue(42)

	for i, opt := range cat.BoosterOptions {
		total := 0
		for _, n := range opt.Contents {
			total += n
		}
		if total != boosterSize {
			t.Errorf("booster option %d has %d cards, want %d", i, total, boosterSize)
		}
	}
}
