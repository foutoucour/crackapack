package images

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// DefaultInterval is the pause after each Scryfall request. Scryfall asks for 50 to 100 ms between
// requests, see docs/core-features.md#legal-and-data.
const DefaultInterval = 100 * time.Millisecond

// ScryfallConfig holds the collaborators of a Scryfall source.
type ScryfallConfig struct {
	Client *http.Client
	// BaseURL is the API root, https://api.scryfall.com in production.
	BaseURL string
	// UserAgent identifies the app: the Scryfall API requires one.
	UserAgent string
	// Interval is the pause after each request.
	Interval time.Duration
	// Sleep waits for d. It is injected so that tests need not wait.
	Sleep func(d time.Duration)
}

// Scryfall is a Source that downloads the "normal" picture of a card from the Scryfall API. It
// makes one request at a time and pauses after each, so a burst of calls stays within the rate
// limit. It is safe for concurrent use.
type Scryfall struct {
	cfg ScryfallConfig
	mu  sync.Mutex
}

// NewScryfall returns a Scryfall source.
func NewScryfall(cfg ScryfallConfig) *Scryfall {
	return &Scryfall{cfg: cfg}
}

// Image implements Source. It returns ErrInvalidID, ErrNotFound, or an error that says what the
// server answered.
func (s *Scryfall) Image(ctx context.Context, scryfallID string) (Image, error) {
	if !ValidID(scryfallID) {
		return Image{}, fmt.Errorf("%w: %q", ErrInvalidID, scryfallID)
	}

	endpoint := s.cfg.BaseURL + "/cards/" + scryfallID + "?" + url.Values{"format": {"image"}, "version": {"normal"}}.Encode()
	return s.fetch(ctx, endpoint, "image/jpeg,image/png;q=0.9,*/*;q=0.5", "scryfall image "+scryfallID)
}

// fetch downloads the picture at endpoint, one request at a time and with a pause after it. what
// names the picture in errors. A 404 is ErrNotFound.
func (s *Scryfall) fetch(ctx context.Context, endpoint, accept, what string) (Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.cfg.Sleep(s.cfg.Interval)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Image{}, fmt.Errorf("%s: build request: %w", what, err)
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	req.Header.Set("Accept", accept)

	img, err := download(s.cfg.Client, req, what)
	var status *statusError
	if errors.As(err, &status) && status.code == http.StatusNotFound {
		return Image{}, fmt.Errorf("%s: %w", what, ErrNotFound)
	}
	return img, err
}
