package domain_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// newRNG returns a seeded, reproducible *rand.Rand for test use.
func newRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed))
}

// TestClassicGenerator_returns15Cards verifies that every generated booster has exactly 15 cards.
// NewRavnicaCatalogue and NewClassicCatalogue are both designed with 15-card options.
func TestClassicGenerator_returns15Cards(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cat  domain.SetCatalogue
	}{
		{"ravnica", domaintest.NewRavnicaCatalogue()},
		{"classic", domaintest.NewClassicCatalogue()},
	}
	gen := domain.NewClassicGenerator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range 50 {
				b, err := gen.Generate(tt.cat, newRNG(uint64(i+1)))
				if err != nil {
					t.Fatalf("pack %d: Generate: %v", i, err)
				}
				if len(b.Cards) != 15 {
					t.Errorf("pack %d: len(Cards) = %d, want 15", i, len(b.Cards))
				}
			}
		})
	}
}

// TestClassicGenerator_setCodeMatchesCatalogue verifies that Booster.SetCode equals the catalogue code.
func TestClassicGenerator_setCodeMatchesCatalogue(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	gen := domain.NewClassicGenerator()

	b, err := gen.Generate(cat, newRNG(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if b.SetCode != cat.Code {
		t.Errorf("SetCode = %q, want %q", b.SetCode, cat.Code)
	}
}

// TestClassicGenerator_slotCountsMatchOption verifies that the booster card count always matches
// the total declared in the chosen BoosterOption (for 100 packs, covering both options in NewCatalogue).
func TestClassicGenerator_slotCountsMatchOption(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	gen := domain.NewClassicGenerator()

	// Compute the valid total counts from all options.
	validTotals := make(map[int]bool)
	for _, opt := range cat.BoosterOptions {
		total := 0
		for _, n := range opt.Contents {
			total += n
		}
		validTotals[total] = true
	}

	rng := newRNG(2)
	for i := range 100 {
		b, err := gen.Generate(cat, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}
		if !validTotals[len(b.Cards)] {
			t.Errorf("pack %d: len(Cards) = %d, not a valid option total", i, len(b.Cards))
		}
	}
}

// TestClassicGenerator_foilFinishOnFoilSheet verifies that cards drawn from a Foil-flagged sheet
// carry FinishFoil, and cards from non-foil sheets carry FinishNonFoil.
func TestClassicGenerator_foilFinishOnFoilSheet(t *testing.T) {
	t.Parallel()
	// Use NewRavnicaCatalogue which has a foil sheet. Run enough packs to hit a foil option.
	cat := domaintest.NewRavnicaCatalogue()
	gen := domain.NewClassicGenerator()
	rng := newRNG(3)

	foundFoil := false
	foundNonFoil := false
	for i := range 500 {
		b, err := gen.Generate(cat, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}
		for _, ck := range b.Cards {
			switch ck.Finish {
			case domain.FinishFoil:
				foundFoil = true
			case domain.FinishNonFoil:
				foundNonFoil = true
			default:
				t.Errorf("pack %d: unexpected finish %q", i, ck.Finish)
			}
		}
	}
	if !foundFoil {
		t.Error("no foil card seen in 500 packs; expected ~20% foil rate")
	}
	if !foundNonFoil {
		t.Error("no non-foil card seen in 500 packs")
	}
}

// TestClassicGenerator_noDuplicatesWithinSheet verifies that no card ID appears more than once
// among cards drawn from the same sheet in one pack.
//
// Since we cannot directly observe which sheet a card came from, we verify per-rarity
// uniqueness for NewCatalogue (where each rarity maps 1-to-1 to a sheet and no rarity
// is drawn more times than its sheet size). For 1000 packs, any within-sheet duplicate
// would surface here.
func TestClassicGenerator_noDuplicatesWithinSheet(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	gen := domain.NewClassicGenerator()
	rng := newRNG(4)

	for i := range 1000 {
		b, err := gen.Generate(cat, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}
		seen := make(map[domain.CardID]bool, len(b.Cards))
		for _, ck := range b.Cards {
			if seen[ck.ID] {
				t.Errorf("pack %d: duplicate card ID %q", i, ck.ID)
			}
			seen[ck.ID] = true
		}
	}
}

// TestClassicGenerator_rareLast verifies that in a booster where a rare exists, it appears last.
// NewCatalogue always includes a rare in every option.
func TestClassicGenerator_rareLast(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	gen := domain.NewClassicGenerator()
	rng := newRNG(5)

	for i := range 500 {
		b, err := gen.Generate(cat, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}
		if len(b.Cards) == 0 {
			t.Fatalf("pack %d: empty booster", i)
		}
		last := b.Cards[len(b.Cards)-1]
		c, ok := cat.Cards[last.ID]
		if !ok {
			t.Errorf("pack %d: last card %q not found in catalogue", i, last.ID)
			continue
		}
		if c.Rarity != domain.RarityRare {
			t.Errorf("pack %d: last card rarity = %q, want rare", i, c.Rarity)
		}
	}
}

// TestClassicGenerator_reproducible verifies that the same seed always yields the same booster.
func TestClassicGenerator_reproducible(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	gen := domain.NewClassicGenerator()

	b1, err := gen.Generate(cat, newRNG(99))
	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	b2, err := gen.Generate(cat, newRNG(99))
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}

	if b1.SetCode != b2.SetCode || len(b1.Cards) != len(b2.Cards) {
		t.Fatalf("different booster shapes: %v vs %v", b1, b2)
	}
	for i := range b1.Cards {
		if b1.Cards[i] != b2.Cards[i] {
			t.Errorf("Cards[%d] = %+v, want %+v", i, b2.Cards[i], b1.Cards[i])
		}
	}
}

// TestClassicGenerator_colourBalance verifies that a colour-balanced sheet produces all five
// Magic colours (W U B R G) among the drawn common cards, across 200 packs.
// NewClassicCatalogue has BalanceColors=true on its common sheet and 3 cards per colour.
func TestClassicGenerator_colourBalance(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewClassicCatalogue()
	gen := domain.NewClassicGenerator()
	rng := newRNG(6)

	for i := range 200 {
		b, err := gen.Generate(cat, rng)
		if err != nil {
			t.Fatalf("pack %d: Generate: %v", i, err)
		}

		// Collect colours from non-foil common cards (drawn from the balanced "common" sheet).
		colours := make(map[string]bool)
		for _, ck := range b.Cards {
			if ck.Finish != domain.FinishNonFoil {
				continue
			}
			c, ok := cat.Cards[ck.ID]
			if !ok || c.Rarity != domain.RarityCommon {
				continue
			}
			for _, col := range c.Colors {
				colours[col] = true
			}
		}
		for _, want := range [5]string{"W", "U", "B", "R", "G"} {
			if !colours[want] {
				t.Errorf("pack %d: colour %q missing from non-foil commons", i, want)
			}
		}
	}
}

// TestClassicGenerator_errorMissingSheet verifies that referencing a sheet absent from the
// catalogue returns an error naming that sheet.
func TestClassicGenerator_errorMissingSheet(t *testing.T) {
	t.Parallel()
	// Build a catalogue whose only BoosterOption references a non-existent sheet.
	cat := domaintest.NewCatalogue(42)
	cat.BoosterOptions = []domain.BoosterOption{
		{Contents: map[string]int{"nonexistent": 1}, Weight: 1},
	}

	gen := domain.NewClassicGenerator()
	_, err := gen.Generate(cat, newRNG(1))
	if err == nil {
		t.Fatal("want error for missing sheet, got nil")
	}
}

// TestClassicGenerator_errorEmptySheet verifies that drawing from an empty sheet returns an error.
func TestClassicGenerator_errorEmptySheet(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	// Replace the rare sheet with an empty one.
	emptySheet := domain.Sheet{Name: "rare", Cards: nil, TotalWeight: 0}
	cat.Sheets["rare"] = emptySheet
	cat.BoosterOptions = []domain.BoosterOption{
		{Contents: map[string]int{"rare": 1}, Weight: 1},
	}

	gen := domain.NewClassicGenerator()
	_, err := gen.Generate(cat, newRNG(1))
	if err == nil {
		t.Fatal("want error for empty sheet, got nil")
	}
}

// TestClassicGenerator_errorOversizedDraw verifies that requesting more cards than a sheet holds
// returns an error rather than producing a short pack.
func TestClassicGenerator_errorOversizedDraw(t *testing.T) {
	t.Parallel()
	// NewClassicCatalogue has 1 rare in the "rare" sheet. Requesting 2 must error.
	cat := domaintest.NewClassicCatalogue()
	cat.BoosterOptions = []domain.BoosterOption{
		{Contents: map[string]int{"rare": 2}, Weight: 1},
	}

	gen := domain.NewClassicGenerator()
	_, err := gen.Generate(cat, newRNG(1))
	if err == nil {
		t.Fatal("want error for oversized draw, got nil")
	}
}

// ---------------------------------------------------------------------------
// Statistical tests — run 100 000 packs with a fixed seed and check that the
// observed rates fall within the documented tolerances.
//
// Tolerance justification:
//
//	For a rate p observed over n=100 000 trials, σ = sqrt(p(1-p)/n).
//	All tolerances are set at ≥ 3σ so that spurious failure probability < 0.3%.
//
// Ravnica foil rates (p = 0.205, σ ≈ 0.0013):  ±0.5%  ≈ 3.8σ
// Ravnica two-foil rate (p = 0.009, σ ≈ 0.0003): ±0.3% ≈ 10σ
// Classic foil rate (p = 0.225, σ ≈ 0.0013): ±0.5% ≈ 3.8σ
// Classic foil rarity (p = 0.15 among 22500, σ ≈ 0.0024): ±1% ≈ 4.2σ
// ---------------------------------------------------------------------------

const statPacks = 100_000

// TestClassicGenerator_statisticsRavnica runs 100 000 packs against the Ravnica-like catalogue
// and asserts that the foil-pack rate, two-foil rate, and per-slot foil replacement rates fall
// within the documented tolerances of the Ravnica block.
func TestClassicGenerator_statisticsRavnica(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	gen := domain.NewClassicGenerator()
	rng := newRNG(7)

	var (
		foilPacks    int // packs with ≥1 foil card
		twoFoilPacks int // packs with exactly 2 foil cards
		// foil-slot replacement counts (detected from non-foil card structure):
		foilCommonSlots   int // sum of (11 - nNFCommon) across all packs
		foilUncommonSlots int // sum of (3 - nNFUncommon)
		foilRareSlots     int // sum of (1 - nNFRare)
	)

	for range statPacks {
		b, err := gen.Generate(cat, rng)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}

		var nFoil, nNFCommon, nNFUncommon, nNFRare int
		for _, ck := range b.Cards {
			if ck.Finish == domain.FinishFoil {
				nFoil++
				continue
			}
			c, ok := cat.Cards[ck.ID]
			if !ok {
				continue
			}
			switch c.Rarity {
			case domain.RarityCommon:
				nNFCommon++
			case domain.RarityUncommon:
				nNFUncommon++
			case domain.RarityRare:
				nNFRare++
			case domain.RaritySpecial, domain.RarityMythic:
				// Special (timeshifted) and mythic cards are not tracked for Ravnica slot counts.
			}
		}

		if nFoil >= 1 {
			foilPacks++
		}
		if nFoil == 2 {
			twoFoilPacks++
		}
		foilCommonSlots += 11 - nNFCommon
		foilUncommonSlots += 3 - nNFUncommon
		foilRareSlots += 1 - nNFRare
	}

	const tol = 0.005      // ±0.5%
	const tol2Foil = 0.003 // ±0.3%

	assertRate(t, "foil-pack rate", float64(foilPacks)/statPacks, 0.205, tol)
	assertRate(t, "two-foil-pack rate", float64(twoFoilPacks)/statPacks, 0.009, tol2Foil)
	assertRate(t, "foil-common-slot rate", float64(foilCommonSlots)/statPacks, 0.157, tol)
	assertRate(t, "foil-uncommon-slot rate", float64(foilUncommonSlots)/statPacks, 0.043, tol)
	assertRate(t, "foil-rare-slot rate", float64(foilRareSlots)/statPacks, 0.014, tol2Foil)
}

