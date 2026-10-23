package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// cardOf builds a minimal Card for use in inline catalogues.
func cardOf(id domain.CardID, rarity domain.Rarity, isBasic bool, finishes ...domain.Finish) domain.Card {
	return domain.Card{
		ID:          id,
		Name:        string(id),
		Number:      "1",
		Rarity:      rarity,
		Finishes:    finishes,
		IsBasicLand: isBasic,
	}
}

// catOf builds a minimal SetCatalogue from a slice of cards.
func catOf(cards ...domain.Card) domain.SetCatalogue {
	m := make(map[domain.CardID]domain.Card, len(cards))
	for _, c := range cards {
		m[c.ID] = c
	}
	return domain.SetCatalogue{Code: "TST", Name: "Test", Cards: m}
}

// ── Add ──────────────────────────────────────────────────────────────────────

func TestCollectionAdd(t *testing.T) {
	t.Parallel()

	key := domain.CardKey{ID: "card-a", Finish: domain.FinishNonFoil}
	snap := domain.PriceSnapshot{"card-a": 100}

	tests := []struct {
		name      string
		initial   int // copies added before the call under test (0 = skip)
		n         int
		wantValue domain.USDCents
		wantErr   bool
		wantN     int // expected InvalidQuantityError.N on error
	}{
		{name: "add one copy", n: 1, wantValue: 100},
		{name: "add many copies", n: 5, wantValue: 500},
		{name: "add twice accumulates", initial: 2, n: 3, wantValue: 500},
		{name: "zero is invalid", n: 0, wantErr: true, wantN: 0},
		{name: "negative is invalid", n: -1, wantErr: true, wantN: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := domain.NewCollection()
			if tc.initial > 0 {
				if err := c.Add(key, tc.initial); err != nil {
					t.Fatalf("setup Add(%d): %v", tc.initial, err)
				}
			}
			err := c.Add(key, tc.n)
			if tc.wantErr {
				var qErr *domain.InvalidQuantityError
				if !errors.As(err, &qErr) {
					t.Fatalf("Add(%d): want InvalidQuantityError, got %v", tc.n, err)
				}
				if qErr.N != tc.wantN {
					t.Errorf("InvalidQuantityError.N = %d, want %d", qErr.N, tc.wantN)
				}
				return
			}
			if err != nil {
				t.Fatalf("Add(%d): unexpected error: %v", tc.n, err)
			}
			if got := c.Value(snap, 0); got != tc.wantValue {
				t.Errorf("Value = %d, want %d", got, tc.wantValue)
			}
		})
	}
}

// ── AddBooster ───────────────────────────────────────────────────────────────

func TestCollectionAddBooster(t *testing.T) {
	t.Parallel()

	keyA := domain.CardKey{ID: "a", Finish: domain.FinishNonFoil}
	keyB := domain.CardKey{ID: "b", Finish: domain.FinishFoil}
	keyC := domain.CardKey{ID: "c", Finish: domain.FinishNonFoil}

	t.Run("adds one copy per card", func(t *testing.T) {
		t.Parallel()
		c := domain.NewCollection()
		c.AddBooster(domain.Booster{
			SetCode: "TST",
			Cards:   []domain.CardKey{keyA, keyB, keyC},
		})
		snap := domain.PriceSnapshot{"a": 100, "b": 200, "c": 300}
		// Total: 1×100 + 1×200 + 1×300 = 600
		if got := c.Value(snap, 0); got != 600 {
			t.Errorf("Value = %d, want 600", got)
		}
	})

	t.Run("duplicate key in booster counts twice", func(t *testing.T) {
		t.Parallel()
		c := domain.NewCollection()
		c.AddBooster(domain.Booster{
			SetCode: "TST",
			Cards:   []domain.CardKey{keyA, keyA},
		})
		snap := domain.PriceSnapshot{"a": 100}
		if got := c.Value(snap, 0); got != 200 {
			t.Errorf("Value = %d, want 200", got)
		}
	})
}

// ── Remove ───────────────────────────────────────────────────────────────────

