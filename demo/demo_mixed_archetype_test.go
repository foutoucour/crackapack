package demo_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// archivistEgoStingyCrowd builds size NPCs of the embedded archivist_ego_stingy archetype over the
// demo catalogues and returns them with the catalogues. Building the crowd fills the wish lists.
func archivistEgoStingyCrowd(t *testing.T, size int) ([]*domain.NPC, []domain.SetCatalogue) {
	t.Helper()
	ds := demo.Build()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := cfg.NPCs.Archetypes["archivist_ego_stingy"]
	cats, _, _ := loadDemoInputs(t, ds)

	prices := domain.PriceSnapshot{}
	rng := rand.New(rand.NewPCG(42, 42))
	spec := domain.ArchetypeSpec{
		Share:             1,
		MinBoostersPerDay: a.MinBoostersPerDay,
		MaxBoostersPerDay: a.MaxBoostersPerDay,
		KeepCopies:        a.KeepCopies,
		FocusSet:          domain.FocusSetMode(a.FocusSet),
		Behaviour:         a.Behaviour,
		WishSize:          a.WishSize,
		MarginPct:         a.MarginPct,
		OfferChancePct:    a.OfferChancePct,
		Colors:            a.Colors,
	}
	for _, name := range a.Traits {
		spec.Traits = append(spec.Traits, domain.TraitSpec{Name: name, Params: a.TraitParams[name]})
	}
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{
		Size:          size,
		Archetypes:    map[string]domain.ArchetypeSpec{"archivist_ego_stingy": spec},
		Valuer:        domain.NewValuer(prices, 10),
		TraitRegistry: domain.DefaultTraits(),
		MaxMarginPct:  cfg.NPCs.MaxMarginPct,
	}, cats, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	counter, err := domain.NewCounter(cats, prices, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 10})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{
		domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
	}, rng)
	if _, err := domain.NewCrowd(npcs, cats, opener, counter, rng); err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	return npcs, cats
}

func TestArchivistEgoStingy_wishesOnlyWUBCards(t *testing.T) {
	t.Parallel()
	npcs, cats := archivistEgoStingyCrowd(t, 3)

	cards := make(map[domain.CardID]domain.Card)
	for _, cat := range cats {
		for id, card := range cat.Cards {
			cards[id] = card
		}
	}
	for _, npc := range npcs {
		for _, wish := range npc.Wishes() {
			card, ok := cards[wish.ID]
			if !ok {
				t.Errorf("NPC %d wishes for %q, which is in no catalogue", npc.ID, wish.ID)
				continue
			}
			for _, colour := range card.Colors {
				if colour != "W" && colour != "U" && colour != "B" {
					t.Errorf("NPC %d wishes for %q of colour %q, want only W, U, B or colourless",
						npc.ID, wish.ID, colour)
				}
			}
		}
	}
}

// TestArchivistEgoStingy_ranksWishesTowardOlderSet shows that the archivist trait is what ranks
// the wishes of this archetype: it has no focus set, so cards of both demo sets are candidates, and
// the older one, DEM, must come first.
func TestArchivistEgoStingy_ranksWishesTowardOlderSet(t *testing.T) {
	t.Parallel()
	npcs, cats := archivistEgoStingyCrowd(t, 1)

	setOf := make(map[domain.CardID]string)
	dates := make(map[string]string)
	for _, cat := range cats {
		dates[cat.Code] = cat.ReleaseDate
		for id := range cat.Cards {
			setOf[id] = cat.Code
		}
	}
	if dates["DEM"] == "" || dates["DSP"] == "" || dates["DEM"] >= dates["DSP"] {
		t.Fatalf("want DEM older than DSP, got DEM=%q DSP=%q", dates["DEM"], dates["DSP"])
	}

	wishes := npcs[0].Wishes()
	if len(wishes) == 0 {
		t.Fatal("the NPC has no wishes after NewCrowd")
	}
	if got := setOf[wishes[0].ID]; got != "DEM" {
		t.Errorf("first wish %q is from set %q, want DEM, the older set", wishes[0].ID, got)
	}
}
