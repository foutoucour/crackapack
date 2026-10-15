package domain_test

import (
	"math/rand/v2"
	"slices"
	"testing"
	"unsafe"

	"github.com/foutoucour/crackapack/internal/domain"
)

// scopeCaptureBehaviour is a TradeBehaviour double that records every Scope slice it sees and
// returns no wishes. Tests use it to inspect what scope each NPC was given without mutating it.
type scopeCaptureBehaviour struct{ scopes *[][]domain.Card }

func (b scopeCaptureBehaviour) PickWishes(ctx domain.WishContext, _ int, _ *rand.Rand) []domain.Wish {
	*b.scopes = append(*b.scopes, ctx.Scope)
	return nil
}

func (scopeCaptureBehaviour) Margin(domain.Offer) domain.Percent { return 0 }

func (scopeCaptureBehaviour) Propose(domain.ProposalContext) (want, give []domain.TradeLine, ok bool) {
	return nil, nil, false
}

// colourTestCatalogue builds a minimal SetCatalogue with five cards: one W, one B, one UB
// (gold), one colourless (Colors nil), and one basic land. No sheets or booster options are
// needed because tests only build the crowd and inspect its scope, without ticking.
func colourTestCatalogue() domain.SetCatalogue {
	return domain.SetCatalogue{
		Code: "CLR",
		Cards: map[domain.CardID]domain.Card{
			"clr-w-001":  {ID: "clr-w-001", Name: "White Card", Rarity: domain.RarityCommon, Colors: []string{"W"}},
			"clr-b-001":  {ID: "clr-b-001", Name: "Black Card", Rarity: domain.RarityCommon, Colors: []string{"B"}},
			"clr-ub-01":  {ID: "clr-ub-01", Name: "Gold UB Card", Rarity: domain.RarityUncommon, Colors: []string{"U", "B"}},
			"clr-c-001":  {ID: "clr-c-001", Name: "Colourless Card", Rarity: domain.RarityRare, Colors: nil},
			"clr-land-1": {ID: "clr-land-1", Name: "Basic Land", Rarity: domain.RarityCommon, IsBasicLand: true},
		},
	}
}

// colourTestCrowd creates a Crowd from NPCs generated with archetypeColors, each given behaviour.
// It panics on error because test setup failures should be fatal to the subtest.
func colourTestCrowd(t *testing.T, archetypeColors []string, behavior domain.TradeBehaviour, size int) (*domain.Crowd, []*domain.NPC) {
	t.Helper()
	cat := colourTestCatalogue()
	sets := []domain.SetCatalogue{cat}
	rng := rand.New(rand.NewPCG(42, 42))

	spec := domain.ArchetypeSpec{
		Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1,
		Colors:   archetypeColors,
		WishSize: 10, // large enough to try to pick everything
	}
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: size, Archetypes: map[string]domain.ArchetypeSpec{"a": spec}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	for _, npc := range npcs {
		npc.Behaviour = behavior
	}

	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{}, rng)
	crowd, err := domain.NewCrowd(npcs, sets, opener, counter, rng)
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	return crowd, npcs
}

// scopeCardIDs extracts the sorted card IDs from a scope slice for comparison.
func scopeCardIDs(scope []domain.Card) []domain.CardID {
	ids := make([]domain.CardID, len(scope))
	for i, c := range scope {
		ids[i] = c.ID
	}
	slices.Sort(ids)
	return ids
}

// sliceDataPtr returns the pointer to the first element of s, or nil if s is empty.
// It is used to test that two slices share the same backing array.
func sliceDataPtr(s []domain.Card) unsafe.Pointer {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Pointer(&s[0])
}

func TestCrowdScope_BNPCSeesOnlyBAndColourlessCards(t *testing.T) {
	t.Parallel()
	var scopes [][]domain.Card
	b := scopeCaptureBehaviour{scopes: &scopes}
	_, _ = colourTestCrowd(t, []string{"B"}, b, 1)

	if len(scopes) != 1 {
		t.Fatalf("behaviour called %d times, want 1", len(scopes))
	}
	got := scopeCardIDs(scopes[0])
	want := []domain.CardID{"clr-b-001", "clr-c-001"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("B NPC scope = %v, want %v", got, want)
	}
}

func TestCrowdScope_UBNPCSeesUBAndColourlessButNotW(t *testing.T) {
	t.Parallel()
	var scopes [][]domain.Card
	b := scopeCaptureBehaviour{scopes: &scopes}
	_, _ = colourTestCrowd(t, []string{"U", "B"}, b, 1)

	if len(scopes) != 1 {
		t.Fatalf("behaviour called %d times, want 1", len(scopes))
	}
	got := scopeCardIDs(scopes[0])
	want := []domain.CardID{"clr-b-001", "clr-c-001", "clr-ub-01"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("UB NPC scope = %v, want %v (must not contain the W card)", got, want)
	}
}