func TestCollectionRemove(t *testing.T) {
	t.Parallel()

	key := domain.CardKey{ID: "card-r", Finish: domain.FinishNonFoil}
	snap := domain.PriceSnapshot{"card-r": 100}

	t.Run("remove partial decrements quantity", func(t *testing.T) {
		t.Parallel()
		c := domain.NewCollection()
		if err := c.Add(key, 3); err != nil {
			t.Fatal(err)
		}
		if err := c.Remove(key, 1); err != nil {
			t.Fatalf("Remove: unexpected error: %v", err)
		}
		if got := c.Value(snap, 0); got != 200 {
			t.Errorf("Value after Remove = %d, want 200", got)
		}
	})

	t.Run("remove all deletes key: subsequent remove sees zero quantity", func(t *testing.T) {
		t.Parallel()
		c := domain.NewCollection()
		if err := c.Add(key, 2); err != nil {
			t.Fatal(err)
		}
		if err := c.Remove(key, 2); err != nil {
			t.Fatalf("Remove: unexpected error: %v", err)
		}
		// Key deleted; a subsequent remove must see Have=0.
		err := c.Remove(key, 1)
		var insuffErr *domain.InsufficientQuantityError
		if !errors.As(err, &insuffErr) {
			t.Fatalf("want InsufficientQuantityError, got %v", err)
		}
		if insuffErr.Have != 0 {
			t.Errorf("Have = %d, want 0", insuffErr.Have)
		}
	})

	tests := []struct {
		name     string
		initial  int
		n        int
		wantErr  any // *InvalidQuantityError or *InsufficientQuantityError
		wantN    int // for InvalidQuantityError
		wantHave int // for InsufficientQuantityError
		wantWant int
	}{
		{
			name:    "n=0 invalid",
			initial: 1, n: 0,
			wantErr: (*domain.InvalidQuantityError)(nil),
			wantN:   0,
		},
		{
			name:    "n=-1 invalid",
			initial: 1, n: -1,
			wantErr: (*domain.InvalidQuantityError)(nil),
			wantN:   -1,
		},
		{
			name:    "remove more than owned",
			initial: 2, n: 5,
			wantErr:  (*domain.InsufficientQuantityError)(nil),
			wantHave: 2, wantWant: 5,
		},
		{
			name:    "remove from empty",
			initial: 0, n: 1,
			wantErr:  (*domain.InsufficientQuantityError)(nil),
			wantHave: 0, wantWant: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := domain.NewCollection()
			if tc.initial > 0 {
				if err := c.Add(key, tc.initial); err != nil {
					t.Fatal(err)
				}
			}
			valueBefore := c.Value(snap, 0)
			err := c.Remove(key, tc.n)

			switch tc.wantErr.(type) {
			case *domain.InvalidQuantityError:
				var qErr *domain.InvalidQuantityError
				if !errors.As(err, &qErr) {
					t.Fatalf("want InvalidQuantityError, got %v", err)
				}
				if qErr.N != tc.wantN {
					t.Errorf("N = %d, want %d", qErr.N, tc.wantN)
				}
			case *domain.InsufficientQuantityError:
				var insuffErr *domain.InsufficientQuantityError
				if !errors.As(err, &insuffErr) {
					t.Fatalf("want InsufficientQuantityError, got %v", err)
				}
				if insuffErr.Have != tc.wantHave {
					t.Errorf("Have = %d, want %d", insuffErr.Have, tc.wantHave)
				}
				if insuffErr.Want != tc.wantWant {
					t.Errorf("Want = %d, want %d", insuffErr.Want, tc.wantWant)
				}
			default:
				t.Fatalf("unhandled wantErr type in test case %q", tc.name)
			}

			// Atomicity: quantity unchanged after any error.
			if got := c.Value(snap, 0); got != valueBefore {
				t.Errorf("value changed on error: before=%d after=%d", valueBefore, got)
			}
		})
	}
}

// ── BinderProgress ───────────────────────────────────────────────────────────

