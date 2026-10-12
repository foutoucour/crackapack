package domain_test

import (
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestLoadPriceSnapshot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		content  string
		wantSnap domain.PriceSnapshot
		wantErr  string // substring; empty means no error expected
	}{
		{
			name:     "happy path round-trip",
			filename: "prices.json",
			content:  `{"card-a": 100, "card-b": 250}`,
			wantSnap: domain.PriceSnapshot{
				"card-a": 100,
				"card-b": 250,
			},
		},
		{
			name:     "empty file returns empty snapshot",
			filename: "prices.json",
			content:  "",
			wantSnap: domain.PriceSnapshot{},
		},
		{
			name:     "empty object returns empty snapshot",
			filename: "prices.json",
			content:  `{}`,
			wantSnap: domain.PriceSnapshot{},
		},
		{
			name:     "zero price is accepted",
			filename: "prices.json",
			content:  `{"card-a": 0}`,
			wantSnap: domain.PriceSnapshot{"card-a": 0},
		},
		{
			name:     "negative price names file and card",
			filename: "prices.json",
			content:  `{"card-bad": -5}`,
			wantErr:  `price snapshot "prices.json": card "card-bad": price must be >= 0, got -5`,
		},
		{
			name:     "malformed JSON names file",
			filename: "prices.json",
			content:  `{"card-a": "not-a-number"}`,
			wantErr:  `price snapshot "prices.json"`,
		},
		{
			name:     "truncated JSON names file",
			filename: "prices.json",
			content:  `{"card-a": 1`,
			wantErr:  `price snapshot "prices.json"`,
		},
		{
			name:     "file not found names file",
			filename: "missing.json",
			content:  "", // will not be written; we open a different name
			wantErr:  `price snapshot "missing.json"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fsys := fstest.MapFS{}
			if tc.content != "" || tc.filename != "missing.json" {
				fsys["prices.json"] = &fstest.MapFile{Data: []byte(tc.content)}
			}

			got, err := domain.LoadPriceSnapshot(fsys, tc.filename)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", tc.wantErr)
				}
				if msg := err.Error(); !contains(msg, tc.wantErr) {
					t.Fatalf("error %q does not contain %q", msg, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.wantSnap) {
				t.Fatalf("snapshot len = %d, want %d", len(got), len(tc.wantSnap))
			}
			for id, want := range tc.wantSnap {
				if got[id] != want {
					t.Errorf("price[%q] = %d, want %d", id, got[id], want)
				}
			}
		})
	}
}

// contains is a helper to avoid importing strings in test files.
func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && searchSubstring(s, sub))
}

func searchSubstring(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
