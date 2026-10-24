package main

import (
	"context"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func codesOf(cats []domain.SetCatalogue) []string {
	out := make([]string, len(cats))
	for i, c := range cats {
		out[i] = c.Code
	}
	return out
}

// scopeFiles is the demo directory, whose DEX is an expensive set from before Mirage, plus OLD, a
// cheap set from before Mirage, RICH, an expensive set from after it, and UNU, a card pool that no
// booster draws from.
func scopeFiles(t *testing.T) fstest.MapFS {
	t.Helper()
	files := maps.Clone(demo.Build().Files.(fstest.MapFS))
	old, rich, unused := playtest.TinySet("OLD", 3, 1), playtest.TinySet("RICH", 3, 1), playtest.TinySet("UNU", 3, 1)
	old.ReleaseDate, rich.ReleaseDate = "1995-06-01", "2010-01-01"
	for _, cat := range []domain.SetCatalogue{old, rich} {
		data, err := domaintest.CatalogueToJSON(cat)
		if err != nil {
			t.Fatalf("catalogue %s: %v", cat.Code, err)
		}
		files[cat.Code+".json"] = &fstest.MapFile{Data: data}
	}
	data, err := domaintest.CardPoolToJSON(unused)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	files["UNU.json"] = &fstest.MapFile{Data: data}

	prices, err := domain.LoadPriceSnapshot(files, "prices.json")
	if err != nil {
		t.Fatalf("prices: %v", err)
	}
	for id := range rich.Cards {
		prices[id] = 100_00 // $100 a card, so a booster of one is worth more than $60
	}
	if files["prices.json"].Data, err = domaintest.PriceSnapshotToJSON(prices); err != nil {
		t.Fatalf("prices: %v", err)
	}
	return files
}

func TestLoadScope_keepsTheSetsFromMirageUnder60DollarsAndThePoolsTheyDraw(t *testing.T) {
	t.Parallel()
	files := scopeFiles(t)
	prices, err := domain.LoadPriceSnapshot(files, "prices.json")
	if err != nil {
		t.Fatalf("prices: %v", err)
	}
	cfg, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	sets, pools, reference, err := loadScope(files, prices, cfg)
	if err != nil {
		t.Fatalf("loadScope: %v", err)
	}

	if got := codesOf(sets); !slices.Equal(got, []string{"DEM", "DSP"}) {
		t.Errorf("sets = %v, want DEM and DSP", got)
	}
	if got := codesOf(pools); !slices.Equal(got, []string{"DST"}) {
		t.Errorf("pools = %v, want DST, which DSP draws from, and not UNU", got)
	}
	// The energy costs stay relative to every cheap set of the directory, as for a list of sets.
	if got := codesOf(reference); !slices.Equal(got, []string{"DEM", "DSP", "OLD"}) {
		t.Errorf("reference = %v, want the cheap sets DEM, DSP and OLD", got)
	}
}

func TestLoadScope_aDirectoryWithNoSetInScopeIsAnError(t *testing.T) {
	t.Parallel()
	old := playtest.TinySet("OLD", 3, 1)
	old.ReleaseDate = "1995-06-01"
	files, err := domaintest.NewMapFS([]domain.SetCatalogue{old}, nil, domain.PriceSnapshot{})
	if err != nil {
		t.Fatalf("files: %v", err)
	}
	cfg, err := config.Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	if _, _, _, err := loadScope(files, domain.PriceSnapshot{}, cfg); err == nil || !strings.Contains(err.Error(), "no set") {
		t.Errorf("loadScope error = %v, want one saying no set is in scope", err)
	}
}

// openUntilUnlock opens boosters of the demo's open set until one unlocks the other, and returns
// the code of the set it opened. The demo's stages are small enough that a few boosters do it.
func openUntilUnlock(t *testing.T, base string) string {
	t.Helper()
	client := &http.Client{}

	resp, err := client.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	home := readAll(t, resp)
	first, lockedSet := "DEM", "DSP"
	if !strings.Contains(home, `value="DEM"`) {
		first, lockedSet = lockedSet, first
	}
	if !strings.Contains(home, `value="`+first+`"`) || strings.Contains(home, `value="`+lockedSet+`"`) {
		t.Fatalf("home page does not open one demo set and lock the other:\n%s", home)
	}

	const fewBoosters = 5
	for i := range fewBoosters {
		resp, err := client.PostForm(base+"/open", url.Values{"set": {first}})
		if err != nil {
			t.Fatalf("POST /open: %v", err)
		}
		if pack := readAll(t, resp); strings.Contains(pack, "Unlocked") {
			t.Logf("unlocked after %d boosters", i+1)
			return first
		}
	}
	t.Fatalf("%d boosters of %s did not unlock %s", fewBoosters, first, lockedSet)
	return ""
}

// The demo plays every set in scope in stages small enough that a few boosters unlock the next one.
func TestRunPlay_theDemoUnlocksTheNextSetInAFewBoosters(t *testing.T) {
	t.Parallel()
	openUntilUnlock(t, servePlay(t, demoPlayArgs(t)))
}

// The collection page of the demo shows the overview, the progress by colour and the mark of the
// binder that unlocked a set, so a player sees every part of the stats.
func TestRunPlay_theDemoCollectionShowsTheStats(t *testing.T) {
	t.Parallel()
	base := servePlay(t, demoPlayArgs(t))
	first := openUntilUnlock(t, base)

	page := getBody(t, base+"/collection?set="+first)

	for _, want := range []string{
		"Cards held", "Binders complete", "<dt>Sets unlocked</dt><dd>2 / 2</dd>", "Unlock threshold reached", `aria-label="Filter by colour"`,
		"White <strong>", "Blue <strong>", "Black <strong>", "Red <strong>", "Green <strong>", "Gold <strong>", "Colourless <strong>", "Land <strong>",
		`aria-label="Filter by rarity"`, "Mythic <strong>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the demo collection page lacks %q:\n%s", want, page)
		}
	}
}

// TestCmdPlay_aCatalogueDirectoryAlonePlaysEverySetInStages runs the command as the player does:
// without --sets, the game is every set in scope of the directory, in stages, under the default
// configuration.
func TestCmdPlay_aCatalogueDirectoryAlonePlaysEverySetInStages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := demo.Build().Files.(fstest.MapFS)
	for name, file := range files {
		if err := os.WriteFile(filepath.Join(dir, name), file.Data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	stdout := newLineSignal("open http://")
	cmd := newPlayCmd(stdout, io.Discard)
	cmd.SetArgs([]string{"--catalogue-dir", dir, "--prices", filepath.Join(dir, "prices.json"),
		"--save", filepath.Join(dir, "save.json"), "--addr", "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("play: %v", err)
		}
	})
	select {
	case <-stdout.seen:
	case err := <-done:
		t.Fatalf("play stopped before serving: %v\n%s", err, stdout.String())
	case <-time.After(serveTimeout):
		t.Fatalf("play did not start:\n%s", stdout.String())
	}
	_, after, _ := strings.Cut(stdout.String(), "open http://")
	addr, _, _ := strings.Cut(after, " ")

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	home := readAll(t, resp)

	// The default first stage is five sets, so both demo sets are in it.
	for _, want := range []string{"Stage 1", `value="DEM"`, `value="DSP"`} {
		if !strings.Contains(home, want) {
			t.Errorf("home page lacks %q:\n%s", want, home)
		}
	}
	if strings.Contains(home, `value="DEX"`) || strings.Contains(home, "Expensive") {
		t.Errorf("home page lists the expensive set:\n%s", home)
	}
}
