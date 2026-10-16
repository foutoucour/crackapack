package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// datedSet returns a minimal SetCatalogue with the given code and ISO release date.
func datedSet(code, date string) domain.SetCatalogue {
	return domain.SetCatalogue{Code: code, ReleaseDate: date}
}

// undatedSet returns a minimal SetCatalogue with the given code and no release date.
func undatedSet(code string) domain.SetCatalogue {
	return domain.SetCatalogue{Code: code}
}

// focusSpec builds a minimal ArchetypeSpec with the given FocusSetMode and share=1.
func focusSpec(mode domain.FocusSetMode) domain.ArchetypeSpec {
	return domain.ArchetypeSpec{
		Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1,
		FocusSet: mode,
	}
}

// TestOldestSet_picksEarliest verifies that the set with the earliest release date wins.
func TestOldestSet_picksEarliest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sets     []domain.SetCatalogue
		wantCode string
	}{
		{
			name:     "two sets, CLS is older",
			sets:     []domain.SetCatalogue{datedSet("RAV", "2005-10-07"), datedSet("CLS", "1999-04-21")},
			wantCode: "CLS",
		},
		{
			name:     "two sets, RAV is older",
			sets:     []domain.SetCatalogue{datedSet("RAV", "2005-10-07"), datedSet("TSP", "2006-10-06")},
			wantCode: "RAV",
		},
		{
			name:     "three sets, PLC is newest",
			sets:     []domain.SetCatalogue{datedSet("PLC", "2007-02-02"), datedSet("RAV", "2005-10-07"), datedSet("CLS", "1999-04-21")},
			wantCode: "CLS",
		},
		{
			name:     "single set",
			sets:     []domain.SetCatalogue{datedSet("RAV", "2005-10-07")},
			wantCode: "RAV",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			params := domain.PopulationParams{
				Size:       5,
				Archetypes: map[string]domain.ArchetypeSpec{"arc": focusSpec(domain.FocusSetOldest)},
			}
			npcs, err := domain.GeneratePopulation(params, tt.sets, testRNG(1))
			if err != nil {
				t.Fatalf("GeneratePopulation: %v", err)
			}
			for _, npc := range npcs {
				if npc.FocusSet != tt.wantCode {
					t.Errorf("NPC %d FocusSet = %q, want %q", npc.ID, npc.FocusSet, tt.wantCode)
				}
			}
		})
	}
}

// TestOldestSet_tieGoesToFirst verifies that on equal dates the first set in the slice wins.
func TestOldestSet_tieGoesToFirst(t *testing.T) {
	t.Parallel()

	sets := []domain.SetCatalogue{
		datedSet("AAA", "2005-10-07"),
		datedSet("BBB", "2005-10-07"),
	}
	params := domain.PopulationParams{
		Size: 3, Archetypes: map[string]domain.ArchetypeSpec{"arc": focusSpec(domain.FocusSetOldest)},
	}
	npcs, err := domain.GeneratePopulation(params, sets, testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	for _, npc := range npcs {
		if npc.FocusSet != "AAA" {
			t.Errorf("NPC %d FocusSet = %q, want AAA (first in order on tie)", npc.ID, npc.FocusSet)
		}
	}
}

// TestOldestSet_undatedSetsError verifies that FocusSetOldest fails when any set
// lacks a release date, naming the undated codes.
func TestOldestSet_undatedSetsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sets      []domain.SetCatalogue
		wantInErr string
	}{
		{
			name:      "single undated set",
			sets:      []domain.SetCatalogue{undatedSet("SYN")},
			wantInErr: "SYN",
		},
		{
			name:      "one dated one undated, undated is named",
			sets:      []domain.SetCatalogue{datedSet("RAV", "2005-10-07"), undatedSet("TSB")},
			wantInErr: "TSB",
		},
		{
			name:      "two undated sets, both named",
			sets:      []domain.SetCatalogue{undatedSet("AAA"), datedSet("RAV", "2005-10-07"), undatedSet("ZZZ")},
			wantInErr: "AAA",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			params := domain.PopulationParams{
				Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"arc": focusSpec(domain.FocusSetOldest)},
			}
			_, err := domain.GeneratePopulation(params, tt.sets, testRNG(1))
			if err == nil {
				t.Fatal("want error for undated sets with FocusSetOldest, got nil")
			}
			if err != nil && len(tt.wantInErr) > 0 {
				errStr := err.Error()
				if !containsString(errStr, tt.wantInErr) {
					t.Errorf("error %q does not mention %q", errStr, tt.wantInErr)
				}
			}
		})
	}
}

