package play

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// QuestsSnapshot is the quests of a game as a save holds them: the stream that draws them, the day
// and the week they were drawn for, and the quests of each. What the player did since the last count
// is not kept: a loaded game starts counting from the collection and the ledger it loads.
type QuestsSnapshot struct {
	Rand   OpenerSnapshot  `json:"rand"`
	Day    int64           `json:"day"`
	Week   int64           `json:"week"`
	Daily  []QuestSnapshot `json:"daily,omitempty"`
	Weekly []QuestSnapshot `json:"weekly,omitempty"`
}

// QuestSnapshot is one quest, by the names a configuration writes its action and its reward with.
type QuestSnapshot struct {
	Action   string         `json:"action"`
	Set      string         `json:"set,omitempty"`
	Colour   string         `json:"colour,omitempty"`
	Rarity   string         `json:"rarity,omitempty"`
	Target   int64          `json:"target"`
	Progress int64          `json:"progress"`
	State    string         `json:"state"`
	Reward   RewardSnapshot `json:"reward"`
	// Got is the cards the reward added, once the quest is done.
	Got []CardCount `json:"got,omitempty"`
}

// RewardSnapshot is the reward of a quest: its kind, the card and the set it names, and for a swap
// the copies it takes.
type RewardSnapshot struct {
	Kind   string        `json:"kind"`
	Card   domain.CardID `json:"card,omitempty"`
	Finish domain.Finish `json:"finish,omitempty"`
	Set    string        `json:"set,omitempty"`
	Give   []CardCount   `json:"give,omitempty"`
}

// questStateNames is how the states of a quest are written in a save.
func questStateNames() map[QuestState]string {
	return map[QuestState]string{QuestOpen: "open", QuestDone: "done", QuestLost: "lost"}
}

// snapshotQuests is the quests of the game, nil in a game without them. g.mu must be held.
func (g *Game) snapshotQuests() (*QuestsSnapshot, error) {
	b := g.quests
	if b == nil {
		return nil, nil
	}
	state, err := b.stream.State()
	if err != nil {
		return nil, err
	}
	return &QuestsSnapshot{
		Rand: OpenerSnapshot{Seed: b.stream.Seed(), State: state},
		Day:  b.day, Week: b.week,
		Daily: snapshotQuestList(b.daily), Weekly: snapshotQuestList(b.weekly),
	}, nil
}

func snapshotQuestList(quests []*quest) []QuestSnapshot {
	var out []QuestSnapshot
	for _, q := range quests {
		out = append(out, QuestSnapshot{
			Action: q.Action.String(), Set: q.Set, Colour: string(q.Filter.Colour), Rarity: string(q.Filter.Rarity),
			Target: q.Target, Progress: q.Progress, State: questStateNames()[q.state],
			Reward: RewardSnapshot{
				Kind: q.Reward.Kind.String(), Card: q.Reward.Card.ID, Finish: q.Reward.Card.Finish, Set: q.Reward.Set,
				Give: countsOf(q.Reward.Give),
			},
			Got: countKeys(q.got),
		})
	}
	return out
}

// countsOf lists the lines as the counts a save holds.
func countsOf(lines []domain.TradeLine) []CardCount {
	var out []CardCount
	for _, l := range lines {
		out = append(out, CardCount{ID: l.Key.ID, Finish: l.Key.Finish, Quantity: l.Qty})
	}
	return out
}

// countKeys is the copies of each distinct key, in key order.
func countKeys(keys []domain.CardKey) []CardCount {
	n := make(map[domain.CardKey]int, len(keys))
	for _, key := range keys {
		n[key]++
	}
	var out []CardCount
	for key, qty := range n {
		out = append(out, CardCount{ID: key.ID, Finish: key.Finish, Quantity: qty})
	}
	slices.SortFunc(out, func(a, b CardCount) int {
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Finish, b.Finish))
	})
	return out
}

