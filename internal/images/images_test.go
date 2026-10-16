package images_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/images"
)

const (
	testID        = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	otherID       = "ffffffff-eeee-4ddd-8ccc-bbbbbbbbbbbb"
	testUserAgent = "crackapack-test/1.0"
	testInterval  = 100 * time.Millisecond
)

func jpegBytes() []byte { return []byte{0xff, 0xd8, 0xff, 0xe0, 'f', 'a', 'k', 'e'} }

// sleeps records every pause the Scryfall source asks for, without waiting.
type sleeps struct {
	mu sync.Mutex
	d  []time.Duration
}

func (s *sleeps) sleep(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d = append(s.d, d)
}

func (s *sleeps) all() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.d...)
}

func newScryfall(t *testing.T, handler http.HandlerFunc) (*images.Scryfall, *sleeps) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	rec := &sleeps{}
	return images.NewScryfall(images.ScryfallConfig{
		Client:    srv.Client(),
		BaseURL:   srv.URL,
		UserAgent: testUserAgent,
		Interval:  testInterval,
		Sleep:     rec.sleep,
	}), rec
}

func serveJPEG(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(jpegBytes())
}

func TestScryfall_asksForTheNormalImageIdentifyingItself(t *testing.T) {
	t.Parallel()
	var got *http.Request
	src, _ := newScryfall(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		serveJPEG(w, r)
	})

	img, err := src.Image(context.Background(), testID)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}

	if got.URL.Path != "/cards/"+testID || got.URL.Query().Get("format") != "image" || got.URL.Query().Get("version") != "normal" {
		t.Errorf("requested %q, want /cards/%s?format=image&version=normal", got.URL.String(), testID)
	}
	if got.Header.Get("User-Agent") != testUserAgent {
		t.Errorf("User-Agent = %q, want %q", got.Header.Get("User-Agent"), testUserAgent)
	}
	if got.Header.Get("Accept") == "" {
		t.Error("Accept header missing: the Scryfall API requires one")
	}
	if img.ContentType != "image/jpeg" || string(img.Data) != string(jpegBytes()) {
		t.Errorf("image = %q %v, want image/jpeg and the served bytes", img.ContentType, img.Data)
	}
}

func TestScryfall_followsTheRedirectToTheImageHost(t *testing.T) {
	t.Parallel()
	host := httptest.NewServer(http.HandlerFunc(serveJPEG))
	t.Cleanup(host.Close)
	src, _ := newScryfall(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, host.URL+"/front/a.jpg", http.StatusFound)
	})

	img, err := src.Image(context.Background(), testID)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	if string(img.Data) != string(jpegBytes()) {
		t.Errorf("data = %v, want the image host's bytes", img.Data)
	}
}

func TestScryfall_refusesABadIDWithoutAnyRequest(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	src, _ := newScryfall(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })

	for _, id := range []string{"", "../../etc/passwd", "not-a-uuid", testID + "/extra", "0A1B2C3D-4E5F-4A6B-8C7D-9E0F1A2B3C4D"} {
		if _, err := src.Image(context.Background(), id); !errors.Is(err, images.ErrInvalidID) {
			t.Errorf("Image(%q) error = %v, want ErrInvalidID", id, err)
		}
	}
	if calls.Load() != 0 {
		t.Errorf("%d requests reached the server, want none", calls.Load())
	}
}

func TestScryfall_failuresAreTold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		handler      http.HandlerFunc
		wantNotFound bool
	}{
		{"unknown card", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }, true},
		{"upstream error", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }, false},
		{"not an image", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>"))
		}, false},
		{"too large", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(make([]byte, images.MaxImageBytes+1))
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src, _ := newScryfall(t, tc.handler)

			_, err := src.Image(context.Background(), testID)

			if err == nil {
				t.Fatal("Image returned nil error")
			}
			if got := errors.Is(err, images.ErrNotFound); got != tc.wantNotFound {
				t.Errorf("errors.Is(err, ErrNotFound) = %v, want %v (err: %v)", got, tc.wantNotFound, err)
			}
		})
	}
}

