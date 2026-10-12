package domain

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

// LoadPriceSnapshot reads a JSON file from fsys and returns a PriceSnapshot.
//
// The file must contain a JSON object mapping card UUIDs (string) to prices in
// US cents (integer). An empty file or an empty object returns an empty snapshot
// with no error.
//
// Errors:
//   - file cannot be opened or read: error names the file
//   - malformed JSON: error names the file
//   - any price is negative: error names the file and the card ID
func LoadPriceSnapshot(fsys fs.FS, name string) (PriceSnapshot, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("price snapshot %q: open: %w", name, err)
	}
	defer func() {
		_ = f.Close() // read-only: close error is not actionable
	}()

	// Decode into a raw map so we can validate each value before constructing
	// the typed PriceSnapshot.
	var raw map[string]int64
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		// An empty file produces io.EOF from Decode; treat it as an empty snapshot.
		// json.NewDecoder returns io.EOF only when the reader is empty.
		if err.Error() == "EOF" {
			return PriceSnapshot{}, nil
		}
		return nil, fmt.Errorf("price snapshot %q: %w", name, err)
	}

	snap := make(PriceSnapshot, len(raw))
	for id, cents := range raw {
		if cents < 0 {
			return nil, fmt.Errorf("price snapshot %q: card %q: price must be >= 0, got %d", name, id, cents)
		}
		snap[CardID(id)] = USDCents(cents)
	}
	return snap, nil
}
