package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	ravSet = `{"meta":{"date":"2026-10-11"},"data":{"code":"RAV","cards":[
		{"uuid":"rav-plain"},{"uuid":"rav-foil-only"},{"uuid":"rav-unpriced"},{"uuid":"rav-rounding"}]}}`
	tspSet = `{"data":{"code":"TSP","cards":[{"uuid":"tsp-plain"}]}}`

	// allPrices mirrors the MTGJSON AllPricesToday layout: uuid, medium, provider, list, finish, date.
	allPrices = `{"meta":{"date":"2026-10-11"},"data":{
		"rav-plain":{"paper":{
			"tcgplayer":{"currency":"USD","buylist":{},"retail":{"normal":{"2026-10-10":9.99,"2026-10-11":0.26},"foil":{"2026-10-11":6.16}}},
			"cardmarket":{"currency":"EUR","buylist":{},"retail":{"normal":{"2026-10-11":0.18}}}}},
		"rav-foil-only":{"paper":{"tcgplayer":{"currency":"USD","buylist":{},"retail":{"foil":{"2026-10-11":6.16}}}}},
		"rav-unpriced":{"paper":{"cardmarket":{"currency":"EUR","buylist":{},"retail":{"normal":{"2026-10-11":0.18}}}}},
		"rav-rounding":{"paper":{"tcgplayer":{"currency":"USD","buylist":{},"retail":{"normal":{"2026-10-11":1.15}}}}},
		"tsp-plain":{"paper":{"tcgplayer":{"currency":"USD","buylist":{},"retail":{"normal":{"2026-10-11":0.29}}}},"mtgo":{}},
		"other-set-card":{"paper":{"tcgplayer":{"currency":"USD","buylist":{},"retail":{"normal":{"2026-10-11":50}}}}}}}`
)

func gzipped(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := io.WriteString(zw, s); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newMTGJSON serves the given body per URL path and counts the requests it receives.
func newMTGJSON(t *testing.T, files map[string][]byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func runTool(t *testing.T, srv *httptest.Server, args ...string) error {
	t.Helper()
	a := app{client: srv.Client(), baseURL: srv.URL, stdout: io.Discard, stderr: io.Discard}
	return a.run(context.Background(), args)
}

func TestRunWritesCataloguesAndPriceSnapshotForRequestedSets(t *testing.T) {
	srv, _ := newMTGJSON(t, map[string][]byte{
		"/RAV.json":               []byte(ravSet),
		"/TSP.json":               []byte(tspSet),
		"/AllPricesToday.json.gz": gzipped(t, allPrices),
	})
	dir := t.TempDir()
	catDir := filepath.Join(dir, "catalogues")
	pricesPath := filepath.Join(dir, "out", "prices.json")

	err := runTool(t, srv, "--sets", "rav,TSP", "--catalogue-dir", catDir, "--prices", pricesPath)
	if err != nil {
		t.Fatal(err)
	}

	for code, want := range map[string]string{"RAV": ravSet, "TSP": tspSet} {
		got, err := os.ReadFile(filepath.Join(catDir, code+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s.json differs from the downloaded body", code)
		}
	}

	raw, err := os.ReadFile(pricesPath)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]int64
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{
		"rav-plain":     26,  // latest date, tcgplayer normal
		"rav-foil-only": 616, // foil is the fallback when no normal price exists
		"rav-rounding":  115, // 1.15 is 114.99999999999999 in float64
		"tsp-plain":     29,
	}
	if !maps.Equal(want, got) {
		t.Errorf("price snapshot = %v, want %v", got, want)
	}
}

func TestRunRejectsInvalidSetCodesBeforeAnyDownload(t *testing.T) {
	tests := []struct {
		name string
		sets string
	}{
		{name: "no sets", sets: ""},
		{name: "path traversal", sets: "../evil"},
		{name: "separator inside", sets: "RAV/x"},
		{name: "empty entry", sets: "RAV,,TSP"},
		{name: "too long", sets: "ABCDEFGHI"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, requests := newMTGJSON(t, nil)
			dir := t.TempDir()

			err := runTool(t, srv, "--sets", tc.sets, "--catalogue-dir", dir, "--prices", filepath.Join(dir, "p.json"))

			if err == nil {
				t.Fatal("run succeeded, want an error")
			}
			if n := requests.Load(); n != 0 {
				t.Errorf("made %d requests, want none", n)
			}
		})
	}
}

func TestRunNamesTheSetMissingUpstreamAndWritesNoPrices(t *testing.T) {
	srv, _ := newMTGJSON(t, map[string][]byte{"/AllPricesToday.json.gz": gzipped(t, allPrices)})
	dir := t.TempDir()
	pricesPath := filepath.Join(dir, "prices.json")

	err := runTool(t, srv, "--sets", "ZZZ", "--catalogue-dir", dir, "--prices", pricesPath)

	if err == nil || !strings.Contains(err.Error(), "ZZZ") || !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %v, want one naming ZZZ and status 404", err)
	}
	for _, name := range []string{"ZZZ.json", "prices.json"} {
		if _, statErr := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(statErr) {
			t.Errorf("%s exists after a failed run, want none", name)
		}
	}
}

func TestRunLeavesNoPartialFileWhenPricesAreTruncated(t *testing.T) {
	full := gzipped(t, allPrices)
	srv, _ := newMTGJSON(t, map[string][]byte{
		"/RAV.json":               []byte(ravSet),
		"/AllPricesToday.json.gz": full[:len(full)/2],
	})
	dir := t.TempDir()
	pricesPath := filepath.Join(dir, "prices.json")

	if err := runTool(t, srv, "--sets", "RAV", "--catalogue-dir", dir, "--prices", pricesPath); err == nil {
		t.Fatal("run succeeded on a truncated price file, want an error")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "RAV.json" {
			t.Errorf("unexpected leftover file %q", e.Name())
		}
	}
}
