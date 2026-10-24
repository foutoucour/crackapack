package demo

import (
	"fmt"
	"html"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain"
)

// addSetIcons puts one icon per set into files, named "<CODE>.svg" as fetchdata names the ones it
// downloads. The demo sets have no icon on Scryfall and the demo needs no network.
func addSetIcons(files fstest.MapFS, cats ...domain.SetCatalogue) {
	for _, cat := range cats {
		files[cat.Code+".svg"] = &fstest.MapFile{Data: []byte(setIconSVG(cat.Code))}
	}
}

// setIconSVG draws the icon of a demo set: its code in a ring, in black like a Scryfall icon, which
// the page turns light.
func setIconSVG(code string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
  <circle cx="32" cy="32" r="28" fill="none" stroke="#000" stroke-width="6"/>
  <text x="32" y="40" text-anchor="middle" font-family="sans-serif" font-size="22" font-weight="700" fill="#000">%s</text>
</svg>
`, html.EscapeString(code))
}
