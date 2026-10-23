package play

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
)

// The cards of the NPCs are most of what a save holds, so a save writes them compactly, see
// docs/adr/0061-the-npcs-are-saved-in-the-games-file-in-a-compact-form.md. A list of cards is one
// string of entries separated by spaces. An entry is a card ID, then "*" for a foil, "@" and a tick
// for the copies of a shelf that arrived on it, and "=" and a count when the count is not 1:
// "a1b2 c3d4*=2 e5f6@288=3".

// errCompactID is why a card ID cannot be written compactly: a character the entries use.
var errCompactID = errors.New("a card ID with a space, '=', '@' or '*' cannot be saved compactly")

// compactEntry is one entry of a compact list.
type compactEntry struct {
	key   domain.CardKey
	tick  int
	ticks bool // the entry has a tick
	n     int
}

func (e compactEntry) String() string {
	var b strings.Builder
	b.WriteString(string(e.key.ID))
	if e.key.Finish == domain.FinishFoil {
		b.WriteByte('*')
	}
	if e.ticks {
		b.WriteByte('@')
		b.WriteString(strconv.Itoa(e.tick))
	}
	if e.n != 1 {
		b.WriteByte('=')
		b.WriteString(strconv.Itoa(e.n))
	}
	return b.String()
}

func joinEntries(entries []compactEntry) (string, error) {
	parts := make([]string, len(entries))
	for i, e := range entries {
		if e.key.ID == "" || strings.ContainsAny(string(e.key.ID), " =@*") {
			return "", fmt.Errorf("%w: %q", errCompactID, e.key.ID)
		}
		parts[i] = e.String()
	}
	return strings.Join(parts, " "), nil
}

// splitEntries reads a compact list. An entry must have a tick when ticks is true and none
// otherwise, and a count of at least 1.
func splitEntries(s string, ticks bool) ([]compactEntry, error) {
	fields := strings.Fields(s)
	out := make([]compactEntry, 0, len(fields))
	for _, f := range fields {
		e := compactEntry{n: 1}
		rest := f
		if i := strings.LastIndexByte(rest, '='); i >= 0 {
			n, err := strconv.Atoi(rest[i+1:])
			if err != nil || n < 1 {
				return nil, fmt.Errorf("entry %q: the count is not a whole number of at least 1", f)
			}
			e.n, rest = n, rest[:i]
		}
		if i := strings.LastIndexByte(rest, '@'); i >= 0 {
			tick, err := strconv.Atoi(rest[i+1:])
			if err != nil || tick < 0 {
				return nil, fmt.Errorf("entry %q: the tick is not a whole number", f)
			}
			e.tick, e.ticks, rest = tick, true, rest[:i]
		}
		if e.ticks != ticks {
			return nil, fmt.Errorf("entry %q: a tick is where none belongs, or missing", f)
		}
		e.key.Finish = domain.FinishNonFoil
		if id, foil := strings.CutSuffix(rest, "*"); foil {
			e.key.Finish, rest = domain.FinishFoil, id
		}
		if rest == "" || strings.ContainsAny(rest, "=@*") {
			return nil, fmt.Errorf("entry %q: no card ID", f)
		}
		e.key.ID = domain.CardID(rest)
		out = append(out, e)
	}
	return out, nil
}

// encodeCounts writes counts by card, in card order.
func encodeCounts(counts map[domain.CardKey]int) (string, error) {
	entries := make([]compactEntry, 0, len(counts))
	for _, key := range slices.SortedFunc(maps.Keys(counts), compareKeys) {
		entries = append(entries, compactEntry{key: key, n: counts[key]})
	}
	return joinEntries(entries)
}

// decodeCounts reads what encodeCounts wrote. A card listed twice is an error.
func decodeCounts(s string) (map[domain.CardKey]int, error) {
	entries, err := splitEntries(s, false)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.CardKey]int, len(entries))
	for _, e := range entries {
		if _, twice := out[e.key]; twice {
			return nil, fmt.Errorf("card %q finish %q is listed twice", e.key.ID, e.key.Finish)
		}
		out[e.key] = e.n
	}
	return out, nil
}

// encodeShelf writes a shelf by card, then by tick, the copies of a card that arrived on one tick
// as one entry.
func encodeShelf(shelf map[domain.CardKey][]int) (string, error) {
	var entries []compactEntry
	for _, key := range slices.SortedFunc(maps.Keys(shelf), compareKeys) {
		for _, tick := range shelf[key] {
			if last := len(entries) - 1; last >= 0 && entries[last].key == key && entries[last].tick == tick {
				entries[last].n++
				continue
			}
			entries = append(entries, compactEntry{key: key, tick: tick, ticks: true, n: 1})
		}
	}
	return joinEntries(entries)
}

// decodeShelf reads what encodeShelf wrote. Whether the ticks are in order is the crowd's to check.
func decodeShelf(s string) (map[domain.CardKey][]int, error) {
	entries, err := splitEntries(s, true)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.CardKey][]int)
	for _, e := range entries {
		for range e.n {
			out[e.key] = append(out[e.key], e.tick)
		}
	}
	return out, nil
}

// encodeWishes writes a wish list in its order, the copies wanted as the count.
func encodeWishes(wishes []domain.Wish) (string, error) {
	entries := make([]compactEntry, len(wishes))
	for i, w := range wishes {
		entries[i] = compactEntry{key: domain.CardKey{ID: w.ID, Finish: domain.FinishNonFoil}, n: w.Want}
	}
	return joinEntries(entries)
}

// decodeWishes reads what encodeWishes wrote.
func decodeWishes(s string) ([]domain.Wish, error) {
	entries, err := splitEntries(s, false)
	if err != nil {
		return nil, err
	}
	var out []domain.Wish
	for _, e := range entries {
		if e.key.Finish != domain.FinishNonFoil {
			return nil, fmt.Errorf("wish for %q: a wish has no finish", e.key.ID)
		}
		out = append(out, domain.Wish{ID: e.key.ID, Want: e.n})
	}
	return out, nil
}

// encodeKeys writes card keys in their order, repeats included.
func encodeKeys(keys []domain.CardKey) (string, error) {
	entries := make([]compactEntry, len(keys))
	for i, key := range keys {
		entries[i] = compactEntry{key: key, n: 1}
	}
	return joinEntries(entries)
}

// decodeKeys reads what encodeKeys wrote.
func decodeKeys(s string) ([]domain.CardKey, error) {
	entries, err := splitEntries(s, false)
	if err != nil {
		return nil, err
	}
	var out []domain.CardKey
	for _, e := range entries {
		for range e.n {
			out = append(out, e.key)
		}
	}
	return out, nil
}

func compareKeys(a, b domain.CardKey) int {
	return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Finish, b.Finish))
}
