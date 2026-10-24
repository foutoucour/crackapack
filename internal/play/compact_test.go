package play

import (
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestCompact_roundTrip(t *testing.T) {
	t.Parallel()
	foil := domain.CardKey{ID: "b2", Finish: domain.FinishFoil}
	plain := domain.CardKey{ID: "a1", Finish: domain.FinishNonFoil}
	counts := map[domain.CardKey]int{plain: 1, foil: 3}
	shelf := map[domain.CardKey][]int{plain: {4, 4, 9}, foil: {0}}
	wishes := []domain.Wish{{ID: "z9", Want: 1}, {ID: "a1", Want: 4}}
	dict := newCardDict()

	// A card is named by its place in the dictionary, in the order the lists first use it.
	s := encodeCounts(counts, dict)
	if s != "0 1*=3" {
		t.Errorf("encodeCounts = %q, want %q", s, "0 1*=3")
	}
	if s = encodeShelf(shelf, dict); s != "0@4=2 0@9 1*@0" {
		t.Errorf("encodeShelf = %q, want %q", s, "0@4=2 0@9 1*@0")
	}
	if s = encodeWishes(wishes, dict); s != "2 0=4" {
		t.Errorf("encodeWishes = %q, want %q", s, "2 0=4")
	}
	if want := []domain.CardID{"a1", "b2", "z9"}; !reflect.DeepEqual(dict.ids, want) {
		t.Errorf("dictionary = %v, want %v", dict.ids, want)
	}

	read := readCardDict(dict.ids)
	if got, err := decodeCounts("0 1*=3", read); err != nil || !reflect.DeepEqual(got, counts) {
		t.Errorf("decodeCounts = %v, %v, want %v", got, err, counts)
	}
	if got, err := decodeShelf("0@4=2 0@9 1*@0", read); err != nil || !reflect.DeepEqual(got, shelf) {
		t.Errorf("decodeShelf = %v, %v, want %v", got, err, shelf)
	}
	if got, err := decodeWishes("2 0=4", read); err != nil || !reflect.DeepEqual(got, wishes) {
		t.Errorf("decodeWishes = %v, %v, want %v", got, err, wishes)
	}
}

// A save from before the dictionary writes the card IDs in the lists, and has no dictionary.
func TestCompact_anOlderSaveNamesTheCardsInTheLists(t *testing.T) {
	t.Parallel()
	older := readCardDict(nil)
	foil := domain.CardKey{ID: "b2", Finish: domain.FinishFoil}
	plain := domain.CardKey{ID: "a1", Finish: domain.FinishNonFoil}

	if got, err := decodeCounts("a1 b2*=3", older); err != nil || !reflect.DeepEqual(got, map[domain.CardKey]int{plain: 1, foil: 3}) {
		t.Errorf("decodeCounts = %v, %v", got, err)
	}
	if got, err := decodeShelf("a1@4=2 b2*@0", older); err != nil || !reflect.DeepEqual(got, map[domain.CardKey][]int{plain: {4, 4}, foil: {0}}) {
		t.Errorf("decodeShelf = %v, %v", got, err)
	}
}

func TestCompact_anIDWithACharacterOfTheEntriesRoundTrips(t *testing.T) {
	t.Parallel()
	odd := domain.CardKey{ID: "a b=@*", Finish: domain.FinishNonFoil}
	dict := newCardDict()

	s := encodeCounts(map[domain.CardKey]int{odd: 2}, dict)

	got, err := decodeCounts(s, readCardDict(dict.ids))
	if err != nil || got[odd] != 2 {
		t.Errorf("decodeCounts(%q) = %v, %v, want 2 of %q", s, got, err, odd.ID)
	}
}

func TestCompact_refuses(t *testing.T) {
	t.Parallel()
	dict := readCardDict([]domain.CardID{"a1"})
	for _, s := range []string{"0=0", "0=x", "=2", "*", "0@3", "0 0", "1", "-1", "a1"} {
		if _, err := decodeCounts(s, dict); err == nil {
			t.Errorf("decodeCounts(%q) went through", s)
		}
	}
	for _, s := range []string{"0", "0@-1", "0@x", "@3", "1@3"} {
		if _, err := decodeShelf(s, dict); err == nil {
			t.Errorf("decodeShelf(%q) went through", s)
		}
	}
	if _, err := decodeWishes("0*", dict); err == nil {
		t.Error("a foil wish went through")
	}
	for _, s := range []string{"a1=0", "a1=x", "a1 a1", "a=1@2"} {
		if _, err := decodeCounts(s, readCardDict(nil)); err == nil {
			t.Errorf("decodeCounts(%q) of an older save went through", s)
		}
	}
}

// Whatever a save holds, reading its NPCs' cards never panics, and what reads writes back to
// something that reads the same.
func FuzzDecodeShelf(f *testing.F) {
	for _, seed := range []string{"a1@4=2 a1@9 b2*@0", "", "x@1=1", "@@=", "a*@2=3"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		shelf, err := decodeShelf(s, readCardDict(nil))
		if err != nil {
			return
		}
		dict := newCardDict()
		again := encodeShelf(shelf, dict)
		back, err := decodeShelf(again, readCardDict(dict.ids))
		if err != nil {
			t.Fatalf("decodeShelf(%q) = %v, which encodes to %q that does not read: %v", s, shelf, again, err)
		}
		for key, ticks := range shelf {
			if len(back[key]) != len(ticks) {
				t.Fatalf("decodeShelf(%q) = %v, read back as %v", s, shelf, back)
			}
		}
	})
}
