package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/play/playtest"
)

var packsLeftPattern = regexp.MustCompile(`(\d+) boosters?`)

func TestDefaultSavePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		demo bool
		want string
	}{
		{"a real game", false, filepath.Join("cfg", "crackapack", "save.json")},
		{"the demo has a save of its own", true, filepath.Join("cfg", "crackapack", "save-demo.json")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := defaultSavePath("cfg", tc.demo); got != tc.want {
				t.Errorf("defaultSavePath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunPlay_aRestartKeepsTheGameAndItsSeed(t *testing.T) {
	t.Parallel()
	a := demoListArgs(t)
	var out1, out2 bytes.Buffer

	a.stdout = &out1
	base, stop := startPlay(t, a)
	firstOfAll := openFirstBooster(t, base)
	second := openFirstBooster(t, base)
	stop()

	// The player starts again, and even asks for another seed: the saved one wins.
	a.seed, a.stdout = 7, &out2
	base, stop = startPlay(t, a)
	third := openFirstBooster(t, base)
	home, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	homeBody := readAll(t, home)
	stop()

	// Three openings of the same set came out of one run of boosters: none repeats the one before it.
	if third == second || third == firstOfAll {
		t.Errorf("the booster after the restart repeats an earlier one:\n%s", third)
	}
	if got := packsLeftPattern.FindStringSubmatch(homeBody); got == nil || got[1] != "2" {
		t.Errorf("home after three openings = %v boosters left, want 2 of the first set's 5:\n%s", got, homeBody)
	}
	if !strings.Contains(out2.String(), "seed 42") {
		t.Errorf("startup line %q does not tell the saved seed 42", out2.String())
	}
	if !strings.Contains(out1.String(), a.savePath) {
		t.Errorf("startup line %q does not tell where the save is", out1.String())
	}
}

func TestRunPlay_aCorruptSaveStopsTheStartAndIsLeftAlone(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	if err := os.WriteFile(a.savePath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write the save: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	a.listener = ln
	a.stdout, a.stderr = &bytes.Buffer{}, &bytes.Buffer{}

	err = runPlay(context.Background(), a)

	if err == nil || !strings.Contains(err.Error(), a.savePath) {
		t.Fatalf("runPlay error = %v, want one naming %s", err, a.savePath)
	}
	if got, _ := os.ReadFile(a.savePath); string(got) != "{" {
		t.Errorf("the save now holds %q, want it left as it was", got)
	}
}

func TestRunPlay_aSaveThatCannotBeWrittenDoesNotStopTheServer(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	store := &playtest.FlakyStore{}
	store.Fail(errors.New("disk full"))
	a.store = store
	base := servePlay(t, a)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.PostForm(base+"/open", url.Values{"set": {"DEM"}})
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	_ = readAll(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /open = %d, want 303", resp.StatusCode)
	}

	page, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	if body := readAll(t, page); !strings.Contains(body, "could not be saved") {
		t.Errorf("home does not tell the save failed:\n%s", body)
	}
}

// lineSignal is a writer that tells, once, when a line containing marker was written to it.
type lineSignal struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	marker string
	seen   chan struct{}
	once   sync.Once
}

func newLineSignal(marker string) *lineSignal {
	return &lineSignal{marker: marker, seen: make(chan struct{})}
}

func (w *lineSignal) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if strings.Contains(w.buf.String(), w.marker) {
		w.once.Do(func() { close(w.seen) })
	}
	return n, err
}

func (w *lineSignal) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// TestCmdPlay_savesTheGameWhereSaveSays runs the command as the player does, so that the flag, the
// store and the startup line are wired together.
func TestCmdPlay_savesTheGameWhereSaveSays(t *testing.T) {
	t.Parallel()
	savePath := filepath.Join(t.TempDir(), "mine", "save.json")
	stdout := newLineSignal("open http://")
	cmd := newPlayCmd(stdout, io.Discard)
	cmd.SetArgs([]string{"--demo", "--save", savePath, "--addr", "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	t.Cleanup(cancel)

	select {
	case <-stdout.seen:
	case err := <-done:
		t.Fatalf("play stopped before serving: %v\n%s", err, stdout.String())
	case <-time.After(serveTimeout):
		t.Fatalf("play did not start:\n%s", stdout.String())
	}
	line := stdout.String()
	if !strings.Contains(line, "save file "+savePath) {
		t.Errorf("startup output does not tell the save file %s:\n%s", savePath, line)
	}
	_, after, _ := strings.Cut(line, "open http://")
	addr, _, _ := strings.Cut(after, " ")
	resp, err := http.PostForm("http://"+addr+"/open", url.Values{"set": {"DEM"}})
	if err != nil {
		t.Fatalf("POST /open: %v", err)
	}
	_ = readAll(t, resp)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("play: %v", err)
	}

	saved, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("read the save: %v", err)
	}
	// The demo plays in stages, so DEM, its first stage, has the ten welcome boosters.
	var snap struct {
		Welcome map[string]int `json:"welcome"`
	}
	if err := json.Unmarshal(saved, &snap); err != nil || snap.Welcome["DEM"] != 9 {
		t.Errorf("the save does not show a welcome booster of DEM opened (%v):\n%s", err, saved)
	}
}

func TestRunPlay_refusesToRunWithoutAStore(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	a.store = nil
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	a.listener = ln
	a.stdout, a.stderr = io.Discard, io.Discard

	if err := runPlay(context.Background(), a); err == nil {
		t.Fatal("runPlay without a store returned nil error")
	}
}

func TestRunPlay_aSaveThatDoesNotFitTheGameNamesItsPath(t *testing.T) {
	t.Parallel()
	a := demoPlayArgs(t)
	if err := os.WriteFile(a.savePath, []byte(`{"version": 1, "welcome": {"DEM": -1}}`), 0o600); err != nil {
		t.Fatalf("write the save: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	a.listener = ln
	a.stdout, a.stderr = io.Discard, io.Discard

	err = runPlay(context.Background(), a)

	if err == nil || !strings.Contains(err.Error(), a.savePath) {
		t.Errorf("runPlay error = %v, want one naming %s", err, a.savePath)
	}
}
