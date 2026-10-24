package web

import (
	"errors"
	"io/fs"
	"net/http"
	"regexp"
)

const svgContentType = "image/svg+xml"

// setCodePattern is what a set code looks like in the catalogue directory: the code ends up in a
// file name, so nothing but short capitals and digits gets that far.
var setCodePattern = regexp.MustCompile(`^[A-Z0-9]{1,8}$`)

// iconFile is the name of the icon of a set in the catalogue directory.
func iconFile(code string) string { return code + ".svg" }

// hasSetIcon tells whether the catalogue directory holds an icon for the set.
func (h *handler) hasSetIcon(code string) bool {
	if !setCodePattern.MatchString(code) {
		return false
	}
	_, err := fs.Stat(h.icons, iconFile(code))
	return err == nil
}

// setIcon serves the icon of a set from the catalogue directory, where fetchdata put it.
func (h *handler) setIcon(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if !setCodePattern.MatchString(code) {
		http.Error(w, "not a set code", http.StatusBadRequest)
		return
	}
	icon, err := fs.ReadFile(h.icons, iconFile(code))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "no icon for this set", http.StatusNotFound)
	case err != nil:
		h.log.Error("set icon", "set", code, "error", err)
		http.Error(w, "icon unavailable", http.StatusInternalServerError)
	default:
		w.Header().Set("Content-Type", svgContentType)
		w.Header().Set("Cache-Control", imageCacheControl)
		_, _ = w.Write(icon)
	}
}