func TestCollectionBinderProgress(t *testing.T) {
	t.Parallel()

	const (
		commonID   domain.CardID = "c-001"
		uncommonID domain.CardID = "u-001"
		rareID     domain.CardID = "r-001"
		basicID    domain.CardID = "b-001"
		outsiderID domain.CardID = "other-set-card"
	)

	bothFinishes := []domain.Finish{domain.FinishNonFoil, domain.FinishFoil}

	tests := []struct {
		name         string
		cat          domain.SetCatalogue
		setup        func(c *domain.Collection)
		wantTotal    int
		wantOwned    int
		wantPlaysets int
		wantByRarity map[domain.Rarity]domain.RarityProgress
	}{
		{
			name: "empty collection zero progress",
			cat: catOf(
				cardOf(commonID, domain.RarityCommon, false, domain.FinishNonFoil),
			),
			setup:     func(_ *domain.Collection) {},
			wantTotal: 1, wantOwned: 0, wantPlaysets: 0,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon: {Owned: 0, Total: 1},
			},
		},
		{
			name: "basic lands excluded from total and owned",
			cat: catOf(
				cardOf(commonID, domain.RarityCommon, false, domain.FinishNonFoil),
				cardOf(basicID, domain.RarityCommon, true, domain.FinishNonFoil),
			),
			setup: func(c *domain.Collection) {
				// Own only the basic land; the common is missing.
				_ = c.Add(domain.CardKey{ID: basicID, Finish: domain.FinishNonFoil}, 4)
			},
			wantTotal: 1, wantOwned: 0, wantPlaysets: 0,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon: {Owned: 0, Total: 1},
			},
		},
		{
			name: "foil copy counts as owned",
			cat:  catOf(cardOf(commonID, domain.RarityCommon, false, bothFinishes...)),
			setup: func(c *domain.Collection) {
				_ = c.Add(domain.CardKey{ID: commonID, Finish: domain.FinishFoil}, 1)
			},
			wantTotal: 1, wantOwned: 1, wantPlaysets: 0,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon: {Owned: 1, Total: 1},
			},
		},
		{
			name: "foil and nonfoil of same card count as one owned",
			cat:  catOf(cardOf(commonID, domain.RarityCommon, false, bothFinishes...)),
			setup: func(c *domain.Collection) {
				_ = c.Add(domain.CardKey{ID: commonID, Finish: domain.FinishNonFoil}, 1)
				_ = c.Add(domain.CardKey{ID: commonID, Finish: domain.FinishFoil}, 1)
			},
			wantTotal: 1, wantOwned: 1, wantPlaysets: 0,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon: {Owned: 1, Total: 1},
			},
		},
		{
			name: "2 nonfoil + 2 foil = playset",
			cat:  catOf(cardOf(commonID, domain.RarityCommon, false, bothFinishes...)),
			setup: func(c *domain.Collection) {
				_ = c.Add(domain.CardKey{ID: commonID, Finish: domain.FinishNonFoil}, 2)
				_ = c.Add(domain.CardKey{ID: commonID, Finish: domain.FinishFoil}, 2)
			},
			wantTotal: 1, wantOwned: 1, wantPlaysets: 1,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon: {Owned: 1, Total: 1},
			},
		},
		{
			name: "3 copies is not a playset",
			cat:  catOf(cardOf(commonID, domain.RarityCommon, false, bothFinishes...)),
			setup: func(c *domain.Collection) {
				_ = c.Add(domain.CardKey{ID: commonID, Finish: domain.FinishNonFoil}, 3)
			},
			wantTotal: 1, wantOwned: 1, wantPlaysets: 0,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon: {Owned: 1, Total: 1},
			},
		},
		{
			name: "card in collection but absent from catalogue is ignored",
			cat:  catOf(cardOf(commonID, domain.RarityCommon, false, domain.FinishNonFoil)),
			setup: func(c *domain.Collection) {
				// Own the outsider only; it must not affect this catalogue's progress.
				_ = c.Add(domain.CardKey{ID: outsiderID, Finish: domain.FinishNonFoil}, 4)
			},
			wantTotal: 1, wantOwned: 0, wantPlaysets: 0,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon: {Owned: 0, Total: 1},
			},
		},
		{
			name: "by rarity breakdown",
			cat: catOf(
				cardOf(commonID, domain.RarityCommon, false, domain.FinishNonFoil),
				cardOf(uncommonID, domain.RarityUncommon, false, domain.FinishNonFoil),
				cardOf(rareID, domain.RarityRare, false, domain.FinishNonFoil),
			),
			setup: func(c *domain.Collection) {
				// Own only the common.
				_ = c.Add(domain.CardKey{ID: commonID, Finish: domain.FinishNonFoil}, 1)
			},
			wantTotal: 3, wantOwned: 1, wantPlaysets: 0,
			wantByRarity: map[domain.Rarity]domain.RarityProgress{
				domain.RarityCommon:   {Owned: 1, Total: 1},
				domain.RarityUncommon: {Owned: 0, Total: 1},
				domain.RarityRare:     {Owned: 0, Total: 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := domain.NewCollection()
			tc.setup(c)
			p := c.BinderProgress(tc.cat)

			if p.Total != tc.wantTotal {
				t.Errorf("Total = %d, want %d", p.Total, tc.wantTotal)
			}
			if p.Owned != tc.wantOwned {
				t.Errorf("Owned = %d, want %d", p.Owned, tc.wantOwned)
			}
			if p.Playsets != tc.wantPlaysets {
				t.Errorf("Playsets = %d, want %d", p.Playsets, tc.wantPlaysets)
			}
			for rarity, want := range tc.wantByRarity {
				got := p.ByRarity[rarity]
				if got != want {
					t.Errorf("ByRarity[%s] = %+v, want %+v", rarity, got, want)
				}
			}
		})
	}
}

