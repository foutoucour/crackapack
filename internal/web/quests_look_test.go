package web_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func iconSet() domain.SetCatalogue {
	set := playtest.TinySet("S", 6, 2)
	set.IconCode = "SS"
	return set
}

func winCardRules(target int64) domain.QuestRules {
	tpl := questTemplate(domain.QuestDaily, domain.QuestOpenBoosters, target, domain.RewardWinCard)
	return domain.QuestRules{DailyCount: 1, Templates: []domain.QuestTemplate{tpl}}
}

func TestQuests_theCardAnOpenQuestGivesShowsTheIconOfItsSet(t *testing.T) {
	t.Parallel()
	shop := newQuestShop(t, winCardRules(2), iconSet())

	body := shop.get(t, "/quests")

	mustContain(t, "the quests page", body, `<span class="set-icon-chip rarity-`, `class="set-icon set-icon-ss rarity-`)
}

func TestQuests_theCardsADoneQuestGaveShowTheIconOfTheirSet(t *testing.T) {
	t.Parallel()
	shop := newQuestShop(t, winCardRules(1), iconSet())
	shop.open(t, 1)

	body := shop.get(t, "/quests")

	mustContain(t, "the quests page", body, "You got", `class="set-icon set-icon-ss rarity-`)
}

func TestQuests_aDoneQuestHasACheckMarkAndAnOpenOneHasNone(t *testing.T) {
	t.Parallel()
	done := newQuestShop(t, winCardRules(1), iconSet())
	done.open(t, 1)
	open := newQuestShop(t, winCardRules(2), iconSet())

	mustContain(t, "the page of a done quest", done.get(t, "/quests"), `<svg class="check-mark"`)
	mustNotContain(t, "the page of an open quest", open.get(t, "/quests"), `check-mark`)
}
