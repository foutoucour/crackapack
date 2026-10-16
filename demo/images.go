package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/images"
)

// svgContentType is the media type of the demo card pictures.
const svgContentType = "image/svg+xml"

// colourlessFill is the face colour of a demo card picture with no colour or several.
const colourlessFill = "#d4d6dc"

// assignScryfallIDs gives every card of cats a Scryfall ID derived from its own ID, so that the
// demo exercises the picture path and stays deterministic. The IDs are well formed but belong to
// no real card: the demo draws its own pictures, see cardArt.
func assignScryfallIDs(cats ...domain.SetCatalogue) {
	for _, cat := range cats {
		for id, card := range cat.Cards {
			card.ScryfallID = syntheticScryfallID(id)
			cat.Cards[id] = card
		}
	}
}

// syntheticScryfallID formats the first 16 bytes of the SHA-256 of id as a lower-case UUID.
func syntheticScryfallID(id domain.CardID) string {
	sum := sha256.Sum256([]byte(id))
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// cardArt is the images.Source of the demo: it draws each card as a small SVG, since the demo
// cards have no picture on Scryfall and the demo needs no network.
type cardArt struct {
	cards map[string]domain.Card // by Scryfall ID
}

func newCardArt(cats ...domain.SetCatalogue) cardArt {
	art := cardArt{cards: make(map[string]domain.Card)}
	for _, cat := range cats {
		for _, card := range cat.Cards {
			art.cards[card.ScryfallID] = card
		}
	}
	return art
}

// Image implements images.Source.
func (a cardArt) Image(_ context.Context, scryfallID string) (images.Image, error) {
	if !images.ValidID(scryfallID) {
		return images.Image{}, fmt.Errorf("%w: %q", images.ErrInvalidID, scryfallID)
	}
	card, ok := a.cards[scryfallID]
	if !ok {
		return images.Image{}, fmt.Errorf("demo card art %s: %w", scryfallID, images.ErrNotFound)
	}
	return images.Image{ContentType: svgContentType, Data: []byte(cardSVG(card))}, nil
}

// cardSVG draws a card at the 488 by 680 size of a Scryfall "normal" picture.
func cardSVG(card domain.Card) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="488" height="680" viewBox="0 0 488 680" role="img" aria-label="%[1]s">
  <rect x="8" y="8" width="472" height="664" rx="28" fill="#15161a"/>
  <rect x="24" y="24" width="440" height="632" rx="16" fill="%[2]s"/>
  <text x="44" y="78" font-family="sans-serif" font-size="30" font-weight="700" fill="#15161a">%[1]s</text>
  <rect x="44" y="110" width="400" height="360" rx="8" fill="#15161a" fill-opacity="0.18"/>
  <text x="244" y="300" text-anchor="middle" font-family="sans-serif" font-size="40" fill="#15161a">DEMO ART</text>
  <text x="244" y="350" text-anchor="middle" font-family="sans-serif" font-size="24" fill="#15161a">no real card</text>
  <text x="44" y="520" font-family="sans-serif" font-size="26" fill="#15161a">%[3]s</text>
</svg>
`, html.EscapeString(card.Name), fillFor(card), html.EscapeString(string(card.Rarity)))
}

func fillFor(card domain.Card) string {
	if len(card.Colors) != 1 {
		return colourlessFill
	}
	switch card.Colors[0] {
	case "W":
		return "#f4efd9"
	case "U":
		return "#bcd7f2"
	case "B":
		return "#c4bcc9"
	case "R":
		return "#f2c1b3"
	case "G":
		return "#bfe0bd"
	default:
		return colourlessFill
	}
}
