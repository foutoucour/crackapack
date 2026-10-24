package web_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func TestHome_aSetRowHasItsIconAsAWatermarkBehindIt(t *testing.T) {
	t.Parallel()
	srv, c := iconServer(t, ravIcons(), ravWithIcon())

	_, body := get(t, c, srv.URL+"/")

	want := `<span class="pack-watermark set-icon-rav" aria-hidden="true"></span>`
	if !strings.Contains(body, want) {
		t.Errorf("the row has no watermark, want %s in:\n%s", want, body)
	}
}

func TestHome_aSetWithoutAnIconCodeHasNoWatermark(t *testing.T) {
	t.Parallel()
	rav := domaintest.NewRavnicaCatalogue()
	rav.IconCode = ""
	srv, c := iconServer(t, ravIcons(), rav)

	_, body := get(t, c, srv.URL+"/")

	if strings.Contains(body, "pack-watermark") {
		t.Errorf("a set with no icon code has a watermark:\n%s", body)
	}
}
