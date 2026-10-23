package domain_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestGeneratePopulationGivesEachNPCItsArchetypeBehaviour(t *testing.T) {
	t.Parallel()
	spec := domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "speculator", WishSize: 3, MarginPct: 15, OfferChancePct: 7, ResponseDelay: 3 * time.Hour}

	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 2, Archetypes: map[string]domain.ArchetypeSpec{"spec": spec}},
		crowdSets(), rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}

	for _, npc := range npcs {
		if npc.Behaviour == nil {
			t.Fatalf("NPC %d has no behaviour", npc.ID)
		}
		if npc.WishSize != 3 || npc.OfferChancePct != 7 || npc.ResponseDelay != 3*time.Hour {
			t.Errorf("NPC %d: WishSize %d, OfferChancePct %d, ResponseDelay %v, want 3, 7 and 3h", npc.ID, npc.WishSize, npc.OfferChancePct, npc.ResponseDelay)
		}
		if got := npc.Behaviour.Margin(domain.Offer{}); got != 15 {
			t.Errorf("NPC %d margin = %d, want 15", npc.ID, got)
		}
	}
}

func TestGeneratePopulationLeavesAnArchetypeWithoutBehaviourNotTrading(t *testing.T) {
	t.Parallel()
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"plain": {Share: 1, KeepCopies: 1}}},
		crowdSets(), rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	if npcs[0].Behaviour != nil {
		t.Error("an archetype with no behaviour name got one")
	}
}

func TestGeneratePopulationRejectsInvalidTradeSettings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		spec domain.ArchetypeSpec
	}{
		{"unknown behaviour", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "gambler"}},
		{"negative wish size", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", WishSize: -1}},
		{"negative margin", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", MarginPct: -1}},
		{"offer chance above 100", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", OfferChancePct: 101}},
		{"negative offer chance", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", OfferChancePct: -1}},
		{"negative response delay", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", ResponseDelay: -time.Hour}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.GeneratePopulation(
				domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"a": tt.spec}},
				crowdSets(), rand.New(rand.NewPCG(1, 1)))
			if err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}
