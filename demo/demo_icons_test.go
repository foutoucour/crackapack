package demo_test

import (
	"encoding/xml"
	"io/fs"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/demo"
)

func TestBuild_everySetOfTheDirectoryHasAnIconNamedAfterIt(t *testing.T) {
	t.Parallel()
	ds := demo.Build()

	for _, code := range []string{"DEM", "DSP", "DEX"} {
		raw, err := fs.ReadFile(ds.Files, code+".svg")
		if err != nil {
			t.Errorf("%s has no icon: %v", code, err)
			continue
		}
		var root struct {
			XMLName xml.Name
		}
		if err := xml.Unmarshal(raw, &root); err != nil || root.XMLName.Local != "svg" {
			t.Errorf("%s.svg is not an SVG document (root %q, error %v):\n%s", code, root.XMLName.Local, err, raw)
		}
		if !strings.Contains(string(raw), code) {
			t.Errorf("%s.svg does not show the set code, so the sets look alike:\n%s", code, raw)
		}
	}
}