func TestCrowdScope_EmptyColorsGetsTodaysFullScope(t *testing.T) {
	t.Parallel()
	var scopesEmpty, scopesFull [][]domain.Card
	bEmpty := scopeCaptureBehaviour{scopes: &scopesEmpty}
	bFull := scopeCaptureBehaviour{scopes: &scopesFull}

	// Build one crowd with empty colors and one with all five colors.
	crowdEmpty, _ := colourTestCrowd(t, nil, bEmpty, 1)
	crowdFull, _ := colourTestCrowd(t, []string{"W", "U", "B", "R", "G"}, bFull, 1)
	_ = crowdEmpty
	_ = crowdFull

	if len(scopesEmpty) != 1 || len(scopesFull) != 1 {
		t.Fatalf("want 1 scope each, got %d and %d", len(scopesEmpty), len(scopesFull))
	}

	// Both should contain exactly the non-basic, non-island cards: W, B, UB, colourless.
	gotEmpty := scopeCardIDs(scopesEmpty[0])
	gotFull := scopeCardIDs(scopesFull[0])
	want := []domain.CardID{"clr-b-001", "clr-c-001", "clr-ub-01", "clr-w-001"}
	slices.Sort(want)
	if !slices.Equal(gotEmpty, want) {
		t.Errorf("empty-color NPC scope = %v, want %v", gotEmpty, want)
	}
	if !slices.Equal(gotFull, want) {
		t.Errorf("all-five NPC scope = %v, want %v", gotFull, want)
	}
}

func TestCrowdScope_EmptyAndAllFiveReturnTheSameSlice(t *testing.T) {
	t.Parallel()
	var scopesEmpty, scopesFull [][]domain.Card
	bEmpty := scopeCaptureBehaviour{scopes: &scopesEmpty}
	bFull := scopeCaptureBehaviour{scopes: &scopesFull}

	// Two NPCs in the same crowd: one with empty colors, one with all five.
	cat := colourTestCatalogue()
	sets := []domain.SetCatalogue{cat}
	rng := rand.New(rand.NewPCG(42, 42))

	specEmpty := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, WishSize: 10}
	specFull := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: []string{"W", "U", "B", "R", "G"}, WishSize: 10}
	npcsEmpty, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"a": specEmpty}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation empty: %v", err)
	}
	npcsFull, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"b": specFull}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation full: %v", err)
	}
	npcsEmpty[0].Behaviour = bEmpty
	npcsFull[0].Behaviour = bFull
	allNPCs := append(npcsEmpty, npcsFull...)

	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{}, rng)
	_, err = domain.NewCrowd(allNPCs, sets, opener, counter, rng)
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}

	if len(scopesEmpty) != 1 || len(scopesFull) != 1 {
		t.Fatalf("want 1 scope each, got %d and %d", len(scopesEmpty), len(scopesFull))
	}
	// Both should be the same backing array (scopeAll is returned directly for both cases).
	pEmpty := sliceDataPtr(scopesEmpty[0])
	pFull := sliceDataPtr(scopesFull[0])
	if pEmpty == nil || pFull == nil {
		t.Fatal("scope is empty; cannot compare backing arrays")
	}
	if pEmpty != pFull {
		t.Error("empty-color and all-five NPC got different backing arrays; want the same scopeAll slice")
	}
}

func TestCrowdScope_100NPCsOfSameArchetypeShareOneBackingArray(t *testing.T) {
	t.Parallel()
	var scopes [][]domain.Card
	b := scopeCaptureBehaviour{scopes: &scopes}
	_, _ = colourTestCrowd(t, []string{"B"}, b, 100)

	if len(scopes) != 100 {
		t.Fatalf("behaviour called %d times, want 100", len(scopes))
	}
	first := sliceDataPtr(scopes[0])
	if first == nil {
		t.Fatal("first scope is empty; cannot compare backing arrays")
	}
	for i, s := range scopes[1:] {
		if sliceDataPtr(s) != first {
			t.Errorf("NPC %d has a different scope backing array; want all 100 to share one", i+1)
			return
		}
	}
}

func TestCrowdScope_UBAndBUShareTheSameBackingArray(t *testing.T) {
	t.Parallel()
	var scopesUB, scopesBU [][]domain.Card
	bUB := scopeCaptureBehaviour{scopes: &scopesUB}
	bBU := scopeCaptureBehaviour{scopes: &scopesBU}

	// Two NPCs in one crowd: one with [U,B], one with [B,U]. Same bitmask → same filtered slice.
	cat := colourTestCatalogue()
	sets := []domain.SetCatalogue{cat}
	rng := rand.New(rand.NewPCG(42, 42))

	specUB := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: []string{"U", "B"}, WishSize: 10}
	specBU := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: []string{"B", "U"}, WishSize: 10}
	npcsUB, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"ub": specUB}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation UB: %v", err)
	}
	npcsBU, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"bu": specBU}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation BU: %v", err)
	}
	npcsUB[0].Behaviour = bUB
	npcsBU[0].Behaviour = bBU
	allNPCs := append(npcsUB, npcsBU...)

	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{}, rng)
	_, err = domain.NewCrowd(allNPCs, sets, opener, counter, rng)
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}

	if len(scopesUB) != 1 || len(scopesBU) != 1 {
		t.Fatalf("want 1 scope each, got %d and %d", len(scopesUB), len(scopesBU))
	}
	pUB := sliceDataPtr(scopesUB[0])
	pBU := sliceDataPtr(scopesBU[0])
	if pUB == nil || pBU == nil {
		t.Fatal("scope is empty; cannot compare backing arrays")
	}
	if pUB != pBU {
		t.Error("[U,B] and [B,U] NPCs got different scope backing arrays; want the same memoised filtered slice")
	}
}

