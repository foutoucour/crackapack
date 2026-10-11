package domain

import (
	"fmt"
	"math/rand/v2"
	"sort"
)

// BoosterLayout identifies the booster generation algorithm for a set era.
// Adding a new layout requires only registering a new BoosterGenerator in the
// map passed to NewOpener; callers of Open are unchanged.
type BoosterLayout string

const (
	// BoosterLayoutClassic is the standard layout for sets from Ravnica (2005) through
	// Future Sight (2007): variable common/uncommon/rare slots with optional foil slots,
	// all defined by the set's BoosterOptions.
	BoosterLayoutClassic BoosterLayout = "classic"
)

// classicGenerator builds one booster pack for the classic era (2005-2007).
// All randomness comes from the injected rng; the result is reproducible from the seed.
//
// Pack-building algorithm:
//
//  1. Pick one BoosterOption by cumulative weight over the slice (defined order, no map
//     iteration). The BoosterOption defines the sheet structure for this pack.
//
//  2. Sort the sheet names from the option's Contents ascending for reproducible iteration.
//
//  3. For each sheet, draw n cards without replacement (within this sheet for this pack;
//     duplicates across different sheets in the same pack are allowed). If the sheet is
//     flagged Foil, every drawn card gets FinishFoil; otherwise FinishNonFoil.
//
//  4. Colour balance: for sheets with BalanceColors=true and n ≥ 5, only mono-coloured
//     cards (exactly one entry in Card.Colors) count toward the per-colour guarantee.
//     Multi-colour and colourless cards are never forced; they fill remaining slots.
//     The five colours (B G R U W, alphabetical order) are each satisfied by one pick,
//     skipping any colour with no mono-coloured representatives on the sheet. This is an
//     approximation of real WotC collation, not exact card-by-card collation.
//
//  5. Reveal order: ascending rarity rank, rare last. Within a rarity: non-foil before
//     foil, then ascending card ID. Rarity ranks: common=0, uncommon=1, special=2,
//     unknown (ID absent from cat.Cards)=3, rare=4. Unknown IDs arise when booster sheets
//     reference cards from a supplementary set file (e.g. TSB timeshifted cards in TSP).
type classicGenerator struct{}

// NewClassicGenerator returns a BoosterGenerator implementing the classic (2005-2007) layout.
func NewClassicGenerator() BoosterGenerator {
	return classicGenerator{}
}

// Generate implements BoosterGenerator.
func (classicGenerator) Generate(cat SetCatalogue, rng *rand.Rand) (Booster, error) {
	opt, err := pickOption(cat.BoosterOptions, rng)
	if err != nil {
		return Booster{}, fmt.Errorf("generate booster for %s: %w", cat.Code, err)
	}

	// Sort sheet names so that iteration order is deterministic (Contents is a map).
	sheetNames := make([]string, 0, len(opt.Contents))
	for name := range opt.Contents {
		sheetNames = append(sheetNames, name)
	}
	sort.Strings(sheetNames)

	var all []CardKey
	for _, name := range sheetNames {
		n := opt.Contents[name]
		sheet, ok := cat.Sheets[name]
		if !ok {
			return Booster{}, fmt.Errorf("generate booster for %s: sheet %q not found in catalogue", cat.Code, name)
		}
		drawn, err := drawFromSheet(sheet, n, cat.Cards, rng)
		if err != nil {
			return Booster{}, fmt.Errorf("generate booster for %s: %w", cat.Code, err)
		}
		all = append(all, drawn...)
	}

	// Sort into reveal order: rare last.
	sort.SliceStable(all, func(i, j int) bool {
		return revealLess(all[i], all[j], cat.Cards)
	})

	return Booster{SetCode: cat.Code, Cards: all}, nil
}

// pickOption selects one BoosterOption by weighted random draw over the slice.
// The slice order is preserved; no map iteration occurs.
func pickOption(opts []BoosterOption, rng *rand.Rand) (BoosterOption, error) {
	if len(opts) == 0 {
		return BoosterOption{}, fmt.Errorf("no booster options defined")
	}
	total := 0
	for _, o := range opts {
		total += o.Weight
	}
	if total <= 0 {
		return BoosterOption{}, fmt.Errorf("booster options: total weight is zero or negative")
	}
	pick := rng.IntN(total)
	cum := 0
	for _, o := range opts {
		cum += o.Weight
		if pick < cum {
			return o, nil
		}
	}
	// Unreachable: pick < total and cum reaches total.
	return opts[len(opts)-1], nil
}

