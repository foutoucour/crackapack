package domain_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func milestoneBinder() domain.SetCatalogue {
	return domain.SetCatalogue{Code: "TST", Name: "Test", Cards: map[domain.CardID]domain.Card{
		"c-white": {ID: "c-white", Rarity: domain.RarityCommon, Colors: []string{"W"}},
		"c-blue":  {ID: "c-blue", Rarity: domain.RarityCommon, Colors: []string{"U"}},
		"r-white": {ID: "r-white", Rarity: domain.RarityRare, Colors: []string{"W"}},
		"s-gold":  {ID: "s-gold", Rarity: domain.RaritySpecial, Colors: []string{"W", "U"}},
		"plains":  {ID: "plains", Rarity: domain.RarityCommon, IsBasicLand: true, IsLand: true},
	}}
}

func TestMilestonesOfListsTheGoalsTheBinderHasCardsFor(t *testing.T) {
	t.Parallel()

	got := domain.MilestonesOf(milestoneBinder())

	want := []domain.Milestone{
		{Set: "TST", Kind: domain.MilestoneRarity, Group: "common"},
		{Set: "TST", Kind: domain.MilestoneRarity, Group: "rare"},
		{Set: "TST", Kind: domain.MilestoneColour, Group: "W"},
		{Set: "TST", Kind: domain.MilestoneColour, Group: "U"},
		{Set: "TST", Kind: domain.MilestoneColour, Group: "gold"},
		{Set: "TST", Kind: domain.MilestoneBinder},
	}
	if !slices.Equal(got, want) {
		t.Errorf("MilestonesOf = %v, want %v: no uncommon or mythic, no special rarity, no basic land colour group", got, want)
	}
}

func TestMilestoneCardsAreTheBinderCardsItNamesInIDOrder(t *testing.T) {
	t.Parallel()
	binder := milestoneBinder()
	tests := []struct {
		name      string
		milestone domain.Milestone
		want      []domain.CardID
	}{
		{"a rarity", domain.Milestone{Set: "TST", Kind: domain.MilestoneRarity, Group: "common"}, []domain.CardID{"c-blue", "c-white"}},
		{"a colour", domain.Milestone{Set: "TST", Kind: domain.MilestoneColour, Group: "W"}, []domain.CardID{"c-white", "r-white"}},
		{"gold is a colour of its own", domain.Milestone{Set: "TST", Kind: domain.MilestoneColour, Group: "gold"}, []domain.CardID{"s-gold"}},
		{"the whole binder", domain.Milestone{Set: "TST", Kind: domain.MilestoneBinder}, []domain.CardID{"c-blue", "c-white", "r-white", "s-gold"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []domain.CardID
			for _, card := range tt.milestone.Cards(binder) {
				got = append(got, card.ID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Cards = %v, want %v", got, tt.want)
			}
		})
	}
}
