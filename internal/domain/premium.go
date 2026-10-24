package domain

// LackingCatalogue is cat with every sheet cut to the cards that held tells the player has fewer than
// a playset of, so that a booster drawn from it holds nothing the player has completed. A sheet keeps
// all its cards when fewer are lacking than the most that a booster draws from it: that slot then
// draws as it always does, and the booster never fails. cat is not changed.
func LackingCatalogue(cat SetCatalogue, held func(CardID) int) SetCatalogue {
	needed := cardsDrawnPerSheet(cat.BoosterOptions)
	out := cat
	out.Sheets = make(map[string]Sheet, len(cat.Sheets))
	for name, sheet := range cat.Sheets {
		out.Sheets[name] = lackingSheet(sheet, needed[name], held)
	}
	return out
}

// cardsDrawnPerSheet is the most cards that any booster option draws from each sheet.
func cardsDrawnPerSheet(options []BoosterOption) map[string]int {
	needed := make(map[string]int)
	for _, opt := range options {
		for sheet, n := range opt.Contents {
			needed[sheet] = max(needed[sheet], n)
		}
	}
	return needed
}

// lackingSheet is sheet without the cards held has a playset of, or sheet itself when that leaves
// fewer than needed.
func lackingSheet(sheet Sheet, needed int, held func(CardID) int) Sheet {
	kept := make([]SheetCard, 0, len(sheet.Cards))
	weight := 0
	for _, sc := range sheet.Cards {
		if held(sc.ID) < PlaysetSize {
			kept = append(kept, sc)
			weight += sc.Weight
		}
	}
	if len(kept) < needed {
		return sheet
	}
	sheet.Cards, sheet.TotalWeight = kept, weight
	return sheet
}
