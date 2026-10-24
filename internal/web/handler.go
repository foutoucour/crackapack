// Package web is the web client of the game: server-rendered pages over play.Game, embedded in the
// binary and served on localhost. It renders what the game returns and holds no rule of its own,
// see docs/adr/0034-the-web-client-is-server-rendered.md.
package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/images"
	"github.com/foutoucour/crackapack/internal/play"
)

//go:embed templates static
var assets embed.FS

const (
	contentSecurityPolicy = "default-src 'self'; frame-ancestors 'none'"
	// imageCacheControl lets the browser keep a card picture for a day: a picture never changes.
	imageCacheControl = "max-age=86400"
)

type handler struct {
	game   *play.Game
	images images.Source
	// icons gives the picture of a set icon, under the icon code.
	icons images.Source
	// iconFallback stands in for an icon that cannot be had; iconCSS is the stylesheet that masks
	// each set's elements with its icon.
	iconFallback []byte
	iconCSS      string
	log          *slog.Logger
	pages        map[string]*template.Template
	// notices tells the result of a purchase or a sale on the page it redirects to.
	notices noticeBox
}

// NewHandler returns the web client of game. Card pictures come from pics and set icons from icons,
// and both are served from this origin so that the page makes no request to another site. Unexpected failures are written
// to log and shown to the player as a plain "something went wrong". It panics when an embedded
// template does not parse, which is a programming error that the tests catch.
func NewHandler(game *play.Game, pics, icons images.Source, log *slog.Logger) http.Handler {
	h := &handler{game: game, images: pics, icons: icons, log: log, pages: make(map[string]*template.Template)}
	fallback, err := assets.ReadFile("static/set-icon-fallback.svg")
	if err != nil {
		panic("web: embedded fallback icon: " + err.Error())
	}
	h.iconFallback, h.iconCSS = fallback, setIconStylesheet(game.Packs())
	// The header shows the energy on every page, so the layout asks for it through a function
	// instead of every page's view carrying it.
	funcs := template.FuncMap{
		"energy":   func() energyView { return newEnergyView(h.game.Energy()) },
		"duration": humanDuration,
		// The coins show in the header of a game that has a Counter.
		"hasCounter": h.game.HasCounter,
		"balance":    h.game.Balance,
		"coins":      coinsText,
		"coinSymbol": func() string { return coinSymbol },
		"percent":    percentOf,
		// A save that failed is told on every page until one goes through.
		"saveFailed": func() bool { return h.game.SaveError() != nil },
		// The header links to the market in a game that has one, with the answers waiting.
		"marketNav": h.marketNav,
	}
	// Each page is parsed together with the layout, since pages define the same blocks.
	for _, name := range []string{"home", "pack", "collection", "confirm", "error", "market", "market_counter"} {
		h.pages[name] = template.Must(template.New(name).Funcs(funcs).ParseFS(assets, "templates/layout.html", "templates/market_parts.html", "templates/"+name+".html"))
	}

	static, err := fs.Sub(assets, "static")
	if err != nil {
		panic("web: embedded static directory: " + err.Error())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("POST /open", h.open)
	mux.HandleFunc("GET /pack", h.pack)
	mux.HandleFunc("GET /collection", h.collection)
	mux.HandleFunc("POST /buy", h.buy)
	mux.HandleFunc("POST /sell", h.sell)
	mux.HandleFunc("POST /sell-duplicates", h.sellDuplicates)
	mux.HandleFunc("GET /market", h.market)
	mux.HandleFunc("POST /ask-around", h.askAround)
	mux.HandleFunc("POST /withdraw", h.withdraw)
	mux.HandleFunc("POST /list", h.list)
	mux.HandleFunc("POST /unlist", h.unlist)
	mux.HandleFunc("POST /accept", h.accept)
	mux.HandleFunc("POST /decline", h.decline)
	mux.HandleFunc("GET /counter-offer", h.counterForm)
	mux.HandleFunc("POST /counter-offer", h.counterAnswer)
	mux.HandleFunc("GET /card-image/{id}", h.cardImage)
	mux.HandleFunc("GET /set-icon/{code}", h.setIcon)
	mux.HandleFunc("GET /set-icons.css", h.setIconsCSS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	// The form posts are the only state change: a browser must not let another site trigger them.
	return securityHeaders(http.NewCrossOriginProtection().Handler(mux))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("Content-Security-Policy", contentSecurityPolicy)
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("X-Frame-Options", "DENY")
		hdr.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (h *handler) home(w http.ResponseWriter, _ *http.Request) {
	h.render(w, http.StatusOK, "home", newHomeView(h.game.Packs(), h.game.Binders()))
}

func (h *handler) open(w http.ResponseWriter, r *http.Request) {
	if _, err := h.game.Open(r.PostFormValue("set")); err != nil {
		h.playFailed(w, err)
		return
	}
	http.Redirect(w, r, "/pack", http.StatusSeeOther)
}

// pack shows the latest opening. It is a page of its own, reached by a redirect, so that a refresh
// shows the booster again instead of opening another one.
func (h *handler) pack(w http.ResponseWriter, r *http.Request) {
	opening, ok := h.game.LastOpening()
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, http.StatusOK, "pack", newPackView(opening, h.game.Packs()))
}

// collection shows one set's binder, the first set's unless ?set= names another, and only the cards
// of one rarity when ?rarity= names one that the binder has, and of one colour group when ?colour=
// names one.
func (h *handler) collection(w http.ResponseWriter, r *http.Request) {
	// The market first: it catches the game up, and what it delivers changes the binders.
	market, err := h.marketIndex()
	if err != nil {
		h.playFailed(w, err)
		return
	}
	binders := h.game.Binders()
	code := r.URL.Query().Get("set")
	if code == "" {
		code = binders[0].SetCode
	}
	binder, err := h.game.Binder(code)
	if err != nil {
		h.playFailed(w, err)
		return
	}
	filter := binderFilterOf(r)
	if filter.Rarity != "" && binder.Progress.ByRarity[domain.Rarity(filter.Rarity)].Total == 0 {
		h.fail(w, http.StatusBadRequest, "Unknown rarity: pick one of the filters above the binder.")
		return
	}
	if filter.Colour != "" && binder.Progress.ByColour[domain.ColourGroup(filter.Colour)].Total == 0 {
		h.fail(w, http.StatusBadRequest, "Unknown colour: pick one of the filters above the binder.")
		return
	}
	var quotes counterQuoter
	if h.game.HasCounter() {
		quotes = h.game
	}
	view, err := newCollectionView(binder, binders, filter, quotes, market)
	if err != nil {
		h.playFailed(w, err)
		return
	}
	if quotes != nil {
		if view.Duplicates, err = h.duplicatesOf(binder.SetCode, filter); err != nil {
			h.playFailed(w, err)
			return
		}
	}
	view.Notice = h.notices.take()
	h.render(w, http.StatusOK, "collection", view)
}

func (h *handler) cardImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !images.ValidID(id) {
		http.Error(w, "not a card id", http.StatusBadRequest)
		return
	}
	img, err := h.images.Image(r.Context(), id)
	switch {
	case errors.Is(err, images.ErrNotFound):
		http.Error(w, "no picture for this card", http.StatusNotFound)
	case err != nil:
		h.log.Error("card image", "error", err)
		http.Error(w, "picture unavailable", http.StatusBadGateway)
	default:
		w.Header().Set("Content-Type", img.ContentType)
		w.Header().Set("Cache-Control", imageCacheControl)
		_, _ = w.Write(img.Data)
	}
}

// setIcon serves the icon of a set. A set whose icon cannot be had gets a plain stand-in, which the
// browser does not keep, so that the real icon shows as soon as it can be downloaded.
func (h *handler) setIcon(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if !images.ValidIconCode(code) {
		http.Error(w, "not a set icon code", http.StatusBadRequest)
		return
	}
	img, err := h.icons.Image(r.Context(), code)
	if err == nil {
		w.Header().Set("Content-Type", img.ContentType)
		w.Header().Set("Cache-Control", imageCacheControl)
		_, _ = w.Write(img.Data)
		return
	}
	if !errors.Is(err, images.ErrNotFound) {
		h.log.Error("set icon", "error", err)
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(h.iconFallback)
}

func (h *handler) setIconsCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write([]byte(h.iconCSS))
}

// playFailed answers a failure of the game with the status that says what went wrong.
func (h *handler) playFailed(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, play.ErrUnknownSet):
		h.fail(w, http.StatusBadRequest, "Unknown set: pick one of the sets on the home page.")
	case errors.Is(err, play.ErrLocked):
		h.fail(w, http.StatusConflict, "This set is locked: fill a binder of an open set to unlock more.")
	case errors.Is(err, play.ErrNotEnoughEnergy):
		var short *play.NotEnoughEnergyError
		if errors.As(err, &short) {
			h.fail(w, http.StatusConflict, fmt.Sprintf("Not enough energy: this booster costs %d and you have %d. Enough in %s.", short.Need, short.Have, humanDuration(short.Wait)))
			return
		}
		h.fail(w, http.StatusConflict, "Not enough energy for this booster.")
	default:
		h.log.Error("play", "error", err)
		h.fail(w, http.StatusInternalServerError, "Something went wrong.")
	}
}

func (h *handler) fail(w http.ResponseWriter, status int, message string) {
	h.render(w, status, "error", errorView{Message: message})
}

// render writes the status line before executing, so an execution error can only truncate the
// page. The templates take values the handler built, so it is not expected; it is logged if it
// happens.
func (h *handler) render(w http.ResponseWriter, status int, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := h.pages[page].ExecuteTemplate(w, "layout", data); err != nil {
		h.log.Error("render page", "page", page, "error", err)
	}
}