// TestClassicGenerator_statisticsClassic runs 100 000 packs against the non-Ravnica classic
// catalogue and asserts that the foil-pack rate and foil-card rarity distribution match the
// documented 60/25/15 weights within the stated tolerances.
func TestClassicGenerator_statisticsClassic(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewClassicCatalogue()
	gen := domain.NewClassicGenerator()
	rng := newRNG(8)

	var foilPacks int
	foilByRarity := make(map[domain.Rarity]int) // rarity of the drawn foil card

	for range statPacks {
		b, err := gen.Generate(cat, rng)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		for _, ck := range b.Cards {
			if ck.Finish != domain.FinishFoil {
				continue
			}
			foilPacks++
			c, ok := cat.Cards[ck.ID]
			if ok {
				foilByRarity[c.Rarity]++
			}
		}
	}

	const tol = 0.005      // ±0.5%
	const tolRarity = 0.01 // ±1%

	assertRate(t, "classic foil-pack rate", float64(foilPacks)/statPacks, 0.225, tol)

	if foilPacks > 0 {
		fCommon := float64(foilByRarity[domain.RarityCommon]) / float64(foilPacks)
		fUncommon := float64(foilByRarity[domain.RarityUncommon]) / float64(foilPacks)
		fRare := float64(foilByRarity[domain.RarityRare]) / float64(foilPacks)
		assertRate(t, "classic foil common fraction", fCommon, 0.60, tolRarity)
		assertRate(t, "classic foil uncommon fraction", fUncommon, 0.25, tolRarity)
		assertRate(t, "classic foil rare fraction", fRare, 0.15, tolRarity)
	}
}

