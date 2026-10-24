package play

import (
	"cmp"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// maxSwapCopies is the most copies a swap asks for: a trade holds ten cards a side.
const maxSwapCopies = 10

// questColours and questRarities are what a sell or a buy quest filters by.
func questColours() []domain.ColourGroup {
	return []domain.ColourGroup{domain.ColourWhite, domain.ColourBlue, domain.ColourBlack, domain.ColourRed, domain.ColourGreen}
}

func questRarities() []domain.Rarity {
	return []domain.Rarity{domain.RarityCommon, domain.RarityUncommon, domain.RarityRare}
}

// drawQuests draws up to count quests of period from the templates, each template at most once and
// by its weight. A template that cannot be drawn now, for want of a card or a set to name, is left
// out, so a board may hold fewer quests than asked for. g.mu must be held.
func (g *Game) drawQuests(period domain.QuestPeriod, count int) []*quest {
	var pool []domain.QuestTemplate
	for _, t := range g.quests.rules.Templates {
		if t.Period == period {
			pool = append(pool, t)
		}
	}
	var drawn []*quest
	for len(drawn) < count && len(pool) > 0 {
		i := g.pickTemplate(pool)
		t := pool[i]
		pool = slices.Delete(pool, i, i+1)
		if q, ok := g.questFrom(t); ok {
			drawn = append(drawn, q)
		}
	}
	return drawn
}

// pickTemplate is the index of a template of pool, chosen by weight.
func (g *Game) pickTemplate(pool []domain.QuestTemplate) int {
	total := 0
	for _, t := range pool {
		total += t.Weight
	}
	pick := g.quests.rng.IntN(total)
	for i, t := range pool {
		if pick < t.Weight {
			return i
		}
		pick -= t.Weight
	}
	return len(pool) - 1
}

// questFrom draws one quest of template t, naming the set, the filter and the cards it needs, and
// reports false when the game has nothing to name for it.
func (g *Game) questFrom(t domain.QuestTemplate) (*quest, bool) {
	q := &quest{Quest: domain.Quest{Period: t.Period, Action: t.Action, Target: t.Target}, state: QuestOpen}
	if t.Action == domain.QuestOpenBoosters {
		set, ok := g.pickSet(g.openSets())
		if !ok {
			return nil, false
		}
		q.Set = set
	}
	switch t.FilterBy {
	case domain.QuestFilterColour:
		q.Filter.Colour = questColours()[g.quests.rng.IntN(len(questColours()))]
	case domain.QuestFilterRarity:
		q.Filter.Rarity = questRarities()[g.quests.rng.IntN(len(questRarities()))]
	case domain.QuestFilterNone:
	}
	reward, ok := g.rewardFrom(t)
	if !ok {
		return nil, false
	}
	q.Reward = reward
	return q, true
}

// rewardFrom draws the reward of template t, and reports false when there is nothing to name.
func (g *Game) rewardFrom(t domain.QuestTemplate) (domain.QuestReward, bool) {
	reward := domain.QuestReward{Kind: t.Reward}
	switch t.Reward {
	case domain.RewardWinCard:
		id, ok := g.pickCard(g.lackingCards(t.MaxValue))
		reward.Card = domain.CardKey{ID: id, Finish: domain.FinishNonFoil}
		return reward, ok
	case domain.RewardFoilCard:
		id, ok := g.pickCard(g.foilableCards(t.MaxValue))
		reward.Card = domain.CardKey{ID: id, Finish: domain.FinishNonFoil}
		return reward, ok
	case domain.RewardPremiumBooster:
		var ok bool
		reward.Set, ok = g.pickSet(g.openSets())
		return reward, ok
	case domain.RewardUnlockSet:
		var ok bool
		reward.Set, ok = g.pickLockedSet()
		return reward, ok
	case domain.RewardDoubleDuplicates:
		var ok bool
		reward.Set, ok = g.pickSet(g.startedSets())
		return reward, ok
	case domain.RewardSwap:
		return g.swapReward(t.MaxValue)
	}
	return reward, false
}

// pickSet is a set of codes chosen at random.
func (g *Game) pickSet(codes []string) (string, bool) {
	if len(codes) == 0 {
		return "", false
	}
	return codes[g.quests.rng.IntN(len(codes))], true
}

// pickCard is a card of ids chosen at random.
func (g *Game) pickCard(ids []domain.CardID) (domain.CardID, bool) {
	if len(ids) == 0 {
		return "", false
	}
	return ids[g.quests.rng.IntN(len(ids))], true
}

// openSets are the codes of the sets the player can open, in display order.
func (g *Game) openSets() []string {
	var codes []string
	for _, cat := range g.sets {
		if !g.locked(cat.Code) {
			codes = append(codes, cat.Code)
		}
	}
	return codes
}

// startedSets are the codes of the open sets whose binder holds a card of the player's.
func (g *Game) startedSets() []string {
	var codes []string
	for _, code := range g.openSets() {
		if g.collection.BinderProgress(g.binders[code]).Owned > 0 {
			codes = append(codes, code)
		}
	}
	return codes
}

// pickLockedSet is a locked set of the earliest stage that has one, chosen at random: the next stage
// the player has not finished opening.
func (g *Game) pickLockedSet() (string, bool) {
	s := g.staging
	if s == nil {
		return "", false
	}
	for _, stage := range s.stages {
		var locked []string
		for _, code := range stage {
			if !s.unlocked[code] {
				locked = append(locked, code)
			}
		}
		if len(locked) > 0 {
			return g.pickSet(locked)
		}
	}
	return "", false
}

// lackingCards are the cards, by ID, of the unlocked binders that the player holds no copy of and
// that are worth at most ceiling coins, and at least one.
func (g *Game) lackingCards(ceiling domain.Coins) []domain.CardID {
	seen := make(map[domain.CardID]bool)
	var ids []domain.CardID
	for _, code := range g.openSets() {
		for id, card := range g.binders[code].Cards {
			if seen[id] || card.IsBasicLand || g.collection.Copies(id) > 0 {
				continue
			}
			seen[id] = true
			if g.worth(domain.CardKey{ID: id, Finish: domain.FinishNonFoil}, ceiling) {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return ids
}

// foilableCards are the cards, by ID, that the player holds a non-foil copy of, that exist as foils
// and are worth at most ceiling coins.
func (g *Game) foilableCards(ceiling domain.Coins) []domain.CardID {
	var ids []domain.CardID
	for key := range g.collection.Snapshot() {
		card := g.cards[key.ID]
		if key.Finish == domain.FinishNonFoil && slices.Contains(card.Finishes, domain.FinishFoil) && g.worth(key, ceiling) {
			ids = append(ids, key.ID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// worth reports whether the card key is worth between one coin and ceiling coins at the Counter's
// reference.
func (g *Game) worth(key domain.CardKey, ceiling domain.Coins) bool {
	value, err := g.economy.counter.Reference(key)
	return err == nil && value >= 1 && value <= ceiling
}

// swapReward draws a swap: a card the player lacks, worth at most ceiling coins, for the fewest copies
// of one card the player has spare whose reference value covers it. It reports false when no spare
// card pays for a card the player lacks.
func (g *Game) swapReward(ceiling domain.Coins) (domain.QuestReward, bool) {
	rng := g.quests.rng
	wanted := g.lackingCards(ceiling)
	rng.Shuffle(len(wanted), func(i, j int) { wanted[i], wanted[j] = wanted[j], wanted[i] })
	spare := g.spareCopies()
	keys := make([]domain.CardKey, 0, len(spare))
	for key := range spare {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b domain.CardKey) int { return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Finish, b.Finish)) })
	rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	for _, id := range wanted {
		want := domain.CardKey{ID: id, Finish: domain.FinishNonFoil}
		wantValue, _ := g.economy.counter.Reference(want)
		for _, give := range keys {
			giveValue, err := g.economy.counter.Reference(give)
			if err != nil || giveValue < 1 {
				continue
			}
			qty := int((wantValue + giveValue - 1) / giveValue)
			if qty <= min(spare[give], maxSwapCopies) {
				return domain.QuestReward{
					Kind: domain.RewardSwap, Card: want, Give: []domain.TradeLine{{Key: give, Qty: qty}},
				}, true
			}
		}
	}
	return domain.QuestReward{}, false
}

// spareCopies is the copies above a playset of each card key that the player can part with: those the
// market does not hold. g.mu must be held.
func (g *Game) spareCopies() map[domain.CardKey]int {
	surplus, err := g.collection.DuplicatesAbove(domain.PlaysetSize)
	if err != nil {
		return nil
	}
	spare := make(map[domain.CardKey]int, len(surplus))
	for _, s := range surplus {
		if free := min(s.Quantity, g.collection.Quantity(s.Key)-g.reserved(s.Key)); free > 0 {
			spare[s.Key] = free
		}
	}
	return spare
}
