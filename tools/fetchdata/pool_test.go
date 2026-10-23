package main

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	// hostSet has one card of its own and two on its booster sheet that live in other files.
	hostSet = `{"data":{"code":"HST","name":"Host","cards":[{"uuid":"host-own","rarity":"mythic"}],
		"booster":{"draft":{"sheets":{"borrowed":{"totalWeight":3,"cards":{"host-own":1,"alpha-card":1,"beta-card":1}}}}}}}`
	alphaSet = `{"data":{"code":"ALP","name":"Alpha","cards":[
		{"uuid":"alpha-card","name":"Borrowed Alpha","rarity":"mythic","finishes":["foil","nonfoil"]},
		{"uuid":"alpha-other","name":"Stays Home","rarity":"common"}]}}`
	betaSet = `{"data":{"code":"BET","name":"Beta","cards":[{"uuid":"beta-card","name":"Borrowed Beta","rarity":"mythic"}]}}`

	// hostPriceDoc prices a borrowed card, which only the donor files tell fetchdata to ask for.
	hostPriceDoc = `{"data":{"alpha-card":{"paper":{"tcgplayer":{"retail":{"normal":{"2026-10-11":12.5}}}}}}}`
)

const (
	testPoolCode = "BORROWED"
	testPoolName = "Borrowed cards"
)

func testPool() borrowedPool {
	return borrowedPool{host: "HST", code: testPoolCode, name: testPoolName, donors: []string{"ALP", "BET"}}
}

type poolFile struct {
	Data struct {
		Code  string `json:"code"`
		Name  string `json:"name"`
		Cards []struct {
			UUID     string   `json:"uuid"`
			Name     string   `json:"name"`
			Finishes []string `json:"finishes"`
		} `json:"cards"`
	} `json:"data"`
}

func runWithPools(t *testing.T, srv *httptest.Server, args ...string) error {
	t.Helper()
	a := app{
		client:  srv.Client(),
		baseURL: srv.URL,
		stdout:  io.Discard,
		stderr:  io.Discard,
		pools:   []borrowedPool{testPool()},
	}
	return a.run(context.Background(), args)
}

func readPool(t *testing.T, dir string) poolFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, testPoolCode+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var pool poolFile
	if err := json.Unmarshal(raw, &pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestRunWritesThePoolOfTheCardsASheetBorrowsFromOtherSets(t *testing.T) {
	srv, _ := newMTGJSON(t, map[string][]byte{
		"/HST.json":               []byte(hostSet),
		"/ALP.json":               []byte(alphaSet),
		"/BET.json":               []byte(betaSet),
		"/AllPricesToday.json.gz": gzipped(t, hostPriceDoc),
	})
	dir := t.TempDir()

	err := runWithPools(t, srv, "--sets", "HST", "--catalogue-dir", dir, "--prices", filepath.Join(dir, "p.json"))
	if err != nil {
		t.Fatal(err)
	}

	pool := readPool(t, dir)
	if pool.Data.Code != testPoolCode || pool.Data.Name != testPoolName {
		t.Errorf("pool = %q %q, want %q %q", pool.Data.Code, pool.Data.Name, testPoolCode, testPoolName)
	}
	var got []string
	for _, c := range pool.Data.Cards {
		got = append(got, c.UUID)
	}
	if want := []string{"alpha-card", "beta-card"}; !slices.Equal(got, want) {
		t.Errorf("pool cards = %v, want %v: the borrowed cards only", got, want)
	}
	if first := pool.Data.Cards[0]; first.Name != "Borrowed Alpha" || !slices.Equal(first.Finishes, []string{"foil", "nonfoil"}) {
		t.Errorf("first pool card = %+v, want the donor's card kept as it was", first)
	}
}

func TestRunFetchesTheDonorsOfAHostAndPricesTheirCards(t *testing.T) {
	srv, _ := newMTGJSON(t, map[string][]byte{
		"/HST.json":               []byte(hostSet),
		"/ALP.json":               []byte(alphaSet),
		"/BET.json":               []byte(betaSet),
		"/AllPricesToday.json.gz": gzipped(t, hostPriceDoc),
	})
	dir := t.TempDir()
	pricesPath := filepath.Join(dir, "p.json")

	if err := runWithPools(t, srv, "--sets", "HST", "--catalogue-dir", dir, "--prices", pricesPath); err != nil {
		t.Fatal(err)
	}

	for _, code := range []string{"ALP", "BET"} {
		if _, err := os.Stat(filepath.Join(dir, code+".json")); err != nil {
			t.Errorf("donor %s was not downloaded: %v", code, err)
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
	if want := map[string]int64{"alpha-card": 1250}; !maps.Equal(want, got) {
		t.Errorf("price snapshot = %v, want %v", got, want)
	}
}

func TestRunWritesNoPoolWhenTheHostIsNotRequested(t *testing.T) {
	srv, requests := newMTGJSON(t, map[string][]byte{
		"/RAV.json":               []byte(ravSet),
		"/AllPricesToday.json.gz": gzipped(t, allPrices),
	})
	dir := t.TempDir()

	if err := runWithPools(t, srv, "--sets", "RAV", "--catalogue-dir", dir, "--prices", filepath.Join(dir, "p.json")); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, testPoolCode+".json")); !os.IsNotExist(err) {
		t.Errorf("pool file exists although its host was not requested: %v", err)
	}
	if n := requests.Load(); n != 2 { // the set and the price file, no donor
		t.Errorf("made %d requests, want 2", n)
	}
}

func TestRunNamesTheBorrowedCardsNoDonorHolds(t *testing.T) {
	srv, _ := newMTGJSON(t, map[string][]byte{
		"/HST.json":               []byte(hostSet),
		"/ALP.json":               []byte(alphaSet),
		"/BET.json":               []byte(`{"data":{"code":"BET","name":"Beta","cards":[]}}`),
		"/AllPricesToday.json.gz": gzipped(t, hostPriceDoc),
	})
	dir := t.TempDir()
	pricesPath := filepath.Join(dir, "p.json")

	err := runWithPools(t, srv, "--sets", "HST", "--catalogue-dir", dir, "--prices", pricesPath)

	if err == nil || !strings.Contains(err.Error(), "HST") || !strings.Contains(err.Error(), "beta-card") {
		t.Fatalf("error = %v, want one naming HST and beta-card", err)
	}
	for _, name := range []string{testPoolCode + ".json", "p.json"} {
		if _, statErr := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(statErr) {
			t.Errorf("%s exists after a failed run, want none", name)
		}
	}
}
