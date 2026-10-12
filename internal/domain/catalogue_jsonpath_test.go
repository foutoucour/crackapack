package domain_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
)

// TestLoadSetCatalogue_jsonTypeErrorPathIsIndependentOfGoVersion pins the dotted path reported
// for a JSON type error. encoding/json fills UnmarshalTypeError.Field itself, but the Go
// releases differ on whether map keys (sheet names, "draft") appear in it, so the loader must
// not rely on it: the same file has to give the same field on every supported toolchain.
func TestLoadSetCatalogue_jsonTypeErrorPathIsIndependentOfGoVersion(t *testing.T) {
	t.Parallel()

	const card = `{"uuid":"u1","name":"n","number":"1","rarity":"common","finishes":["nonfoil"],"supertypes":[],"identifiers":{}}`

	tests := []struct {
		name      string
		json      string
		wantField string
	}{
		{
			name: "string where a sheet total weight is expected",
			json: `{"data":{"code":"SYN","name":"X","cards":[` + card + `],` +
				`"booster":{"draft":{"boosters":[],"sheets":{"common":{"cards":{"u1":1},"totalWeight":"x"}}}}}}`,
			wantField: "data.booster.draft.sheets.common.totalWeight",
		},
		{
			name: "number where a card rarity string is expected, in the second card",
			json: `{"data":{"code":"SYN","name":"X","cards":[` + card + `,` +
				`{"uuid":"u2","name":"n","number":"2","rarity":5,"finishes":["nonfoil"],"supertypes":[],"identifiers":{}}` +
				`],"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
			wantField: "data.cards[1].rarity",
		},
		{
			name: "string where a booster option weight is expected",
			json: `{"data":{"code":"SYN","name":"X","cards":[` + card + `],` +
				`"booster":{"draft":{"boosters":[{"contents":{"common":1},"weight":"heavy"}],"sheets":{}}}}}`,
			wantField: "data.booster.draft.boosters[0].weight",
		},
		{
			name: "object where the card list is expected",
			json: `{"data":{"code":"SYN","name":"X","cards":{},` +
				`"booster":{"draft":{"boosters":[],"sheets":{}}}}}`,
			wantField: "data.cards",
		},
		{
			name: "string where a sheet card weight is expected, with a second sheet before it",
			json: `{"data":{"code":"SYN","name":"X","cards":[` + card + `],` +
				`"booster":{"draft":{"boosters":[],"sheets":{` +
				`"alpha":{"cards":{"u1":1},"totalWeight":1},` +
				`"zeta":{"cards":{"u1":"many"},"totalWeight":1}}}}}}`,
			wantField: "data.booster.draft.sheets.zeta.cards.u1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"set.json": &fstest.MapFile{Data: []byte(tt.json)}}
			_, err := domain.LoadSetCatalogue(fsys, "set.json")
			var ce *domain.CatalogueError
			if !errors.As(err, &ce) {
				t.Fatalf("want *CatalogueError, got %T: %v", err, err)
			}
			if ce.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", ce.Field, tt.wantField)
			}
		})
	}
}
