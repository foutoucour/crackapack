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

	s, err := encodeCounts(counts)
	if err != nil || s != "a1 b2*=3" {
		t.Errorf("encodeCounts = %q, %v, want %q", s, err, "a1 b2*=3")
	}
	if got, err := decodeCounts(s); err != nil || !reflect.DeepEqual(got, counts) {
		t.Errorf("decodeCounts = %v, %v, want %v", got, err, counts)
	}
	s, err = encodeShelf(shelf)
	if err != nil || s != "a1@4=2 a1@9 b2*@0" {
		t.Errorf("encodeShelf = %q, %v, want %q", s, err, "a1@4=2 a1@9 b2*@0")
	}
	if got, err := decodeShelf(s); err != nil || !reflect.DeepEqual(got, shelf) {
		t.Errorf("decodeShelf = %v, %v, want %v", got, err, shelf)
	}
	s, err = encodeWishes(wishes)
	if err != nil || s != "z9 a1=4" {
		t.Errorf("encodeWishes = %q, %v, want %q", s, err, "z9 a1=4")
	}
	if got, err := decodeWishes(s); err != nil || !reflect.DeepEqual(got, wishes) {
		t.Errorf("decodeWishes = %v, %v, want %v", got, err, wishes)
	}
}

func TestCompact_refuses(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"a1=0", "a1=x", "=2", "*", "a1@3", "a1 a1"} {
		if _, err := decodeCounts(s); err == nil {
			t.Errorf("decodeCounts(%q) went through", s)
		}
	}
	for _, s := range []string{"a1", "a1@-1", "a1@x", "@3"} {
		if _, err := decodeShelf(s); err == nil {
			t.Errorf("decodeShelf(%q) went through", s)
		}
	}
	if _, err := decodeWishes("a1*"); err == nil {
		t.Error("a foil wish went through")
	}
	if _, err := encodeCounts(map[domain.CardKey]int{{ID: "a b", Finish: domain.FinishNonFoil}: 1}); err == nil {
		t.Error("an ID with a space was encoded")
	}
}

// Whatever a save holds, reading its NPCs' cards never panics, and what reads writes back to
// something that reads the same.
func FuzzDecodeShelf(f *testing.F) {
	for _, seed := range []string{"a1@4=2 a1@9 b2*@0", "", "x@1=1", "@@=", "a*@2=3"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		shelf, err := decodeShelf(s)
		if err != nil {
			return
		}
		again, err := encodeShelf(shelf)
		if err != nil {
			return
		}
		back, err := decodeShelf(again)
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
