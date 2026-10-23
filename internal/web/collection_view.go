package web

import (
	"net/url"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

type collectionView struct {
	Overview play.Overview
	Tabs     []tabView
	SetName  string
	Owned    int
	Total    int
	Playsets int
	// PastThreshold marks a binder that has reached the unlock threshold.
	PastThreshold bool
	Filters       []filterView
	Colours       []groupView
	Slots         []slotView
	Pools         []poolView
	// Counter is true in a game with a Counter, whose prices the slots show. Locked is true when
	// the binder's set cannot be opened yet, so that none of its cards can be bought.
	Counter    bool
	Locked     bool
	Duplicates duplicatesView
	Notice     noticeView
}

// tabView is one entry of the switcher between binders.
type tabView struct {
	SetCode string
	SetName string
	Owned   int
	Total   int
	Current bool
	// Locked is true while the set cannot be opened: its binder still shows.
	Locked bool
}

// filterView is one entry of the filter on rarity: a link, with what the player holds of that
// rarity. The first is "All", which has no count since the progress bar says it.
type filterView struct {
	Label   string
	Href    string
	Owned   int
	Total   int
	Counted bool
	Current bool
}

// groupView is what the player holds of one colour group of the binder.
type groupView struct {
	Label string
	Owned int
	Total int
}

// poolView is a section of the binder that comes from another catalogue, such as the timeshifted
// cards of Time Spiral.
type poolView struct {
	Name  string
	Slots []slotView
}

// slotView is one slot of a binder: a card with its copies, or an empty slot when it has none.
type slotView struct {
	ID      domain.CardID
	Name    string
	Number  string
	Rarity  string
	Colour  string
	ImageID string
	// Regular and Foil are the copies held, by finish.
	Regular int
	Foil    int
	// Sales are what the Counter pays for the copies held, one per finish, and Buy what it offers
	// for an empty slot. Both are empty in a game without a Counter.
	Sales []saleView
	Buy   buyView
}

// Held reports whether the player holds a copy of the card, in either finish.
func (s slotView) Held() bool { return s.Regular+s.Foil > 0 }

// binderRarities lists the rarities of a binder in the order they are shown.
func binderRarities() []domain.Rarity {
	return []domain.Rarity{domain.RarityCommon, domain.RarityUncommon, domain.RarityRare, domain.RarityMythic, domain.RaritySpecial}
}

// newCollectionView builds the page of binder. rarity, when not empty, keeps only the slots of that
// rarity, while the progress stays the whole binder's. The slots show the prices of quotes, which
// is nil in a game without a Counter.
func newCollectionView(binder play.Binder, binders []play.BinderSummary, rarity string, quotes counterQuoter) (collectionView, error) {
	view := collectionView{
		Counter:       quotes != nil,
		Overview:      play.OverviewOf(binders),
		SetName:       binder.SetName,
		Owned:         binder.Progress.Owned,
		Total:         binder.Progress.Total,
		Playsets:      binder.Progress.Playsets,
		PastThreshold: binder.PastThreshold,
	}
	for _, b := range binders {
		view.Tabs = append(view.Tabs, tabView{
			SetCode: b.SetCode, SetName: b.SetName,
			Owned: b.Progress.Owned, Total: b.Progress.Total,
			Current: b.SetCode == binder.SetCode, Locked: b.Locked,
		})
		if b.SetCode == binder.SetCode {
			view.Locked = b.Locked
		}
	}

	shelf := shelf{set: binder.SetCode, filter: rarity, quotes: quotes, locked: view.Locked}
	var err error
	if view.Slots, err = newSlotViews(binder.Slots, rarity, shelf); err != nil {
		return collectionView{}, err
	}
	for _, pool := range binder.Pools {
		slots, err := newSlotViews(pool.Slots, rarity, shelf)
		if err != nil {
			return collectionView{}, err
		}
		if len(slots) > 0 {
			view.Pools = append(view.Pools, poolView{Name: pool.Name, Slots: slots})
		}
	}

	view.Filters = []filterView{{Label: "All", Href: collectionHref(binder.SetCode, ""), Current: rarity == ""}}
	for _, r := range binderRarities() {
		if p, ok := binder.Progress.ByRarity[r]; ok && p.Total > 0 {
			view.Filters = append(view.Filters, filterView{
				Label: rarityLabel(r), Href: collectionHref(binder.SetCode, string(r)),
				Owned: p.Owned, Total: p.Total, Counted: true, Current: string(r) == rarity,
			})
		}
	}
	for _, g := range domain.ColourGroups() {
		if p, ok := binder.Progress.ByColour[g]; ok {
			view.Colours = append(view.Colours, groupView{Label: colourGroupLabel(g), Owned: p.Owned, Total: p.Total})
		}
	}
	return view, nil
}

// collectionHref is the address of a binder, filtered on rarity when it is not empty.
func collectionHref(setCode, rarity string) string {
	q := url.Values{"set": {setCode}}
	if rarity != "" {
		q.Set("rarity", rarity)
	}
	return "/collection?" + q.Encode()
}

func newSlotViews(cards []play.BinderCard, rarity string, shelf shelf) ([]slotView, error) {
	out := make([]slotView, 0, len(cards))
	for _, c := range cards {
		if rarity != "" && string(c.Card.Rarity) != rarity {
			continue
		}
		slot := slotView{
			ID:      c.Card.ID,
			Name:    c.Card.Name,
			Number:  c.Card.Number,
			Rarity:  string(c.Card.Rarity),
			Colour:  colourClass(c.Card.Colors),
			ImageID: c.Card.ScryfallID,
			Regular: c.Regular,
			Foil:    c.Foil,
		}
		if err := shelf.apply(&slot, c); err != nil {
			return nil, err
		}
		out = append(out, slot)
	}
	return out, nil
}

func colourGroupLabel(g domain.ColourGroup) string {
	switch g {
	case domain.ColourWhite:
		return "White"
	case domain.ColourBlue:
		return "Blue"
	case domain.ColourBlack:
		return "Black"
	case domain.ColourRed:
		return "Red"
	case domain.ColourGreen:
		return "Green"
	case domain.ColourGold:
		return "Gold"
	case domain.ColourColourless:
		return "Colourless"
	case domain.ColourLand:
		return "Land"
	default:
		return string(g)
	}
}

func rarityLabel(r domain.Rarity) string {
	switch r {
	case domain.RarityCommon:
		return "Common"
	case domain.RarityUncommon:
		return "Uncommon"
	case domain.RarityRare:
		return "Rare"
	case domain.RarityMythic:
		return "Mythic"
	case domain.RaritySpecial:
		return "Special"
	default:
		return string(r)
	}
}