// containsString reports whether s contains sub. Avoids importing strings.
func containsString(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestOldestSet_shareZeroSafeWithUndatedSets verifies that a share-0 archetype with
// FocusSetOldest does not fail even when sets carry no release dates.
func TestOldestSet_shareZeroSafeWithUndatedSets(t *testing.T) {
	t.Parallel()

	sets := []domain.SetCatalogue{undatedSet("SYN"), undatedSet("TSB")}
	params := domain.PopulationParams{
		Size: 5,
		Archetypes: map[string]domain.ArchetypeSpec{
			"regular":   {Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1},
			"archivist": {Share: 0, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, FocusSet: domain.FocusSetOldest},
		},
	}
	npcs, err := domain.GeneratePopulation(params, sets, testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v, want no error for share-0 archetype with undated sets", err)
	}
	for _, npc := range npcs {
		if npc.Archetype == "archivist" {
			t.Errorf("NPC %d: archetype archivist (share=0) generated an NPC", npc.ID)
		}
	}
}

// TestOldestSet_sizeZeroSafe verifies that population size zero does not attempt
// oldest-set resolution and succeeds even with undated sets.
func TestOldestSet_sizeZeroSafe(t *testing.T) {
	t.Parallel()

	sets := []domain.SetCatalogue{undatedSet("SYN")}
	params := domain.PopulationParams{
		Size: 0,
		Archetypes: map[string]domain.ArchetypeSpec{
			"arc": {Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, FocusSet: domain.FocusSetOldest},
		},
	}
	npcs, err := domain.GeneratePopulation(params, sets, testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	if len(npcs) != 0 {
		t.Errorf("got %d NPCs for size=0, want 0", len(npcs))
	}
}

// TestOldestSet_randomStillConsumesDraws verifies that FocusSetRandom NPCs still
// consume a random draw when FocusSetOldest NPCs are present (draw budget unchanged).
func TestOldestSet_randomStillConsumesDraws(t *testing.T) {
	t.Parallel()

	sets := []domain.SetCatalogue{datedSet("AAA", "2005-10-07"), datedSet("BBB", "2006-10-06")}
	params := domain.PopulationParams{
		Size: 20,
		Archetypes: map[string]domain.ArchetypeSpec{
			"random": focusSpec(domain.FocusSetRandom),
			"oldest": focusSpec(domain.FocusSetOldest),
		},
	}
	npcs, err := domain.GeneratePopulation(params, sets, testRNG(42))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}

	seenRandom := map[string]bool{}
	for _, npc := range npcs {
		if npc.Archetype == "oldest" && npc.FocusSet != "AAA" {
			t.Errorf("oldest NPC %d FocusSet = %q, want AAA", npc.ID, npc.FocusSet)
		}
		if npc.Archetype == "random" {
			seenRandom[npc.FocusSet] = true
		}
	}
	// With 10 random NPCs over 2 sets both sets should appear.
	if len(seenRandom) < 2 {
		t.Errorf("random NPCs drew only sets %v, want both AAA and BBB covered", seenRandom)
	}
}

// TestFocusSetNone_noFocusSet verifies that FocusSetNone NPCs have an empty FocusSet string.
func TestFocusSetNone_noFocusSet(t *testing.T) {
	t.Parallel()

	sets := []domain.SetCatalogue{datedSet("RAV", "2005-10-07"), datedSet("CLS", "1999-04-21")}
	params := domain.PopulationParams{
		Size: 5, Archetypes: map[string]domain.ArchetypeSpec{"arc": focusSpec(domain.FocusSetNone)},
	}
	npcs, err := domain.GeneratePopulation(params, sets, testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	for _, npc := range npcs {
		if npc.FocusSet != "" {
			t.Errorf("NPC %d FocusSet = %q, want empty for FocusSetNone", npc.ID, npc.FocusSet)
		}
	}
}

// TestGeneratePopulation_unknownFocusModeRejected verifies that an out-of-range FocusSetMode
// is rejected before any NPC is built.
func TestGeneratePopulation_unknownFocusModeRejected(t *testing.T) {
	t.Parallel()

	params := domain.PopulationParams{
		Size: 1,
		Archetypes: map[string]domain.ArchetypeSpec{
			"bad": {Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, FocusSet: domain.FocusSetMode(99)},
		},
	}
	_, err := domain.GeneratePopulation(params, testSets(), testRNG(1))
	if err == nil {
		t.Fatal("want error for unknown FocusSetMode(99), got nil")
	}
}

// TestOldestSet_usesRealTestCatalogues verifies that the domaintest helpers carry
// dates and that the oldest is CLS (1999-04-21 < 2005-10-07 for RAV).
func TestOldestSet_usesRealTestCatalogues(t *testing.T) {
	t.Parallel()

	sets := []domain.SetCatalogue{domaintest.NewRavnicaCatalogue(), domaintest.NewClassicCatalogue()}
	params := domain.PopulationParams{
		Size: 5, Archetypes: map[string]domain.ArchetypeSpec{"arc": focusSpec(domain.FocusSetOldest)},
	}
	npcs, err := domain.GeneratePopulation(params, sets, testRNG(1))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	for _, npc := range npcs {
		if npc.FocusSet != "CLS" {
			t.Errorf("NPC %d FocusSet = %q, want CLS (oldest real test catalogue)", npc.ID, npc.FocusSet)
		}
	}
}
