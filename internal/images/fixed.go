package images

import (
	"context"
	"fmt"
	"net/http"
)

// CardBackID names the picture of the back of a card. It is not a Scryfall ID: it is the key under
// which the back is served at /card-image/ and stored in the cache, shaped like an ID so that it
// passes ValidID and gives a safe file name.
const CardBackID = "cafe0bac-0000-4000-8000-00000000cafe"

// FixedConfig holds the collaborators of a Fixed source.
type FixedConfig struct {
	Client *http.Client
	// ID is the only ID the source answers to.
	ID string
	// URL is where the picture is downloaded from.
	URL string
	// UserAgent identifies the app to the server.
	UserAgent string
}

// Fixed is a Source of one picture at a fixed URL, such as the back of a card. It asks the URL
// only when it is asked for its own ID, and it is meant to sit under a Cached so that the download
// happens once. It is safe for concurrent use.
type Fixed struct {
	cfg FixedConfig
}

// NewFixed returns a Fixed source.
func NewFixed(cfg FixedConfig) *Fixed {
	return &Fixed{cfg: cfg}
}

// Image implements Source. It returns ErrNotFound for any ID but its own. A failed download is an
// error that is not ErrNotFound, so that a Chain does not take it for "ask the next source".
func (f *Fixed) Image(ctx context.Context, id string) (Image, error) {
	if id != f.cfg.ID {
		return Image{}, fmt.Errorf("fixed picture: %w", ErrNotFound)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.cfg.URL, nil)
	if err != nil {
		return Image{}, fmt.Errorf("fixed picture %s: build request: %w", f.cfg.URL, err)
	}
	req.Header.Set("User-Agent", f.cfg.UserAgent)
	req.Header.Set("Accept", "image/jpeg,image/png;q=0.9")
	return download(f.cfg.Client, req, "fixed picture "+f.cfg.URL)
}
