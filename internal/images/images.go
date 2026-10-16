// Package images fetches card pictures from Scryfall and keeps them in a local cache, see
// docs/core-features.md#legal-and-data. The domain never sees it: a client asks for a picture by
// the card's Scryfall ID and shows what it gets back, unchanged.
package images

import (
	"context"
	"errors"
	"regexp"
)

// MaxImageBytes bounds the size of one picture. A Scryfall "normal" image is about 100 KB.
const MaxImageBytes = 5 << 20

var (
	// ErrInvalidID is returned for an ID that is not a lower-case UUID. It is checked before any
	// request or file access, since the ID ends up in a URL and in a file name.
	ErrInvalidID = errors.New("images: invalid Scryfall id")
	// ErrNotFound is returned when the source has no picture for a valid ID.
	ErrNotFound = errors.New("images: no picture for this card")
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ValidID reports whether id is a Scryfall ID: a lower-case UUID.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Image is one picture and its media type.
type Image struct {
	ContentType string
	Data        []byte
}

// Source returns the picture of the card with the given Scryfall ID.
type Source interface {
	Image(ctx context.Context, scryfallID string) (Image, error)
}
