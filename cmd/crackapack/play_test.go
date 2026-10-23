package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/images"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

const serveTimeout = 5 * time.Second

// servePlay runs runPlay on a free port and returns its base URL. The server stops, and its
// error is checked, when the test ends.
func servePlay(t *testing.T, a playArgs) string {
	t.Helper()
	base, stop := startPlay(t, a)
	t.Cleanup(stop)
	return base
}

// startPlay is servePlay for a test that stops the server itself, to start another on the same
// save. stop may be called more than once, and nothing is written to a.stdout once it has returned.
func startPlay(t *testing.T, a playArgs) (base string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	a.listener = ln
	if a.stdout == nil {
		a.stdout = io.Discard
	}
	a.stderr = io.Discard

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runPlay(ctx, a) }()
	var once sync.Once
	stop = func() {
		once.Do(func() {
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
	}
	return "http://" + ln.Addr().String(), stop
}

// fakeBack is the card back source in place of the download: it answers to the back's ID only.
type fakeBack struct{ data []byte }

func (f fakeBack) Image(_ context.Context, id string) (images.Image, error) {
	if id != images.CardBackID {
		return images.Image{}, images.ErrNotFound
	}
	return images.Image{ContentType: "image/jpeg", Data: f.data}, nil
}

// demoPlayArgs is the demo session, in stages, with the demo's configuration and a clock that does
// not move.
func demoPlayArgs(t *testing.T) playArgs {
	t.Helper()
	ds := demo.Build()
	cfg, err := config.Load(strings.NewReader(ds.PlayConfigYAML))
	if err != nil {
		t.Fatalf("load the demo configuration: %v", err)
	}
	savePath := filepath.Join(t.TempDir(), "save.json")
	return playArgs{
		catFS: ds.Files, scope: true,
		pricesFS: ds.Files, pricesName: "prices.json",
		cfg: cfg, now: playtest.NewClock().Now,
		images: ds.Images, cardBack: fakeBack{data: []byte("back-jpeg")}, seed: 42,
		savePath: savePath, store: play.NewFileStore(savePath),
	}
}

// demoListArgs is demoPlayArgs on the demo's list of sets, every one open, as --sets plays them.
func demoListArgs(t *testing.T) playArgs {
	t.Helper()
	ds := demo.Build()
	a := demoPlayArgs(t)
	a.scope, a.setCodes, a.suppCodes = false, ds.SetCodes, ds.SupplementaryCodes
	return a
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
	a := demoListArgs(t)
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

func TestCmdPlay_withoutDemoNeedsTheCatalogueDirectory(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	err := run([]string{"play"}, &stdout, &stderr)

	if err == nil {
		t.Fatal("play with no flag returned nil error")
	}
	if !strings.Contains(err.Error(), "--catalogue-dir") {
		t.Errorf("error %q does not name --catalogue-dir", err)
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
