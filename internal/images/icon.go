package images

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var iconCodePattern = regexp.MustCompile(`^[A-Za-z0-9]{2,8}$`)

// ValidIconCode reports whether code is safe as the code of a set icon, such as RAV or 10E.
func ValidIconCode(code string) bool { return iconCodePattern.MatchString(code) }

// ScryfallIcons is a Source of set icons: the key is the code of the icon, and the picture is the
// SVG that Scryfall draws for the set. It shares the pacing of [Scryfall] and is safe for
// concurrent use. Pass a ScryfallConfig whose BaseURL is the icon host, https://svgs.scryfall.io.
type ScryfallIcons struct {
	scryfall *Scryfall
}

// NewScryfallIcons returns a ScryfallIcons source.
func NewScryfallIcons(cfg ScryfallConfig) *ScryfallIcons {
	return &ScryfallIcons{scryfall: NewScryfall(cfg)}
}

// Image implements Source, with the icon code as the key. It returns ErrInvalidID, ErrNotFound for
// a set Scryfall does not draw, or an error that says what the server answered.
func (s *ScryfallIcons) Image(ctx context.Context, code string) (Image, error) {
	if !ValidIconCode(code) {
		return Image{}, fmt.Errorf("%w: %q", ErrInvalidID, code)
	}
	endpoint := s.scryfall.cfg.BaseURL + "/sets/" + strings.ToLower(code) + ".svg"
	return s.scryfall.fetch(ctx, endpoint, "image/svg+xml", "scryfall set icon "+code)
}
