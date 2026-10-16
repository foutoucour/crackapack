package images_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/foutoucour/crackapack/internal/images"
)

func newFixed(t *testing.T, handler http.HandlerFunc) *images.Fixed {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return images.NewFixed(images.FixedConfig{
		Client:    srv.Client(),
		ID:        images.CardBackID,
		URL:       srv.URL + "/w/images/back.jpg",
		UserAgent: testUserAgent,
	})
}

func TestFixed_servesItsOnePictureUnderItsID(t *testing.T) {
	t.Parallel()
	var got *http.Request
	src := newFixed(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		serveJPEG(w, r)
	})

	img, err := src.Image(context.Background(), images.CardBackID)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}

	if got.URL.Path != "/w/images/back.jpg" || got.Header.Get("User-Agent") != testUserAgent {
		t.Errorf("requested %q as %q, want /w/images/back.jpg identifying the app", got.URL.Path, got.Header.Get("User-Agent"))
	}
	if img.ContentType != "image/jpeg" || string(img.Data) != string(jpegBytes()) {
		t.Errorf("image = %q %v, want image/jpeg and the served bytes", img.ContentType, img.Data)
	}
}

func TestFixed_knowsNoOtherIDAndMakesNoRequestForOne(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	src := newFixed(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })

	_, err := src.Image(context.Background(), otherID)

	if !errors.Is(err, images.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if calls.Load() != 0 {
		t.Errorf("%d requests reached the server, want none", calls.Load())
	}
}

func TestFixed_failuresAreTold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"missing upstream", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }},
		{"upstream error", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }},
		{"not an image", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>"))
		}},
		{"too large", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(make([]byte, images.MaxImageBytes+1))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := newFixed(t, tc.handler)

			_, err := src.Image(context.Background(), images.CardBackID)

			if err == nil {
				t.Fatal("Image returned nil error")
			}
			// A fixed picture that fails is a failure of the source, not an unknown card: the
			// chain must not mistake it for "ask the next source".
			if errors.Is(err, images.ErrNotFound) {
				t.Errorf("error = %v, want one that is not ErrNotFound", err)
			}
		})
	}
}

func TestCardBackID_isAValidIDSoItCanBeServedAndCachedLikeAnyPicture(t *testing.T) {
	t.Parallel()
	if !images.ValidID(images.CardBackID) {
		t.Errorf("CardBackID %q is not a valid id", images.CardBackID)
	}
}

// named is a Source that answers for one id only.
type named struct {
	id   string
	img  images.Image
	hits atomic.Int32
}

func (n *named) Image(_ context.Context, id string) (images.Image, error) {
	n.hits.Add(1)
	if id != n.id {
		return images.Image{}, images.ErrNotFound
	}
	return n.img, nil
}

func TestChain_asksEachSourceInOrderAndStopsAtTheFirstThatKnowsTheID(t *testing.T) {
	t.Parallel()
	first := &named{id: testID, img: images.Image{ContentType: "image/jpeg", Data: []byte("first")}}
	second := &named{id: otherID, img: images.Image{ContentType: "image/png", Data: []byte("second")}}
	chain := images.Chain(first, second)

	a, errA := chain.Image(context.Background(), testID)
	b, errB := chain.Image(context.Background(), otherID)

	if errA != nil || errB != nil {
		t.Fatalf("errors = %v, %v", errA, errB)
	}
	if string(a.Data) != "first" || string(b.Data) != "second" {
		t.Errorf("got %q and %q, want first and second", a.Data, b.Data)
	}
	if second.hits.Load() != 1 {
		t.Errorf("the second source was asked %d times, want 1 (only for the id the first did not know)", second.hits.Load())
	}
}

func TestChain_isNotFoundWhenNoSourceKnowsTheID(t *testing.T) {
	t.Parallel()
	chain := images.Chain(&named{id: testID}, &named{id: otherID})

	_, err := chain.Image(context.Background(), "11111111-1111-4111-8111-111111111111")

	if !errors.Is(err, images.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestChain_aRealFailureStopsTheChainInsteadOfBeingHidden(t *testing.T) {
	t.Parallel()
	boom := errors.New("offline")
	late := &named{id: testID}
	chain := images.Chain(&countingSource{err: boom}, late)

	_, err := chain.Image(context.Background(), testID)

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the first source's failure", err)
	}
	if late.hits.Load() != 0 {
		t.Errorf("a later source was asked after a real failure")
	}
}
