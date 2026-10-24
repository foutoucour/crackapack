package images

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const cacheDirPerm = 0o755

// supportedTypes are the picture types the cache stores, in the order it looks for them.
func supportedTypes() []string {
	return []string{"image/jpeg", "image/png", "image/svg+xml"}
}

// Cached is a Source that keeps each picture it gets from src in a directory, so that a card is
// downloaded once and the game then works offline. It is safe for concurrent use.
type Cached struct {
	src Source
	dir string
	// valid says whether a key is safe for a URL and a file name; a bad one never reaches src.
	valid func(string) bool
	// mu serialises the misses, so that concurrent requests for one card download it once.
	mu sync.Mutex
}

// NewCached returns a Cached that stores pictures in dir, created on the first miss.
func NewCached(src Source, dir string) *Cached {
	return &Cached{src: src, dir: dir, valid: ValidID}
}

// NewCachedIcons returns a Cached for set icons, whose keys are icon codes, see [ValidIconCode].
func NewCachedIcons(src Source, dir string) *Cached {
	return &Cached{src: src, dir: dir, valid: ValidIconCode}
}

// Image implements Source. A failure of src is returned and not cached, so the next call retries.
func (c *Cached) Image(ctx context.Context, scryfallID string) (Image, error) {
	if !c.valid(scryfallID) {
		return Image{}, fmt.Errorf("%w: %q", ErrInvalidID, scryfallID)
	}
	if img, ok := c.read(scryfallID); ok {
		return img, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Another request may have stored it while this one waited.
	if img, ok := c.read(scryfallID); ok {
		return img, nil
	}
	img, err := c.src.Image(ctx, scryfallID)
	if err != nil {
		return Image{}, err
	}
	if err := c.write(scryfallID, img); err != nil {
		return Image{}, err
	}
	return img, nil
}

func (c *Cached) read(id string) (Image, bool) {
	for _, contentType := range supportedTypes() {
		ext, _ := extensionFor(contentType)
		data, err := os.ReadFile(filepath.Join(c.dir, id+ext))
		if err == nil {
			return Image{ContentType: contentType, Data: data}, true
		}
	}
	return Image{}, false
}

// write stores img under a temporary name and renames it, so a reader never sees half a file.
func (c *Cached) write(id string, img Image) error {
	ext, ok := extensionFor(img.ContentType)
	if !ok {
		return fmt.Errorf("cache image %s: content type %q is not a supported picture", id, img.ContentType)
	}
	if err := os.MkdirAll(c.dir, cacheDirPerm); err != nil {
		return fmt.Errorf("cache image %s: %w", id, err)
	}
	tmp, err := os.CreateTemp(c.dir, id+".*.tmp")
	if err != nil {
		return fmt.Errorf("cache image %s: %w", id, err)
	}
	_, writeErr := tmp.Write(img.Data)
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("cache image %s: write: %w", id, err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(c.dir, id+ext)); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("cache image %s: %w", id, err)
	}
	return nil
}

// extensionFor returns the file extension of a supported picture type.
func extensionFor(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/svg+xml":
		return ".svg", true
	default:
		return "", false
	}
}
