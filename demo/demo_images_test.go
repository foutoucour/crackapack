package demo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/images"
)

// loadDemoCards loads every card of the demo files, openable sets and card pools alike.
func loadDemoCards(t *testing.T, ds demo.Dataset) []domain.Card {
	t.Helper()
	var cards []domain.Card
	for _, code := range ds.SetCodes {
		cat, err := domain.LoadSetCatalogue(ds.Files, code+".json")
		if err != nil {
			t.Fatalf("load %s: %v", code, err)
		}
		for _, c := range cat.Cards {
			cards = append(cards, c)
		}
	}
	for _, code := range ds.SupplementaryCodes {
		pool, err := domain.LoadCardPool(ds.Files, code+".json")
		if err != nil {
			t.Fatalf("load %s: %v", code, err)
		}
		for _, c := range pool.Cards {
			cards = append(cards, c)
		}
	}
	return cards
}

func TestBuild_everyCardHasAUniqueValidScryfallID(t *testing.T) {
	t.Parallel()
	seen := make(map[string]domain.CardID)

	for _, c := range loadDemoCards(t, demo.Build()) {
		if !images.ValidID(c.ScryfallID) {
			t.Errorf("card %q has Scryfall id %q, want a valid one", c.ID, c.ScryfallID)
		}
		if other, dup := seen[c.ScryfallID]; dup {
			t.Errorf("cards %q and %q share Scryfall id %q", other, c.ID, c.ScryfallID)
		}
		seen[c.ScryfallID] = c.ID
	}
}

func TestBuild_isStillDeterministicWithTheIDs(t *testing.T) {
	t.Parallel()
	first, second := loadDemoCards(t, demo.Build()), loadDemoCards(t, demo.Build())

	ids := func(cards []domain.Card) map[domain.CardID]string {
		m := make(map[domain.CardID]string, len(cards))
		for _, c := range cards {
			m[c.ID] = c.ScryfallID
		}
		return m
	}
	a, b := ids(first), ids(second)
	for id, want := range a {
		if b[id] != want {
			t.Errorf("card %q: Scryfall id %q then %q across two builds", id, want, b[id])
		}
	}
}

func TestDemoImages_drawTheCardAsAnSVGNamedAfterIt(t *testing.T) {
	t.Parallel()
	ds := demo.Build()
	var card domain.Card
	for _, c := range loadDemoCards(t, ds) {
		if c.Rarity == domain.RarityRare {
			card = c
			break
		}
	}

	img, err := ds.Images.Image(context.Background(), card.ScryfallID)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}

	if img.ContentType != "image/svg+xml" {
		t.Errorf("ContentType = %q, want image/svg+xml", img.ContentType)
	}
	if !strings.Contains(string(img.Data), card.Name) {
		t.Errorf("the picture does not show the card name %q:\n%s", card.Name, img.Data)
	}
}

func TestDemoImages_errorsLikeARealSource(t *testing.T) {
	t.Parallel()
	ds := demo.Build()

	if _, err := ds.Images.Image(context.Background(), "ffffffff-ffff-4fff-8fff-ffffffffffff"); !errors.Is(err, images.ErrNotFound) {
		t.Errorf("unknown id error = %v, want ErrNotFound", err)
	}
	if _, err := ds.Images.Image(context.Background(), "nope"); !errors.Is(err, images.ErrInvalidID) {
		t.Errorf("bad id error = %v, want ErrInvalidID", err)
	}
}
