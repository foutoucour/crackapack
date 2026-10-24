package web

import (
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

type collectionView struct {
	Overview play.Overview
	// Sets are the binders of the sets that can be opened, and LockedSets the others, which the page
	// folds away.
	Sets       []tabView
	LockedSets []tabView
	SetName    string
	// Icon is the set's icon in the colour of the rarest card held, drawn faint behind the header.
	Icon     iconView
	Owned    int
	Total    int
	Playsets int
	// PastThreshold marks a binder that has reached the unlock threshold.
	PastThreshold bool
	Rarities      filterPanel
	Colours       filterPanel
	Shows         showPanel
	Slots         []slotView
	Pools         []poolView
	// Counter is true in a game with a Counter, whose prices the slots show. Locked is true when
	// the binder's set cannot be opened yet, so that none of its cards can be bought.
	Counter    bool
	Locked     bool
	Duplicates duplicatesView
	Notice     noticeView
	// Market is true in a game with a market, whose controls the slots show.
	Market bool
}

// tabView is one entry of the list of binders.
type tabView struct {
	SetCode string
	SetName string
	Owned   int
	Total   int
	Current bool
	// Locked is true while the set cannot be opened: its binder still shows.
	Locked bool
	// Icon is the set's icon in the colour of the rarest card held.
	Icon iconView
}

// filterPanel is the list of one filter of a binder, with the name that its nav and its heading go by.
type filterPanel struct {
	Title string
	Label string
	Rows  []filterView
}

// filterView is one entry of a filter, on rarity or on colour: a link that picks it, or clears it
// when it is the one in force, with what the player holds of it. The first of a filter is the
// whole binder, which clears the filter. Class names the colour of its bar.
type filterView struct {
	Label   string
	Href    string
	Class   string
	Owned   int
	Total   int
	Current bool
}

// showPanel is the list of the show filter: what the player can do with the cards of the binder.
type showPanel struct {
	Label string
	Rows  []showView
}

// showView is one entry of the show filter: a link that picks it, or clears it when it is the one
// in force, with the number of cards of the binder it keeps under the other filters.
type showView struct {
	Label   string
	Href    string
	Count   int
	Current bool
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
	// Icon is the icon of the binder's set in the colour of the card's rarity.
	Icon iconView
	// Regular and Foil are the copies held, by finish.
	Regular int
	Foil    int
	// Sales are what the Counter pays for the copies held, one per finish, and Buy what it offers
	// for an empty slot. Both are empty in a game without a Counter.
	Sales []saleView
	Buy   buyView
	// Hunt is the market's side of an empty slot, and Lists that of each finish held. Both are
	// empty in a game without a market, and Hunt in a locked binder.
	Hunt  *huntView
	Lists []marketSaleView
	// Hunted is the hunt of an empty slot, in a game with a Counter and in an unlocked binder.
	Hunted *huntedMark
}

// Held reports whether the player holds a copy of the card, in either finish.
func (s slotView) Held() bool { return s.Regular+s.Foil > 0 }

// binderRarities lists the rarities of a binder in the order they are shown.
func binderRarities() []domain.Rarity {
	return []domain.Rarity{domain.RarityCommon, domain.RarityUncommon, domain.RarityRare, domain.RarityMythic, domain.RaritySpecial}
}

// newCollectionView builds the page of binder. filter keeps only the slots it names, while the
// progress stays the whole binder's. The slots show the prices of quotes, which
// is nil in a game without a Counter, and the controls of market, nil in a game without one.
func newCollectionView(binder play.Binder, binders []play.BinderSummary, filter binderFilter, quotes counterQuoter, market *marketIndex, hunted map[domain.CardKey]bool) (collectionView, error) {
	view := collectionView{
		Counter:       quotes != nil,
		Market:        market != nil,
		Overview:      play.OverviewOf(binders),
		SetName:       binder.SetName,
		Icon:          newIconView(binder.IconCode, rarestHeld(binder.Progress)),
		Owned:         binder.Progress.Owned,
		Total:         binder.Progress.Total,
		Playsets:      binder.Progress.Playsets,
		PastThreshold: binder.PastThreshold,
	}
	for _, b := range binders {
		tab := tabView{
			SetCode: b.SetCode, SetName: b.SetName,
			Owned: b.Progress.Owned, Total: b.Progress.Total,
			Current: b.SetCode == binder.SetCode, Locked: b.Locked,
			Icon: newIconView(b.IconCode, rarestHeld(b.Progress)),
		}
		if b.Locked {
			view.LockedSets = append(view.LockedSets, tab)
		} else {
			view.Sets = append(view.Sets, tab)
		}
		if b.SetCode == binder.SetCode {
			view.Locked = b.Locked
		}
	}

	shelf := shelf{set: binder.SetCode, filter: filter, quotes: quotes, locked: view.Locked, market: market, hunted: hunted}
	var err error
	if view.Slots, err = newSlotViews(binder.Slots, binder.IconCode, shelf); err != nil {
		return collectionView{}, err
	}
	for _, pool := range binder.Pools {
		slots, err := newSlotViews(pool.Slots, binder.IconCode, shelf)
		if err != nil {
			return collectionView{}, err
		}
		if len(slots) > 0 {
			view.Pools = append(view.Pools, poolView{Name: pool.Name, Slots: slots})
		}
	}

	view.Shows = newShowPanel(binder.SetCode, filter, quotes != nil, market != nil, view.Slots, view.Pools)
	view.Slots = keepShown(view.Slots, filter)
	view.Pools = shownPools(view.Pools, filter)

	view.Rarities = filterPanel{Title: "Rarity", Label: "Filter by rarity", Rows: []filterView{{
		Label: "All rarities", Href: collectionHref(binder.SetCode, filter.toggleRarity("")), Class: "bar-all",
		Owned: binder.Progress.Owned, Total: binder.Progress.Total, Current: filter.Rarity == "",
	}}}
	for _, r := range binderRarities() {
		if p, ok := binder.Progress.ByRarity[r]; ok && p.Total > 0 {
			view.Rarities.Rows = append(view.Rarities.Rows, filterView{
				Label: rarityLabel(r), Href: collectionHref(binder.SetCode, filter.toggleRarity(string(r))), Class: "bar-" + string(r),
				Owned: p.Owned, Total: p.Total, Current: string(r) == filter.Rarity,
			})
		}
	}
	view.Colours = filterPanel{Title: "Colour", Label: "Filter by colour", Rows: []filterView{{
		Label: "All colours", Href: collectionHref(binder.SetCode, filter.toggleColour("")), Class: "bar-all",
		Owned: binder.Progress.Owned, Total: binder.Progress.Total, Current: filter.Colour == "",
	}}}
	for _, g := range domain.ColourGroups() {
		if p, ok := binder.Progress.ByColour[g]; ok {
			view.Colours.Rows = append(view.Colours.Rows, filterView{
				Label: colourGroupLabel(g), Href: collectionHref(binder.SetCode, filter.toggleColour(string(g))), Class: "bar-" + colourGroupClass(g),
				Owned: p.Owned, Total: p.Total, Current: string(g) == filter.Colour,
			})
		}
	}
	return view, nil
}

// newShowPanel builds the show filter of the binder of set, which counts what each choice keeps of
// the slots and pools that the other filters left.
func newShowPanel(set string, filter binderFilter, counter, market bool, slots []slotView, pools []poolView) showPanel {
	panel := showPanel{Label: "Show cards"}
	for _, choice := range showChoices(counter, market) {
		keep := binderFilter{Show: choice.Value}
		count := len(keepShown(slots, keep))
		for _, pool := range pools {
			count += len(keepShown(pool.Slots, keep))
		}
		panel.Rows = append(panel.Rows, showView{
			Label: choice.Label, Href: collectionHref(set, filter.toggleShow(choice.Value)),
			Count: count, Current: choice.Value == filter.Show,
		})
	}
	return panel
}

// keepShown is the slots that the show filter of filter keeps.
func keepShown(slots []slotView, filter binderFilter) []slotView {
	return slices.DeleteFunc(slices.Clone(slots), func(s slotView) bool { return !filter.keepsSlot(s) })
}

// shownPools is the pools with the slots that the show filter of filter keeps, without the pools it
// leaves empty.
func shownPools(pools []poolView, filter binderFilter) []poolView {
	var out []poolView
	for _, pool := range pools {
		if slots := keepShown(pool.Slots, filter); len(slots) > 0 {
			out = append(out, poolView{Name: pool.Name, Slots: slots})
		}
	}
	return out
}

// percentOf is the share of total that owned makes, in whole percent and rounded down, so that 100
// means complete. An empty binder is at 0.
func percentOf(owned, total int) int {
	if total <= 0 {
		return 0
	}
	return owned * 100 / total
}

// collectionHref is the address of a binder, with filter in force.
func collectionHref(setCode string, filter binderFilter) string {
	q := filter.values()
	q.Set("set", setCode)
	return "/collection?" + q.Encode()
}

// newSlotViews builds the slots of cards that the filter of shelf keeps.
func newSlotViews(cards []play.BinderCard, iconCode string, shelf shelf) ([]slotView, error) {
	out := make([]slotView, 0, len(cards))
	for _, c := range cards {
		if !shelf.filter.keeps(c.Card) {
			continue
		}
		slot := slotView{
			ID:      c.Card.ID,
			Name:    c.Card.Name,
			Number:  c.Card.Number,
			Rarity:  string(c.Card.Rarity),
			Colour:  colourClass(c.Card.Colors),
			ImageID: c.Card.ScryfallID,
			Icon:    newIconView(iconCode, string(c.Card.Rarity)),
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

// colourGroupClass names the CSS colour of a colour group, as colourClass does for a card.
func colourGroupClass(g domain.ColourGroup) string {
	switch g {
	case domain.ColourWhite:
		return "white"
	case domain.ColourBlue:
		return "blue"
	case domain.ColourBlack:
		return "black"
	case domain.ColourRed:
		return "red"
	case domain.ColourGreen:
		return "green"
	case domain.ColourGold, domain.ColourColourless, domain.ColourLand:
		return string(g)
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
