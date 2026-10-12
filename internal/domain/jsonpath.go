package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// jsonFrame is one open JSON container while jsonPathAt walks the tokens.
type jsonFrame struct {
	isObject  bool
	key       string // current key of an object
	expectKey bool   // an object is waiting for its next key
	index     int    // current element of an array
}

// jsonPathAt returns the dotted path of the JSON value that ends at byte offset in data, in the
// form the loader uses for its own validation errors: "data.booster.draft.sheets.common.totalWeight"
// or "data.cards[3].rarity". It reports false when the offset does not fall on a value.
//
// encoding/json fills UnmarshalTypeError.Field itself, but releases differ on whether map keys
// and array indexes appear in it (and how), so the loader does not rely on it. The Offset field of
// UnmarshalTypeError is the byte offset reached when the mismatch was found: the end of a scalar,
// or just after the opening bracket of a container.
func jsonPathAt(data []byte, offset int64) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var stack []jsonFrame

	for {
		tok, err := dec.Token()
		if err != nil {
			return "", false
		}

		if delim, ok := tok.(json.Delim); ok && (delim == '}' || delim == ']') {
			stack = stack[:len(stack)-1]
			markValueDone(stack)
			continue
		}

		if isKeyToken(stack) {
			stack[len(stack)-1].key = tok.(string)
			stack[len(stack)-1].expectKey = false
			continue
		}

		if dec.InputOffset() >= offset {
			return formatJSONPath(stack), len(stack) > 0
		}

		if delim, ok := tok.(json.Delim); ok {
			stack = append(stack, jsonFrame{isObject: delim == '{', expectKey: delim == '{'})
			continue
		}
		markValueDone(stack)
	}
}

// isKeyToken reports whether the next token is an object key rather than a value.
func isKeyToken(stack []jsonFrame) bool {
	return len(stack) > 0 && stack[len(stack)-1].isObject && stack[len(stack)-1].expectKey
}

// markValueDone records that a value of the innermost container has been fully read.
func markValueDone(stack []jsonFrame) {
	if len(stack) == 0 {
		return
	}
	top := &stack[len(stack)-1]
	if top.isObject {
		top.expectKey = true
		return
	}
	top.index++
}

func formatJSONPath(stack []jsonFrame) string {
	var sb strings.Builder
	for _, f := range stack {
		if f.isObject {
			if sb.Len() > 0 {
				sb.WriteByte('.')
			}
			sb.WriteString(f.key)
			continue
		}
		fmt.Fprintf(&sb, "[%d]", f.index)
	}
	return sb.String()
}
