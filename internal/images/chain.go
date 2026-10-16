package images

import (
	"context"
	"errors"
	"fmt"
)

type chain []Source

// Chain returns a Source that asks each of sources in turn and answers with the first that knows
// the ID. A source that returns ErrNotFound passes the ID on; any other error ends the chain, so
// that a failure is not hidden behind a later source.
func Chain(sources ...Source) Source {
	return chain(sources)
}

// Image implements Source.
func (c chain) Image(ctx context.Context, id string) (Image, error) {
	for _, src := range c {
		img, err := src.Image(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		return img, err
	}
	return Image{}, fmt.Errorf("no source has picture %s: %w", id, ErrNotFound)
}
