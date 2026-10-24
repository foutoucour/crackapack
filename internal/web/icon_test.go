package web_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

const ravIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><path d="M0 0h8v8z"/></svg>`

func TestHome_aTileShowsTheIconOfItsSetWhenTheCatalogueHoldsOne(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	tsp, _ := domaintest.NewTimeSpiralCatalogues()
	srv, c, _ := serveWithIcons(t, newGame(t, rav, tsp), fakeImages{}, fstest.MapFS{rav.Code + ".svg": {Data: []byte(ravIcon)}})

	_, body := get(t, c, srv.URL+"/")

	if want := `src="/set-icon/` + rav.Code + `"`; !strings.Contains(body, want) {
		t.Errorf("home page lacks %q:\n%s", want, body)
	}
	if bad := `/set-icon/` + tsp.Code; strings.Contains(body, bad) {
		t.Errorf("home page links %q although the catalogue holds no icon for it:\n%s", bad, body)
	}
}

func TestSetIcon_servesTheSVGOfTheCatalogue(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	srv, c, _ := serveWithIcons(t, newGame(t, rav), fakeImages{}, fstest.MapFS{rav.Code + ".svg": {Data: []byte(ravIcon)}})

	resp, body := get(t, c, srv.URL+"/set-icon/"+rav.Code)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
	if resp.Header.Get("Cache-Control") == "" {
		t.Error("Cache-Control is empty: the browser would fetch the icon on every visit")
	}
	if body != ravIcon {
		t.Errorf("body = %q, want the icon of the catalogue", body)
	}
}

func TestSetIcon_refusesWhatIsNotASetCodeAndDoesNotInventAnIcon(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	icons := fstest.MapFS{
		rav.Code + ".svg": {Data: []byte(ravIcon)},
		"notes.txt":       {Data: []byte("secret")},
	}
	srv, c, _ := serveWithIcons(t, newGame(t, rav), fakeImages{}, icons)

	tests := []struct {
		name string
		code string
		want int
	}{
		{name: "set without an icon", code: "TSP", want: http.StatusNotFound},
		{name: "lower case", code: "rav", want: http.StatusBadRequest},
		{name: "encoded path separator", code: "RAV%2F..%2Fnotes", want: http.StatusBadRequest},
		{name: "too long", code: "ABCDEFGHI", want: http.StatusBadRequest},
		{name: "other file", code: "notes.txt", want: http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := get(t, c, srv.URL+"/set-icon/"+tc.code)

			if resp.StatusCode != tc.want {
				t.Errorf("GET /set-icon/%s = %d, want %d", tc.code, resp.StatusCode, tc.want)
			}
		})
	}
}