// drawFromSheet draws n cards without replacement from sheet using weighted random selection.
//
// When sheet.BalanceColors is true and n ≥ 5, a colour-balance pass picks one mono-coloured
// card per colour (B G R U W) before filling the remaining slots. See classicGenerator for
// the full rule.
//
// If sheet.Foil is true, every drawn card receives FinishFoil; otherwise FinishNonFoil.
//
// Errors are returned when:
//   - sheet is empty
//   - n exceeds the number of distinct cards in sheet (without-replacement constraint)
func drawFromSheet(sheet Sheet, n int, cards map[CardID]Card, rng *rand.Rand) ([]CardKey, error) {
	if n <= 0 {
		// n=0 draws nothing; n<0 would panic on make([]CardKey, 0, n).
		// Both are caller bugs (caught at load time) but guarded here defensively.
		return nil, fmt.Errorf("sheet %q: n must be strictly positive, got %d", sheet.Name, n)
	}
	if len(sheet.Cards) == 0 {
		return nil, fmt.Errorf("sheet %q: empty", sheet.Name)
	}
	if n > len(sheet.Cards) {
		return nil, fmt.Errorf("sheet %q: need %d cards but it holds only %d distinct entries",
			sheet.Name, n, len(sheet.Cards))
	}

	finish := FinishNonFoil
	if sheet.Foil {
		finish = FinishFoil
	}

	// Working copy: set Weight=0 to mark a card as drawn.
	pool := make([]SheetCard, len(sheet.Cards))
	copy(pool, sheet.Cards)
	totalWeight := sheet.TotalWeight

	drawn := make([]CardKey, 0, n)

	if sheet.BalanceColors && n >= 5 {
		// Build per-colour candidate lists: only mono-coloured cards (len(Colors)==1).
		// Multi-colour and colourless cards are never forced; they remain in the fill pool.
		mono := make(map[string][]int, 5) // colour letter → pool indices
		for i, sc := range pool {
			c, ok := cards[sc.ID]
			if ok && len(c.Colors) == 1 {
				col := c.Colors[0]
				mono[col] = append(mono[col], i)
			}
		}

		// Pick one card per colour in alphabetical order (B G R U W). The order is fixed
		// here rather than in a global to satisfy the no-globals linter rule.
		for _, col := range [5]string{"B", "G", "R", "U", "W"} {
			idxs := mono[col]
			if len(idxs) == 0 {
				continue
			}
			// Weighted pick within this colour's candidates.
			colTotal := 0
			for _, idx := range idxs {
				colTotal += pool[idx].Weight
			}
			if colTotal == 0 {
				continue
			}
			pick := rng.IntN(colTotal)
			cum := 0
			chosen := idxs[0]
			for _, idx := range idxs {
				cum += pool[idx].Weight
				if pick < cum {
					chosen = idx
					break
				}
			}
			drawn = append(drawn, CardKey{ID: pool[chosen].ID, Finish: finish})
			totalWeight -= pool[chosen].Weight
			pool[chosen].Weight = 0
		}
	}

	// Fill remaining slots by weighted random without replacement.
	remaining := n - len(drawn)
	for i := range remaining {
		if totalWeight <= 0 {
			return nil, fmt.Errorf("sheet %q: pool exhausted before drawing card %d of %d", sheet.Name, len(drawn)+i+1, n)
		}
		pick := rng.IntN(totalWeight)
		cum := 0
		found := false
		for j, sc := range pool {
			if sc.Weight == 0 {
				continue
			}
			cum += sc.Weight
			if pick < cum {
				drawn = append(drawn, CardKey{ID: sc.ID, Finish: finish})
				totalWeight -= pool[j].Weight
				pool[j].Weight = 0
				found = true
				break
			}
		}
		if !found {
			// Defensive: should be unreachable given the totalWeight check above.
			return nil, fmt.Errorf("sheet %q: weighted draw failed at card %d (logic error)", sheet.Name, len(drawn)+1)
		}
	}

	return drawn, nil
}

// revealLess reports whether card key a sorts before b in reveal order.
// See classicGenerator for the full ordering rule.
func revealLess(a, b CardKey, cards map[CardID]Card) bool {
	rankA, foilA := revealKey(a, cards)
	rankB, foilB := revealKey(b, cards)
	if rankA != rankB {
		return rankA < rankB
	}
	if foilA != foilB {
		return foilA < foilB
	}
	return a.ID < b.ID
}

// revealKey returns (rarityRank, foilBit) for reveal ordering.
func revealKey(k CardKey, cards map[CardID]Card) (int, int) {
	rank := 3 // unknown: between special (2) and rare (4)
	if c, ok := cards[k.ID]; ok {
		rank = rarityRank(c.Rarity)
	}
	foilBit := 0
	if k.Finish == FinishFoil {
		foilBit = 1
	}
	return rank, foilBit
}

// rarityRank maps a Rarity to its reveal-order sort rank.
// Rare is always 4 (last). Unknown strings return 3 (between special and rare).
func rarityRank(r Rarity) int {
	switch r {
	case RarityCommon:
		return 0
	case RarityUncommon:
		return 1
	case RaritySpecial:
		return 2
	case RarityRare:
		return 4
	default:
		return 3
	}
}

// Opener opens boosters by delegating to a registered BoosterGenerator per layout.
// New layouts are added by registering a generator in the map passed to NewOpener;
// no existing callers need to change.
type Opener struct {
	generators map[BoosterLayout]BoosterGenerator
	rng        *rand.Rand
}

// NewOpener returns an Opener that delegates to the provided generators.
// The rng is used for all Open calls; callers must not use rng concurrently with Open.
func NewOpener(generators map[BoosterLayout]BoosterGenerator, rng *rand.Rand) *Opener {
	return &Opener{generators: generators, rng: rng}
}

// Open generates one booster from cat using the generator registered for layout.
// It returns an error if the layout is unknown or the generator fails.
func (o *Opener) Open(layout BoosterLayout, cat SetCatalogue) (Booster, error) {
	gen, ok := o.generators[layout]
	if !ok {
		return Booster{}, fmt.Errorf("open %q: unknown layout", layout)
	}
	return gen.Generate(cat, o.rng)
}
