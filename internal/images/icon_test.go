package images_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/foutoucour/crackapack/internal/images"
)

const svgContentType = "image/svg+xml"

func svgBytes() []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>`)
}

func serveSVG(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", svgContentType)
	_, _ = w.Write(svgBytes())
}

func newScryfallIcons(t *testing.T, handler http.HandlerFunc) *images.ScryfallIcons {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return images.NewScryfallIcons(images.ScryfallConfig{
		Client:    srv.Client(),
		BaseURL:   srv.URL,
		UserAgent: testUserAgent,
		Interval:  testInterval,
		Sleep:     (&sleeps{}).sleep,
	})
}

func TestScryfallIcons_asksForTheSVGOfTheLowerCasedCodeIdentifyingItself(t *testing.T) {
	t.Parallel()
	var got *http.Request
	icons := newScryfallIcons(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		serveSVG(w, r)
	})

	img, err := icons.Image(context.Background(), "RAV")
	if err != nil {
		t.Fatalf("Image: %v", err)
	}

	if got.URL.Path != "/sets/rav.svg" {
		t.Errorf("requested %q, want /sets/rav.svg", got.URL.Path)
	}
	if got.Header.Get("User-Agent") != testUserAgent {
		t.Errorf("User-Agent = %q, want %q", got.Header.Get("User-Agent"), testUserAgent)
	}
	if img.ContentType != svgContentType || string(img.Data) != string(svgBytes()) {
		t.Errorf("image = %q %q, want %s and the served bytes", img.ContentType, img.Data, svgContentType)
	}
}

func TestScryfallIcons_aSetItDoesNotKnowIsNotFound(t *testing.T) {
	t.Parallel()
	icons := newScryfallIcons(t, func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })

	_, err := icons.Image(context.Background(), "ZZZZ")

	if !errors.Is(err, images.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestScryfallIcons_refusesABadCodeWithoutAnyRequest(t *testing.T) {
	t.Parallel()
	requests := 0
	icons := newScryfallIcons(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		serveSVG(w, r)
	})

	for _, code := range []string{"", "../escape", "r/av", "RAV.svg", "toolongcode1"} {
		if _, err := icons.Image(context.Background(), code); !errors.Is(err, images.ErrInvalidID) {
			t.Errorf("Image(%q) error = %v, want ErrInvalidID", code, err)
		}
	}
	if requests != 0 {
		t.Errorf("%d requests for bad codes, want 0", requests)
	}
}

func TestValidIconCode_acceptsTheCodesOfRealSets(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"RAV", "10E", "5DN", "PRAETORS", "m10"} {
		if !images.ValidIconCode(code) {
			t.Errorf("ValidIconCode(%q) = false, want true", code)
		}
	}
}

func TestCachedIcons_fetchesOnceThenServesTheSVGFromDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := &countingSource{img: images.Image{ContentType: svgContentType, Data: svgBytes()}}
	cache := images.NewCachedIcons(src, dir)

	first, err := cache.Image(context.Background(), "RAV")
	if err != nil {
		t.Fatalf("first Image: %v", err)
	}
	second, err := images.NewCachedIcons(&countingSource{}, dir).Image(context.Background(), "RAV")
	if err != nil {
		t.Fatalf("Image after restart: %v", err)
	}

	if src.calls.Load() != 1 {
		t.Errorf("source called %d times, want 1", src.calls.Load())
	}
	for _, img := range []images.Image{first, second} {
		if img.ContentType != svgContentType || string(img.Data) != string(svgBytes()) {
			t.Errorf("image = %q %q, want %s and the source bytes", img.ContentType, img.Data, svgContentType)
		}
	}
}

func TestCachedIcons_refusesABadCodeWithoutTouchingTheDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "never-created")
	src := &countingSource{}

	_, err := images.NewCachedIcons(src, dir).Image(context.Background(), "../escape")

	if !errors.Is(err, images.ErrInvalidID) {
		t.Fatalf("error = %v, want ErrInvalidID", err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("the cache directory was created for a bad code (stat error: %v)", statErr)
	}
	if src.calls.Load() != 0 {
		t.Errorf("source called %d times, want 0", src.calls.Load())
	}
}