func TestCrowdScope_DifferentColourSetsGetDifferentScopes(t *testing.T) {
	t.Parallel()
	var scopesW, scopesB [][]domain.Card
	bW := scopeCaptureBehaviour{scopes: &scopesW}
	bB := scopeCaptureBehaviour{scopes: &scopesB}

	cat := colourTestCatalogue()
	sets := []domain.SetCatalogue{cat}
	rng := rand.New(rand.NewPCG(42, 42))

	specW := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: []string{"W"}, WishSize: 10}
	specB := domain.ArchetypeSpec{Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1, Colors: []string{"B"}, WishSize: 10}
	npcsW, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"w": specW}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation W: %v", err)
	}
	npcsB, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"b": specB}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation B: %v", err)
	}
	npcsW[0].Behaviour = bW
	npcsB[0].Behaviour = bB
	allNPCs := append(npcsW, npcsB...)

	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{}, rng)
	_, err = domain.NewCrowd(allNPCs, sets, opener, counter, rng)
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}

	if len(scopesW) != 1 || len(scopesB) != 1 {
		t.Fatalf("want 1 scope each, got %d and %d", len(scopesW), len(scopesB))
	}
	pW := sliceDataPtr(scopesW[0])
	pB := sliceDataPtr(scopesB[0])
	if pW == nil || pB == nil {
		t.Fatal("a scope is empty; cannot compare backing arrays")
	}
	if pW == pB {
		t.Error("W and B NPCs share a scope backing array; want distinct filtered slices")
	}
}

func TestCrowdScope_FocusSetNarrowsBeforeColourFilter(t *testing.T) {
	t.Parallel()
	// Two sets: CLR (W, B, UB, colourless, land) and another set with a B card.
	// A B NPC focused on CLR must only see CLR's B and colourless cards, not the B card in the other set.
	catA := colourTestCatalogue() // code CLR
	catB := domain.SetCatalogue{
		Code: "OTH",
		Cards: map[domain.CardID]domain.Card{
			"oth-b-001": {ID: "oth-b-001", Name: "Other Black", Rarity: domain.RarityCommon, Colors: []string{"B"}},
		},
	}
	sets := []domain.SetCatalogue{catA, catB}
	rng := rand.New(rand.NewPCG(42, 42))

	spec := domain.ArchetypeSpec{
		Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1,
		Colors: []string{"B"}, WishSize: 10, FocusSet: true,
	}
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 10, Archetypes: map[string]domain.ArchetypeSpec{"a": spec}},
		sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}

	var scopes [][]domain.Card
	b := scopeCaptureBehaviour{scopes: &scopes}
	for _, npc := range npcs {
		npc.Behaviour = b
	}

	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{}, rng)
	_, err = domain.NewCrowd(npcs, sets, opener, counter, rng)
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}

	// Every NPC focused on CLR must see only CLR's B card and colourless card.
	// NPCs focused on OTH must see only OTH's B card (no colourless in OTH).
	for i, scope := range scopes {
		npc := npcs[i]
		for _, card := range scope {
			// The card must belong to the NPC's focus set.
			switch npc.FocusSet {
			case "CLR":
				if card.ID != "clr-b-001" && card.ID != "clr-c-001" {
					t.Errorf("CLR-focused B NPC %d scope contains %q, want only clr-b-001 and clr-c-001",
						npc.ID, card.ID)
				}
			case "OTH":
				if card.ID != "oth-b-001" {
					t.Errorf("OTH-focused B NPC %d scope contains %q, want only oth-b-001", npc.ID, card.ID)
				}
			}
		}
	}
}

func BenchmarkNewCrowdColourScope(b *testing.B) {
	cat := colourTestCatalogue()
	sets := []domain.SetCatalogue{cat}

	spec := domain.ArchetypeSpec{
		Share: 1, MinBoostersPerDay: 1, MaxBoostersPerDay: 1, KeepCopies: 1,
		Colors: []string{"B"}, WishSize: 5,
	}

	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(),
		domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 1})
	if err != nil {
		b.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{}, rand.New(rand.NewPCG(1, 1)))

	b.ResetTimer()
	for b.Loop() {
		rng := rand.New(rand.NewPCG(1, 1))
		npcs, _ := domain.GeneratePopulation(
			domain.PopulationParams{Size: 100, Archetypes: map[string]domain.ArchetypeSpec{"a": spec}},
			sets, rng)
		_, _ = domain.NewCrowd(npcs, sets, opener, counter, rng)
	}
}
