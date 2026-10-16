package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/images"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const serveTimeout = 5 * time.Second

// servePlay runs runPlay on a free port and returns its base URL. The server stops, and its
// error is checked, when the test ends.
func servePlay(t *testing.T, a playArgs) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	a.listener = ln
	a.stdout, a.stderr = io.Discard, io.Discard

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runPlay(ctx, a) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("runPlay: %v", err)
			}
		case <-time.After(serveTimeout):
			t.Error("runPlay did not stop after its context was cancelled")
		}
	})
	return "http://" + ln.Addr().String()
}

// fakeBack is the card back source in place of the download: it answers to the back's ID only.
type fakeBack struct{ data []byte }

func (f fakeBack) Image(_ context.Context, id string) (images.Image, error) {
	if id != images.CardBackID {
		return images.Image{}, images.ErrNotFound
	}
	return images.Image{ContentType: "image/jpeg", Data: f.data}, nil
}

// demoPlayArgs is the demo session, with the default configuration and a clock that does not move.
func demoPlayArgs(t *testing.T) playArgs {
	t.Helper()
	ds := demo.Build()
	cfg, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("load the default configuration: %v", err)
	}
	return playArgs{
		catFS: ds.Files, setCodes: ds.SetCodes, suppCodes: ds.SupplementaryCodes,
		pricesFS: ds.Files, pricesName: "prices.json",
		cfg: cfg, now: playtest.NewClock().Now,
		images: ds.Images, cardBack: fakeBack{data: []byte("back-jpeg")}, seed: 42,
	}
}

func TestRunPlay_servesTheCardBackBesideTheCardPictures(t *testing.T) {
	t.Parallel()
	base := servePlay(t, demoPlayArgs(t))

	resp, err := http.Get(base + "/card-image/" + images.CardBackID)
	if err != nil {
		t.Fatalf("GET the card back: %v", err)
	}
	body := readAll(t, resp)

	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" || body != "back-jpeg" {
		t.Errorf("card back = %d %q %q, want 200 image/jpeg back-jpeg", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
}

func TestRunPlay_showsDemoCardPicturesFromItsOwnOrigin(t *testing.T) {
	t.Parallel()
	base := servePlay(t, demoPlayArgs(t))
	page := openFirstBooster(t, base)

	_, after, found := strings.Cut(page, `<img src="`)
	if !found {
		t.Fatalf("the booster page shows no card picture:\n%s", page)
	}
	src, _, _ := strings.Cut(after, `"`)
	if !strings.HasPrefix(src, "/card-image/") {
		t.Fatalf("picture src = %q, want a path on this server", src)
	}

	resp, err := http.Get(base + src)
	if err != nil {
		t.Fatalf("GET %s: %v", src, err)
	}
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/svg+xml" || !strings.Contains(body, "<svg") {
		t.Errorf("GET %s = %d %q, want 200 image/svg+xml with an SVG:\n%s", src, resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
}

func TestRunPlay_servesTheDemoSetsAndOpensABooster(t *testing.T) {
	t.Parallel()
	base := servePlay(t, demoPlayArgs(t))

	home, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	homeBody := readAll(t, home)
	if home.StatusCode != http.StatusOK || !strings.Contains(homeBody, `value="DEM"`) {
		t.Fatalf("home = %d, want 200 listing the demo sets:\n%s", home.StatusCode, homeBody)
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.PostForm(base+"/open", url.Values{"set": {"DEM"}})
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	_ = readAll(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /open = %d, want 303", resp.StatusCode)
	}
}

func TestRunPlay_sameSeedOpensTheSameBooster(t *testing.T) {
	t.Parallel()
	first := openFirstBooster(t, servePlay(t, demoPlayArgs(t)))
	second := openFirstBooster(t, servePlay(t, demoPlayArgs(t)))

	if first != second {
		t.Errorf("same seed gave different boosters:\n%s\n---\n%s", first, second)
	}
}

func TestRunPlay_rejectsAnUnknownSetCode(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	a.setCodes = []string{"NOPE"}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	a.listener = ln
	a.stdout, a.stderr = io.Discard, io.Discard

	err = runPlay(context.Background(), a)

	if err == nil || !strings.Contains(err.Error(), "NOPE.json") {
		t.Fatalf("runPlay error = %v, want one naming NOPE.json", err)
	}
}

func TestCmdPlay_withoutDemoNeedsTheCatalogueFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	err := run([]string{"play"}, &stdout, &stderr)

	if err == nil {
		t.Fatal("play with no flag returned nil error")
	}
	for _, want := range []string{"--catalogue-dir", "--sets"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

func openFirstBooster(t *testing.T, base string) string {
	t.Helper()
	client := &http.Client{}
	resp, err := client.PostForm(base+"/open", url.Values{"set": {"DEM"}})
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	// The client follows the redirect, so this is the pack page.
	return readAll(t, resp)
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}
