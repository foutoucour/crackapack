package web

import (
	"errors"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

func revealed(id string, rarity domain.Rarity, isNew bool) play.RevealedCard {
	return play.RevealedCard{
		Card:   domain.Card{ID: domain.CardID(id), Name: id, Rarity: rarity, ScryfallID: "pic-" + id},
		Finish: domain.FinishNonFoil,
		New:    isNew,
	}
}

func noValue(domain.CardID, domain.Finish) (domain.Coins, error) {
	return 0, errors.New("no counter")
}

func TestNewPackView_everyRareAndMythicIsHighlightedNewOrHeldAndNothingElseIs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		rarity domain.Rarity
		isNew  bool
		want   string
	}{
		{"a new rare", domain.RarityRare, true, "rare"},
		{"a new mythic", domain.RarityMythic, true, "mythic"},
		{"a rare held already", domain.RarityRare, false, "rare"},
		{"a mythic held already", domain.RarityMythic, false, "mythic"},
		{"a new uncommon", domain.RarityUncommon, true, ""},
		{"a new common", domain.RarityCommon, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opening := play.Opening{Cards: []play.RevealedCard{revealed("a", tt.rarity, tt.isNew)}}

			view := newPackView(opening, nil, noValue)

			if got := view.Cards[0].Highlight; got != tt.want {
				t.Errorf("Highlight = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewPackView_everyCardCarriesItsReferenceValueWhenTheGameCanSayIt(t *testing.T) {
	t.Parallel()
	opening := play.Opening{Cards: []play.RevealedCard{
		revealed("cheap", domain.RarityCommon, false),
		revealed("dear", domain.RarityRare, false),
	}}
	values := map[domain.CardID]domain.Coins{"cheap": 3, "dear": 250}
	value := func(id domain.CardID, _ domain.Finish) (domain.Coins, error) { return values[id], nil }

	view := newPackView(opening, nil, value)

	if got, want := []domain.Coins{view.Cards[0].Value, view.Cards[1].Value}, []domain.Coins{3, 250}; !slices.Equal(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}
}

func TestNewPackView_aGameWithoutACounterShowsNoValue(t *testing.T) {
	t.Parallel()
	opening := play.Opening{Cards: []play.RevealedCard{revealed("a", domain.RarityRare, true)}}

	view := newPackView(opening, nil, noValue)

	if got := view.Cards[0].Value; got != 0 {
		t.Errorf("Value = %d, want 0, which the page does not show", got)
	}
}

func TestNewPackView_preloadsThePicturesOfTheFirstTheSecondAndTheLastCard(t *testing.T) {
	t.Parallel()
	card := func(n int) play.RevealedCard {
		return revealed(string(rune('a'+n)), domain.RarityCommon, false)
	}
	withoutPicture := card(1)
	withoutPicture.Card.ScryfallID = ""

	tests := []struct {
		name  string
		cards []play.RevealedCard
		want  []string
	}{
		{"a full booster", []play.RevealedCard{card(0), card(1), card(2), card(3), card(4)}, []string{"pic-a", "pic-b", "pic-e"}},
		{"two cards", []play.RevealedCard{card(0), card(1)}, []string{"pic-a", "pic-b"}},
		{"one card", []play.RevealedCard{card(0)}, []string{"pic-a"}},
		{"a card without a picture is skipped", []play.RevealedCard{card(0), withoutPicture, card(2), card(3)}, []string{"pic-a", "pic-d"}},
		{"no card", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			view := newPackView(play.Opening{Cards: tt.cards}, nil, noValue)

			if !slices.Equal(view.Preload, tt.want) {
				t.Errorf("Preload = %v, want %v", view.Preload, tt.want)
			}
		})
	}
}