func TestCollectionBinderProgress_byColour(t *testing.T) {
	t.Parallel()

	both := []domain.Finish{domain.FinishNonFoil, domain.FinishFoil}
	card := func(id domain.CardID, colours ...string) domain.Card {
		c := cardOf(id, domain.RarityCommon, false, both...)
		c.Colors = colours
		return c
	}
	land := func(id domain.CardID, colours ...string) domain.Card {
		c := card(id, colours...)
		c.IsLand = true
		return c
	}
	basic := land("basic")
	basic.IsBasicLand = true

	cat := catOf(
		card("white-1", "W"), card("white-2", "W"),
		card("blue", "U"), card("black", "B"), card("red", "R"), card("green", "G"),
		card("gold", "W", "U"), card("gold-three", "B", "R", "G"),
		card("colourless"),
		land("land"), land("arbor", "G"),
		basic,
	)
	c := domain.NewCollection()
	for _, held := range []struct {
		id     domain.CardID
		finish domain.Finish
	}{
		{"white-1", domain.FinishNonFoil}, {"white-2", domain.FinishFoil},
		{"gold", domain.FinishNonFoil}, {"colourless", domain.FinishNonFoil},
		{"arbor", domain.FinishNonFoil}, {"basic", domain.FinishNonFoil},
	} {
		if err := c.Add(domain.CardKey{ID: held.id, Finish: held.finish}, 1); err != nil {
			t.Fatalf("Add %s: %v", held.id, err)
		}
	}

	got := c.BinderProgress(cat).ByColour

	want := map[domain.ColourGroup]domain.RarityProgress{
		domain.ColourWhite:      {Owned: 2, Total: 2},
		domain.ColourBlue:       {Owned: 0, Total: 1},
		domain.ColourBlack:      {Owned: 0, Total: 1},
		domain.ColourRed:        {Owned: 0, Total: 1},
		domain.ColourGreen:      {Owned: 0, Total: 1},
		domain.ColourGold:       {Owned: 1, Total: 2},
		domain.ColourColourless: {Owned: 1, Total: 1},
		domain.ColourLand:       {Owned: 1, Total: 2},
	}
	for group, w := range want {
		if got[group] != w {
			t.Errorf("ByColour[%s] = %+v, want %+v", group, got[group], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("ByColour has %d groups, want %d: %+v", len(got), len(want), got)
	}
}

func TestCollectionBinderProgress_byColourSumsToTheBinder(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(42)
	p := domain.NewCollection().BinderProgress(cat)

	total := 0
	for _, g := range p.ByColour {
		total += g.Total
	}
	if total != p.Total {
		t.Errorf("the colour groups add up to %d cards, the binder has %d: %+v", total, p.Total, p.ByColour)
	}
}

func TestColourGroups_listTheGroupsInTheOrderTheyAreShown(t *testing.T) {
	t.Parallel()

	want := []domain.ColourGroup{"W", "U", "B", "R", "G", "gold", "colourless", "land"}
	got := domain.ColourGroups()
	if len(got) != len(want) {
		t.Fatalf("ColourGroups() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ColourGroups() = %v, want %v", got, want)
		}
	}
}

// TestCollectionBinderProgressWithSyntheticCatalogue verifies progress against
// the domaintest catalogue, which has 10 commons, 4 uncommons, 2 rares and 2 basic lands.
func TestCollectionBinderProgressWithSyntheticCatalogue(t *testing.T) {
	t.Parallel()

	cat := domaintest.NewCatalogue(42)
	c := domain.NewCollection()
	// Add one non-foil copy of every card in the catalogue.
	for id := range cat.Cards {
		_ = c.Add(domain.CardKey{ID: id, Finish: domain.FinishNonFoil}, 1)
	}
	p := c.BinderProgress(cat)

	// 10 commons + 4 uncommons + 2 rares + 4 colorshifted = 20 non-basic cards.
	// 2 basic lands are excluded.
	const wantTotal = 22 // 10c + 4u + 2r + 4 cs-uncommon + 2 cs-rare
	if p.Total != wantTotal {
		t.Errorf("Total = %d, want %d", p.Total, wantTotal)
	}
	// All non-basic cards are owned (we added one copy of each card including basics,
	// but basics are excluded from Total/Owned).
	if p.Owned != wantTotal {
		t.Errorf("Owned = %d, want %d (all non-basic owned)", p.Owned, wantTotal)
	}
	// No playsets: only 1 copy each.
	if p.Playsets != 0 {
		t.Errorf("Playsets = %d, want 0", p.Playsets)
	}
}

// ── DuplicatesAbove ──────────────────────────────────────────────────────────

func TestCollectionDuplicatesAbove(t *testing.T) {
	t.Parallel()

	const id domain.CardID = "card-x"
	nfKey := domain.CardKey{ID: id, Finish: domain.FinishNonFoil}
	foilKey := domain.CardKey{ID: id, Finish: domain.FinishFoil}

	tests := []struct {
		name     string
		setup    func(c *domain.Collection)
		keep     int
		want     []domain.Surplus
		wantErr  bool
		wantKeep int // expected in InvalidKeepError
	}{
		{
			name:  "no surplus at keep=4",
			setup: func(c *domain.Collection) { _ = c.Add(nfKey, 4) },
			keep:  4,
			want:  nil,
		},
		{
			name:  "one surplus all nonfoil",
			setup: func(c *domain.Collection) { _ = c.Add(nfKey, 5) },
			keep:  4,
			want:  []domain.Surplus{{Key: nfKey, Quantity: 1}},
		},
		{
			name:  "surplus all foil when no nonfoil",
			setup: func(c *domain.Collection) { _ = c.Add(foilKey, 5) },
			keep:  4,
			want:  []domain.Surplus{{Key: foilKey, Quantity: 1}},
		},
		{
			name: "nonfoil taken first foils kept: 3nf+3foil keep=4",
			setup: func(c *domain.Collection) {
				_ = c.Add(nfKey, 3)
				_ = c.Add(foilKey, 3)
			},
			keep: 4,
			want: []domain.Surplus{{Key: nfKey, Quantity: 2}},
		},
		{
			name: "surplus splits across finishes when nonfoil exhausted: 1nf+3foil keep=2",
			setup: func(c *domain.Collection) {
				_ = c.Add(nfKey, 1)
				_ = c.Add(foilKey, 3)
			},
			keep: 2,
			want: []domain.Surplus{
				{Key: nfKey, Quantity: 1},
				{Key: foilKey, Quantity: 1},
			},
		},
		{
			name:    "keep=0 invalid",
			setup:   func(_ *domain.Collection) {},
			keep:    0,
			wantErr: true, wantKeep: 0,
		},
		{
			name:    "keep=5 invalid",
			setup:   func(_ *domain.Collection) {},
			keep:    5,
			wantErr: true, wantKeep: 5,
		},
		{
			name:  "keep=1 valid",
			setup: func(c *domain.Collection) { _ = c.Add(nfKey, 2) },
			keep:  1,
			want:  []domain.Surplus{{Key: nfKey, Quantity: 1}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := domain.NewCollection()
			tc.setup(c)
			got, err := c.DuplicatesAbove(tc.keep)
			if tc.wantErr {
				var kErr *domain.InvalidKeepError
				if !errors.As(err, &kErr) {
					t.Fatalf("want InvalidKeepError, got %v", err)
				}
				if kErr.Keep != tc.wantKeep {
					t.Errorf("Keep = %d, want %d", kErr.Keep, tc.wantKeep)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("DuplicatesAbove(%d) = %v, want %v", tc.keep, got, tc.want)
			}
			for i, s := range got {
				if s != tc.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, s, tc.want[i])
				}
			}
		})
	}
}