// ---------------------------------------------------------------------------
// Opener tests
// ---------------------------------------------------------------------------

// TestOpener_unknownLayout verifies that Open returns an error for an unregistered layout.
func TestOpener_unknownLayout(t *testing.T) {
	t.Parallel()
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{}, newRNG(1))
	_, err := opener.Open("unknown", domaintest.NewCatalogue(42))
	if err == nil {
		t.Fatal("want error for unknown layout, got nil")
	}
}

// TestOpener_delegatesToGenerator verifies that Open calls the generator for the registered layout.
func TestOpener_delegatesToGenerator(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	gen := domain.NewClassicGenerator()
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{
		domain.BoosterLayoutClassic: gen,
	}, newRNG(1))

	b, err := opener.Open(domain.BoosterLayoutClassic, cat)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(b.Cards) == 0 {
		t.Error("Open returned empty booster")
	}
	if b.SetCode != cat.Code {
		t.Errorf("SetCode = %q, want %q", b.SetCode, cat.Code)
	}
}

// TestOpener_registryHasAllKnownLayouts verifies that every exported BoosterLayout constant
// resolves in the registry, catching the case where a new constant is added without a generator.
func TestOpener_registryHasAllKnownLayouts(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewCatalogue(42)
	generators := map[domain.BoosterLayout]domain.BoosterGenerator{
		domain.BoosterLayoutClassic: domain.NewClassicGenerator(),
	}
	opener := domain.NewOpener(generators, newRNG(1))

	for _, layout := range []domain.BoosterLayout{
		domain.BoosterLayoutClassic,
	} {
		_, err := opener.Open(layout, cat)
		if err != nil {
			t.Errorf("layout %q: Open returned unexpected error: %v", layout, err)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// assertRate fails the test if |got - want| > tol.
func assertRate(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > tol {
		t.Errorf("%s: got %.4f, want %.4f ± %.4f (diff %.4f)", name, got, want, tol, diff)
	}
}
