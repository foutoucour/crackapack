package domain_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// withKeyruneCode injects a keyruneCode field into the minimal set JSON.
func withKeyruneCode(code string) string {
	return strings.Replace(minimalSetJSON(""), `"name": "Synthetic",`, `"name": "Synthetic", "keyruneCode": "`+code+`",`, 1)
}

// TestLoadSetCatalogue_keyruneCodeLoaded verifies that the code of the set's icon is read as given.
func TestLoadSetCatalogue_keyruneCodeLoaded(t *testing.T) {
	t.Parallel()

	cat, err := domain.LoadSetCatalogue(mapFSWith("set.json", withKeyruneCode("RAV")), "set.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue: %v", err)
	}
	if cat.IconCode != "RAV" {
		t.Errorf("IconCode = %q, want %q", cat.IconCode, "RAV")
	}
}

// TestLoadSetCatalogue_keyruneCodeAbsent verifies that a set without an icon code loads, so that a
// set file written before the icons still opens.
func TestLoadSetCatalogue_keyruneCodeAbsent(t *testing.T) {
	t.Parallel()

	cat, err := domain.LoadSetCatalogue(mapFSWith("set.json", minimalSetJSON("")), "set.json")
	if err != nil {
		t.Fatalf("LoadSetCatalogue: %v", err)
	}
	if cat.IconCode != "" {
		t.Errorf("IconCode = %q, want empty when the field is absent", cat.IconCode)
	}
}
