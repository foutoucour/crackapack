package web

import (
	"fmt"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/images"
	"github.com/foutoucour/crackapack/internal/play"
)

// iconView is the icon of a set, drawn in the colour of a rarity. The Code is empty when the set
// has no icon, and the page then shows none.
type iconView struct {
	Code   string
	Rarity string
}

// Class names the CSS rule that masks the icon with its picture, see setIconStylesheet.
func (i iconView) Class() string { return iconClass(i.Code) }

func iconClass(code string) string { return "set-icon-" + strings.ToLower(code) }

// newIconView returns the icon of the set with the given code in the colour of rarity. A code that
// is not safe for a class name and a URL gives no icon.
func newIconView(code, rarity string) iconView {
	if !images.ValidIconCode(code) {
		return iconView{}
	}
	return iconView{Code: code, Rarity: rarity}
}

// rarestHeld is the rarity of the rarest card a binder holds, common when it holds nothing.
func rarestHeld(progress domain.Progress) string {
	for _, rarity := range []domain.Rarity{domain.RarityMythic, domain.RarityRare, domain.RarityUncommon} {
		if progress.ByRarity[rarity].Owned > 0 {
			return string(rarity)
		}
	}
	return string(domain.RarityCommon)
}

// setIconStylesheet has one rule for each set that has an icon, which masks an element of the
// set-icon class with the set's picture. The picture comes from this origin, and a style attribute
// cannot carry it: the content security policy refuses inline styles.
func setIconStylesheet(packs []play.Pack) string {
	var css strings.Builder
	seen := make(map[string]bool, len(packs))
	for _, p := range packs {
		if !images.ValidIconCode(p.IconCode) || seen[p.IconCode] {
			continue
		}
		seen[p.IconCode] = true
		fmt.Fprintf(&css, ".%s { mask-image: url(\"/set-icon/%s\"); }\n", iconClass(p.IconCode), p.IconCode)
	}
	return css.String()
}
