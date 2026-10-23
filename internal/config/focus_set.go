package config

import (
	"fmt"

	yaml "go.yaml.in/yaml/v3"

	"github.com/foutoucour/crackapack/internal/domain"
)

// FocusSet is the config encoding of domain.FocusSetMode. It decodes YAML scalars:
//   - false, "false", no, off (and other YAML bool-false forms) → FocusSetNone
//   - true, "true", yes, on  (and other YAML bool-true forms)   → FocusSetRandom
//   - "oldest" (bare or quoted)                                  → FocusSetOldest
//
// Any other value — "newest", an integer, a list or a map — is rejected with an error
// that names the key and mentions the three accepted forms.
type FocusSet domain.FocusSetMode

const (
	// FocusSetNone is the zero value: the NPC picks a random set each booster.
	// It corresponds to focus_set: false in YAML.
	FocusSetNone FocusSet = FocusSet(domain.FocusSetNone)
	// FocusSetRandom draws one set at generation; the NPC opens only that set.
	// It corresponds to focus_set: true in YAML.
	FocusSetRandom FocusSet = FocusSet(domain.FocusSetRandom)
	// FocusSetOldest resolves to the set with the earliest release date.
	// It corresponds to focus_set: oldest in YAML. See ADR-0026.
	FocusSetOldest FocusSet = FocusSet(domain.FocusSetOldest)
)

// MarshalYAML encodes a FocusSet as the scalar UnmarshalYAML reads back, so a Config written as
// YAML loads again.
func (f FocusSet) MarshalYAML() (any, error) {
	switch f {
	case FocusSetRandom:
		return true, nil
	case FocusSetOldest:
		return "oldest", nil
	case FocusSetNone:
		return false, nil
	default:
		return nil, fmt.Errorf("focus_set: unknown mode %d", f)
	}
}

// UnmarshalYAML decodes a focus_set YAML scalar into a FocusSet. It accepts the boolean
// forms that yaml.v3 recognises (true, false, yes, no, on, off, and their capitalisations)
// and also the string "oldest".
func (f *FocusSet) UnmarshalYAML(node *yaml.Node) error {
	if node.Value == "oldest" {
		*f = FocusSetOldest
		return nil
	}
	var b bool
	if err := node.Decode(&b); err != nil {
		return &yaml.TypeError{Errors: []string{fmt.Sprintf(
			"line %d: cannot decode %q as focus_set, want true, false or oldest",
			node.Line, node.Value,
		)}}
	}
	if b {
		*f = FocusSetRandom
	} else {
		*f = FocusSetNone
	}
	return nil
}