func TestScryfall_pausesAfterEveryRequestSoTheyAreSpacedOut(t *testing.T) {
	t.Parallel()
	src, rec := newScryfall(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cards/"+otherID {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		serveJPEG(w, r)
	})

	_, _ = src.Image(context.Background(), testID)
	_, _ = src.Image(context.Background(), otherID) // a failure paces the next request too

	got := rec.all()
	if len(got) != 2 || got[0] != testInterval || got[1] != testInterval {
		t.Errorf("pauses = %v, want two of %v", got, testInterval)
	}
}

// countingSource is a Source that records its calls.
type countingSource struct {
	calls atomic.Int32
	img   images.Image
	err   error
}

func (s *countingSource) Image(context.Context, string) (images.Image, error) {
	s.calls.Add(1)
	return s.img, s.err
}

func TestCached_fetchesOnceThenServesFromDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := &countingSource{img: images.Image{ContentType: "image/jpeg", Data: jpegBytes()}}
	cache := images.NewCached(src, dir)

	first, err := cache.Image(context.Background(), testID)
	if err != nil {
		t.Fatalf("first Image: %v", err)
	}
	second, err := cache.Image(context.Background(), testID)
	if err != nil {
		t.Fatalf("second Image: %v", err)
	}

	if src.calls.Load() != 1 {
		t.Errorf("source called %d times, want 1", src.calls.Load())
	}
	for _, img := range []images.Image{first, second} {
		if img.ContentType != "image/jpeg" || string(img.Data) != string(jpegBytes()) {
			t.Errorf("image = %q %v, want image/jpeg and the source bytes", img.ContentType, img.Data)
		}
	}
}

func TestCached_survivesARestartBecauseItReadsTheDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := &countingSource{img: images.Image{ContentType: "image/png", Data: []byte("png")}}
	if _, err := images.NewCached(src, dir).Image(context.Background(), testID); err != nil {
		t.Fatalf("Image: %v", err)
	}

	fresh := &countingSource{}
	img, err := images.NewCached(fresh, dir).Image(context.Background(), testID)

	if err != nil {
		t.Fatalf("Image after restart: %v", err)
	}
	if fresh.calls.Load() != 0 || img.ContentType != "image/png" || string(img.Data) != "png" {
		t.Errorf("after restart: %d source calls, image %q %q; want 0, image/png, png", fresh.calls.Load(), img.ContentType, img.Data)
	}
}

func TestCached_aFailureIsNotCachedSoTheNextCallRetries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := &countingSource{err: errors.New("offline")}
	cache := images.NewCached(src, dir)

	if _, err := cache.Image(context.Background(), testID); err == nil {
		t.Fatal("Image returned nil error for a failing source")
	}
	src.err, src.img = nil, images.Image{ContentType: "image/jpeg", Data: jpegBytes()}
	if _, err := cache.Image(context.Background(), testID); err != nil {
		t.Fatalf("Image after recovery: %v", err)
	}

	if src.calls.Load() != 2 {
		t.Errorf("source called %d times, want 2", src.calls.Load())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("cache holds %d files, want 1 and no temporary leftovers", len(entries))
	}
}

func TestCached_concurrentRequestsForOneCardFetchItOnce(t *testing.T) {
	t.Parallel()
	src := &countingSource{img: images.Image{ContentType: "image/jpeg", Data: jpegBytes()}}
	cache := images.NewCached(src, t.TempDir())

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.Image(context.Background(), testID); err != nil {
				t.Errorf("Image: %v", err)
			}
		}()
	}
	wg.Wait()

	if src.calls.Load() != 1 {
		t.Errorf("source called %d times, want 1", src.calls.Load())
	}
}

func TestCached_refusesABadIDWithoutTouchingTheDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "never-created")
	src := &countingSource{}

	_, err := images.NewCached(src, dir).Image(context.Background(), "../escape")

	if !errors.Is(err, images.ErrInvalidID) {
		t.Fatalf("error = %v, want ErrInvalidID", err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("the cache directory was created for a bad id (stat error: %v)", statErr)
	}
	if src.calls.Load() != 0 {
		t.Errorf("source called %d times, want 0", src.calls.Load())
	}
}