// TestCollectionDuplicatesAboveDeterministic verifies that results are always sorted
// by CardKey regardless of map iteration order.
func TestCollectionDuplicatesAboveDeterministic(t *testing.T) {
	t.Parallel()

	// "z-001" sorts after "a-001"; map iteration may return either first.
	aKey := domain.CardKey{ID: "a-001", Finish: domain.FinishNonFoil}
	zKey := domain.CardKey{ID: "z-001", Finish: domain.FinishNonFoil}

	c := domain.NewCollection()
	_ = c.Add(zKey, 5) // surplus 1 at keep=4
	_ = c.Add(aKey, 5) // surplus 1 at keep=4

	got, err := c.DuplicatesAbove(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 surplus entries, got %d: %v", len(got), got)
	}
	if got[0].Key.ID != "a-001" {
		t.Errorf("first entry = %v, want a-001 first (sorted)", got[0].Key.ID)
	}
	if got[1].Key.ID != "z-001" {
		t.Errorf("second entry = %v, want z-001 second (sorted)", got[1].Key.ID)
	}
}

// ── Value ────────────────────────────────────────────────────────────────────

func TestCollectionValue(t *testing.T) {
	t.Parallel()

	const (
		idA domain.CardID = "a"
		idB domain.CardID = "b"
	)
	nfA := domain.CardKey{ID: idA, Finish: domain.FinishNonFoil}
	foilA := domain.CardKey{ID: idA, Finish: domain.FinishFoil}
	nfB := domain.CardKey{ID: idB, Finish: domain.FinishNonFoil}

	snap := domain.PriceSnapshot{idA: 100} // idB intentionally absent

	tests := []struct {
		name     string
		setup    func(c *domain.Collection)
		fallback domain.USDCents
		want     domain.USDCents
	}{
		{
			name:     "empty collection is zero",
			setup:    func(_ *domain.Collection) {},
			fallback: 50,
			want:     0,
		},
		{
			name: "two copies at 100 cents each",
			setup: func(c *domain.Collection) {
				_ = c.Add(nfA, 2)
			},
			fallback: 0,
			want:     200,
		},
		{
			name: "foil and nonfoil share the same price: 1nf+1foil = 200",
			setup: func(c *domain.Collection) {
				_ = c.Add(nfA, 1)
				_ = c.Add(foilA, 1)
			},
			fallback: 0,
			want:     200,
		},
		{
			name: "card absent from snap uses fallback",
			setup: func(c *domain.Collection) {
				_ = c.Add(nfB, 3) // idB has no entry in snap
			},
			fallback: 50,
			want:     150,
		},
		{
			name: "multiple cards summed",
			setup: func(c *domain.Collection) {
				_ = c.Add(nfA, 2) // 2 × 100 = 200
				_ = c.Add(nfB, 1) // 1 × 50 fallback = 50
			},
			fallback: 50,
			want:     250,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := domain.NewCollection()
			tc.setup(c)
			got := c.Value(snap, tc.fallback)
			if got != tc.want {
				t.Errorf("Value = %d, want %d", got, tc.want)
			}
		})
	}
}

