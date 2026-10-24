package play_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func TestPacks_carryTheIconCodeOfTheirSet(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	rav.IconCode = "RAV"
	g := newGame(t, rav)

	if got := g.Packs()[0].IconCode; got != "RAV" {
		t.Errorf("Packs()[0].IconCode = %q, want %q", got, "RAV")
	}
}

func TestBinders_carryTheIconCodeOfTheirSet(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	rav.IconCode = "RAV"
	g := newGame(t, rav)

	if got := g.Binders()[0].IconCode; got != "RAV" {
		t.Errorf("Binders()[0].IconCode = %q, want %q", got, "RAV")
	}
	binder, err := g.Binder(rav.Code)
	if err != nil {
		t.Fatalf("Binder: %v", err)
	}
	if binder.IconCode != "RAV" {
		t.Errorf("Binder.IconCode = %q, want %q", binder.IconCode, "RAV")
	}
}

func TestOpen_carriesTheIconCodeOfTheSet(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	rav.IconCode = "RAV"
	g := newGame(t, rav)

	opening, err := g.Open(rav.Code)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opening.IconCode != "RAV" {
		t.Errorf("Opening.IconCode = %q, want %q", opening.IconCode, "RAV")
	}
}
