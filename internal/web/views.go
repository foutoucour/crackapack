package web

import (
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

type homeView struct {
	// Staged is true in a game whose sets open up in stages: each stage has a heading of its own.
	Staged bool
	Stages []stageView
}

// stageView is one stage of the home page: the sets the player can open, as tiles, and the locked
// ones as a short list, so that a hundred sets stay a page one can scan.
type stageView struct {
	Number int
	Open   []tileView
	Locked []play.Pack
}

// tileView is a set the player can open, with its icon in the colour of the rarest card held.
type tileView struct {
	play.Pack
	Icon  iconView
	Price boosterPrice
}

// Size is the number of sets in the stage.
func (s stageView) Size() int { return len(s.Open) + len(s.Locked) }

// newHomeView groups packs by stage, keeping their order. A game without stages has one group.
// binders tell the rarest card held of each set, which colours its icon.
func newHomeView(packs []play.Pack, binders []play.BinderSummary) homeView {
	rarest := make(map[string]string, len(binders))
	for _, b := range binders {
		rarest[b.SetCode] = rarestHeld(b.Progress)
	}
	var view homeView
	for _, p := range packs {
		if n := len(view.Stages); n == 0 || view.Stages[n-1].Number != p.Stage {
			view.Stages = append(view.Stages, stageView{Number: p.Stage})
		}
		stage := &view.Stages[len(view.Stages)-1]
		if p.Locked {
			stage.Locked = append(stage.Locked, p)
		} else {
			stage.Open = append(stage.Open, tileView{Pack: p, Icon: newIconView(p.IconCode, rarest[p.SetCode]), Price: newBoosterPrice(p)})
		}
		view.Staged = view.Staged || p.Stage > 0
	}
	return view
}

type errorView struct {
	Message string
	// BackHref and BackLabel are the way out of the page, the home page when they are empty.
	BackHref  string
	BackLabel string
	// AltHref and AltLabel are a second way out, such as the market page, empty when there is none.
	AltHref  string
	AltLabel string
}

type packView struct {
	SetName string
	SetCode string
	Cards   []cardView
	// Left is the welcome boosters of this set the player still holds.
	Left int
	// Price is what another booster of the set costs once the welcome ones are gone.
	Price boosterPrice
	// Unlocked are the sets the booster unlocked.
	Unlocked []play.UnlockedSet
	// Preload are the pictures to fetch before the reveal needs them: the first card's, the second's
	// and the last's, which is what "skip to the end" shows.
	Preload []string
	// Icon is the icon of the set, behind the buttons.
	Icon iconView
}

// cardValuer says what a card is worth in coins, and fails in a game that has no coins.
type cardValuer func(domain.CardID, domain.Finish) (domain.Coins, error)

type cardView struct {
	Name   string
	Rarity string
	Colour string
	// ImageID is the card's Scryfall ID, empty when it has none and the card shows as text.
	ImageID string
	Foil    bool
	New     bool
	// Highlight is the rarity of a new rare or mythic, which turns with an effect of its own, and
	// empty for any other card.
	Highlight string
	// Value is the reference value of the card in coins, zero when the page shows none.
	Value domain.Coins
	// Icon is the icon of the card's set in the colour of its rarity.
	Icon iconView
}

func newPackView(opening play.Opening, packs []play.Pack, value cardValuer) packView {
	view := packView{
		SetName:  opening.SetName,
		SetCode:  opening.SetCode,
		Cards:    make([]cardView, len(opening.Cards)),
		Unlocked: opening.Unlocked,
		Icon:     newIconView(opening.IconCode, string(domain.RarityCommon)),
	}
	for _, p := range packs {
		if p.SetCode == opening.SetCode {
			view.Left, view.Price = p.Count, newBoosterPrice(p)
		}
	}
	for i, c := range opening.Cards {
		view.Cards[i] = cardView{
			Name:    c.Card.Name,
			Rarity:  string(c.Card.Rarity),
			Colour:  colourClass(c.Card.Colors),
			ImageID: c.Card.ScryfallID,
			Foil:    c.Finish == domain.FinishFoil,
			New:     c.New,
			Icon:    newIconView(opening.IconCode, string(c.Card.Rarity)),
		}
		if c.New && (c.Card.Rarity == domain.RarityRare || c.Card.Rarity == domain.RarityMythic) {
			view.Cards[i].Highlight = string(c.Card.Rarity)
		}
		// A game without coins has no value to show, and says so with an error.
		if coins, err := value(c.Card.ID, c.Finish); err == nil {
			view.Cards[i].Value = coins
		}
	}
	view.Preload = preloadIDs(view.Cards)
	return view
}

// preloadIDs are the pictures of the first, the second and the last card, the cards without a
// picture left out.
func preloadIDs(cards []cardView) []string {
	var ids []string
	for i, c := range cards {
		if (i <= 1 || i == len(cards)-1) && c.ImageID != "" {
			ids = append(ids, c.ImageID)
		}
	}
	return ids
}

// colourClass names the CSS frame of a card: "colourless" for no colour, the colour's name for one,
// and "gold" for several.
func colourClass(colours []string) string {
	if len(colours) > 1 {
		return "gold"
	}
	if len(colours) == 0 {
		return "colourless"
	}
	switch colours[0] {
	case "W":
		return "white"
	case "U":
		return "blue"
	case "B":
		return "black"
	case "R":
		return "red"
	case "G":
		return "green"
	default:
		return "colourless"
	}
}