// ── CoinValue ────────────────────────────────────────────────────────────────

func TestCollectionCoinValue(t *testing.T) {
	t.Parallel()

	const id domain.CardID = "coin-card"
	nfKey := domain.CardKey{ID: id, Finish: domain.FinishNonFoil}

	tests := []struct {
		name     string
		setup    func(c *domain.Collection)
		snap     domain.PriceSnapshot
		fallback domain.USDCents
		rate     domain.CoinRate
		want     domain.Coins
		wantErr  error
	}{
		{
			name:     "empty collection is zero coins no error",
			setup:    func(_ *domain.Collection) {},
			snap:     domain.PriceSnapshot{},
			fallback: 0,
			rate:     100,
			want:     0,
			wantErr:  nil,
		},
		{
			name: "normal conversion: 100 cents at 100 coins per USD = 100 coins",
			setup: func(c *domain.Collection) {
				_ = c.Add(nfKey, 1)
			},
			snap:    domain.PriceSnapshot{id: 100},
			rate:    100,
			want:    100,
			wantErr: nil,
		},
		{
			name: "ErrSubCoin when total rounds to zero",
			// 5 cents × rate 10 = 50 / 10000 < 1 coin
			setup: func(c *domain.Collection) {
				_ = c.Add(nfKey, 1)
			},
			snap:    domain.PriceSnapshot{id: 5},
			rate:    10,
			want:    0,
			wantErr: domain.ErrSubCoin,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := domain.NewCollection()
			tc.setup(c)
			got, err := c.CoinValue(tc.snap, tc.fallback, tc.rate)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("CoinValue = %d, want %d", got, tc.want)
			}
		})
	}
}
