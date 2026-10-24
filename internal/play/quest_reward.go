package play

import "github.com/foutoucour/crackapack/internal/domain"

// finishQuest gives the reward of a quest whose action is done, and marks the quest done. A reward
// that can no longer be given, because the card or the spare copies it names are gone, or because the
// booster cannot be drawn, marks the quest lost and changes nothing else. g.mu must be held.
func (g *Game) finishQuest(q *quest) {
	got, ok := g.giveReward(q.Reward)
	if !ok {
		q.state = QuestLost
		return
	}
	q.state, q.got = QuestDone, got
}

// giveReward gives reward to the player, and returns the cards it added.
func (g *Game) giveReward(reward domain.QuestReward) (got []domain.CardKey, ok bool) {
	switch reward.Kind {
	case domain.RewardWinCard:
		g.addCopy(reward.Card)
		return []domain.CardKey{reward.Card}, true
	case domain.RewardFoilCard:
		return g.foilCopy(reward.Card)
	case domain.RewardPremiumBooster:
		return g.openPremium(reward.Set)
	case domain.RewardUnlockSet:
		g.unlockSet(reward.Set)
		return nil, true
	case domain.RewardDoubleDuplicates:
		return nil, g.doubleDuplicates(reward.Set)
	case domain.RewardSwap:
		return g.swap(reward)
	}
	return nil, false
}

// addCopy adds one copy of key to the collection. A positive quantity cannot fail.
func (g *Game) addCopy(key domain.CardKey) {
	_ = g.collection.Add(key, 1)
}

// foilCopy turns one non-foil copy of key into a foil, when the player holds one the market does not.
func (g *Game) foilCopy(key domain.CardKey) ([]domain.CardKey, bool) {
	if g.collection.Quantity(key)-g.reserved(key) < 1 {
		return nil, false
	}
	_ = g.collection.Remove(key, 1)
	foil := domain.CardKey{ID: key.ID, Finish: domain.FinishFoil}
	g.addCopy(foil)
	return []domain.CardKey{foil}, true
}

// openPremium opens a booster of setCode that holds no card the player has a playset of, see
// [domain.LackingCatalogue], and adds its cards to the collection.
func (g *Game) openPremium(setCode string) ([]domain.CardKey, bool) {
	cat, ok := g.set(setCode)
	if !ok {
		return nil, false
	}
	booster, err := g.opener.Open(domain.BoosterLayoutClassic, domain.LackingCatalogue(cat, g.collection.Copies))
	if err != nil {
		return nil, false
	}
	g.collection.AddBooster(booster)
	return booster.Cards, true
}

// unlockSet opens setCode, as the unlock of a binder does, for good.
func (g *Game) unlockSet(setCode string) {
	if s := g.staging; s != nil {
		s.unlocked[setCode] = true
		g.openCrowdSets(g.economy)
	}
}

// doubleDuplicates doubles the copies above a playset of the cards of the binder of setCode: with
// five copies the player has six, with six eight. It reports false for a set the game does not have.
func (g *Game) doubleDuplicates(setCode string) bool {
	binder, ok := g.binders[setCode]
	if !ok {
		return false
	}
	surplus, err := g.collection.DuplicatesAbove(domain.PlaysetSize)
	if err != nil {
		return false
	}
	for _, s := range surplus {
		if _, inBinder := binder.Cards[s.Key.ID]; inBinder {
			_ = g.collection.Add(s.Key, s.Quantity)
		}
	}
	return true
}

// swap takes the copies the reward names, which must still be spare, and gives the card of the
// reward for them.
func (g *Game) swap(reward domain.QuestReward) ([]domain.CardKey, bool) {
	spare := g.spareCopies()
	for _, line := range reward.Give {
		if spare[line.Key] < line.Qty {
			return nil, false
		}
	}
	for _, line := range reward.Give {
		_ = g.collection.Remove(line.Key, line.Qty)
	}
	g.addCopy(reward.Card)
	return []domain.CardKey{reward.Card}, true
}