// restoreQuests checks the quests of snap and returns what puts them in the game. A quest of a card or
// a set that the game does not have any more is dropped, as a hunt is; a name that is none is an
// error. Nothing changes until commit is called, so that a save refused later leaves the game as it
// was. g.mu must be held, and the game must have quests.
func (g *Game) restoreQuests(snap QuestsSnapshot) (commit func(), err error) {
	stream := NewStream(snap.Rand.Seed)
	if err := stream.Restore(snap.Rand.Seed, snap.Rand.State); err != nil {
		return nil, fmt.Errorf("quests: %w", err)
	}
	daily, err := g.restoreQuestList(snap.Daily, domain.QuestDaily)
	if err != nil {
		return nil, err
	}
	weekly, err := g.restoreQuestList(snap.Weekly, domain.QuestWeekly)
	if err != nil {
		return nil, err
	}
	b := g.quests
	return func() {
		*b.stream = *stream
		b.rng = rand.New(b.stream)
		b.day, b.week, b.daily, b.weekly = snap.Day, snap.Week, daily, weekly
	}, nil
}

func (g *Game) restoreQuestList(snaps []QuestSnapshot, period domain.QuestPeriod) ([]*quest, error) {
	var out []*quest
	for i, s := range snaps {
		q, ok, err := g.restoreQuest(s, period)
		if err != nil {
			return nil, fmt.Errorf("quests: %s quest %d: %w", period, i, err)
		}
		if ok {
			out = append(out, q)
		}
	}
	return out, nil
}

// restoreQuest is the quest s stands for, false when it names a card or a set the game does not have.
func (g *Game) restoreQuest(s QuestSnapshot, period domain.QuestPeriod) (*quest, bool, error) {
	action, err := domain.ParseQuestAction(s.Action)
	if err != nil {
		return nil, false, err
	}
	kind, err := domain.ParseQuestRewardKind(s.Reward.Kind)
	if err != nil {
		return nil, false, err
	}
	state := QuestState(0)
	for st, name := range questStateNames() {
		if name == s.State {
			state = st
		}
	}
	if state == 0 {
		return nil, false, fmt.Errorf("state %q is unknown", s.State)
	}
	q := &quest{state: state, Quest: domain.Quest{
		Period: period, Action: action, Set: s.Set,
		Filter: domain.CardFilter{Colour: domain.ColourGroup(s.Colour), Rarity: domain.Rarity(s.Rarity)},
		Target: s.Target, Progress: s.Progress,
		Reward: domain.QuestReward{Kind: kind, Card: domain.CardKey{ID: s.Reward.Card, Finish: s.Reward.Finish}, Set: s.Reward.Set},
	}}
	if !g.knowsQuest(q.Quest, s) {
		return nil, false, nil
	}
	for _, c := range s.Reward.Give {
		q.Reward.Give = append(q.Reward.Give, domain.TradeLine{Key: domain.CardKey{ID: c.ID, Finish: c.Finish}, Qty: c.Quantity})
	}
	for _, c := range s.Got {
		for range c.Quantity {
			q.got = append(q.got, domain.CardKey{ID: c.ID, Finish: c.Finish})
		}
	}
	return q, true, nil
}

// knowsQuest reports whether every set and card that the quest names is in the game.
func (g *Game) knowsQuest(q domain.Quest, s QuestSnapshot) bool {
	for _, code := range []string{q.Set, q.Reward.Set} {
		if _, ok := g.set(code); code != "" && !ok {
			return false
		}
	}
	keys := []domain.CardKey{q.Reward.Card}
	for _, c := range append(slices.Clone(s.Reward.Give), s.Got...) {
		keys = append(keys, domain.CardKey{ID: c.ID, Finish: c.Finish})
	}
	for _, key := range keys {
		if key.ID == "" {
			continue
		}
		if card, ok := g.cards[key.ID]; !ok || !slices.Contains(card.Finishes, key.Finish) {
			return false
		}
	}
	return true
}
