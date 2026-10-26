package play

import (
	"errors"
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// snapshotCrowd is the crowd as the save holds it: the stream, the tick, and what each NPC holds
// and remembers, its cards compactly, named by dict.
func snapshotCrowd(e *economy, dict *cardDict) (*CrowdSnapshot, error) {
	s, err := e.crowd.State()
	if err != nil {
		return nil, err
	}
	snap := &CrowdSnapshot{Tick: s.Tick, NPCs: make([]NPCSnapshot, len(s.NPCs))}
	for i, n := range s.NPCs {
		snap.NPCs[i] = snapshotNPC(n, dict)
	}
	if snap.Rand, err = streamSnapshot(e.crowdStream); err != nil {
		return nil, err
	}
	return snap, nil
}

func snapshotNPC(n domain.NPCState, dict *cardDict) NPCSnapshot {
	out := NPCSnapshot{Credit: n.Credit, DeclineStreak: n.DeclineStreak, ChanceTilt: n.ChanceTilt}
	out.Held = encodeCounts(n.Cards, dict)
	out.Shelf = encodeShelf(n.Shelf, dict)
	out.Wishes = encodeWishes(n.Wishes, dict)
	if r := n.Retry; r.Limit != 0 || r.Pending || r.Used != 0 || r.Day != 0 || len(r.Avoid) > 0 {
		out.Retry = &RetrySnapshot{Limit: r.Limit, Pending: r.Pending, Used: r.Used, Day: r.Day}
		out.Retry.Avoid = encodeKeys(r.Avoid, dict)
	}
	return out
}

// restoreCrowd puts the NPCs of snap in the crowd of e. A save with no NPCs, or whose crowd has
// another number of NPCs than the game's, which a change of the population makes, keeps the game's
// new crowd, see docs/adr/0062-a-save-whose-npcs-do-not-fit-gets-the-games-new-ones.md, and reset is
// true.
func restoreCrowd(e *economy, snap *CrowdSnapshot, dict *cardDict) (reset bool, err error) {
	switch {
	case snap == nil:
		return e.crowd != nil, nil
	case e.crowd == nil:
		return false, errors.New("counter: the save has NPCs, and this game has none")
	}
	current, err := e.crowd.State()
	if err != nil {
		return false, err
	}
	if len(snap.NPCs) != len(current.NPCs) {
		return true, nil
	}
	state := domain.CrowdState{Tick: snap.Tick, NPCs: make([]domain.NPCState, len(snap.NPCs))}
	for i, n := range snap.NPCs {
		if state.NPCs[i], err = npcState(n, dict); err != nil {
			return false, fmt.Errorf("counter: NPC %d: %w", i, err)
		}
	}
	if err := e.crowdStream.Restore(snap.Rand.Seed, snap.Rand.State); err != nil {
		return false, fmt.Errorf("counter: crowd: %w", err)
	}
	return false, e.crowd.Restore(state)
}

func npcState(n NPCSnapshot, dict *cardDict) (domain.NPCState, error) {
	if n.Cards != nil && n.Held != "" {
		return domain.NPCState{}, errors.New("the cards are listed both one by one and compactly")
	}
	out := domain.NPCState{Credit: n.Credit, DeclineStreak: n.DeclineStreak, ChanceTilt: n.ChanceTilt}
	var errs [5]error
	if n.Cards != nil {
		out.Cards, errs[0] = cardMap(n.Cards)
	} else {
		out.Cards, errs[0] = decodeCounts(n.Held, dict)
	}
	out.Shelf, errs[1] = decodeShelf(n.Shelf, dict)
	out.Wishes, errs[2] = decodeWishes(n.Wishes, dict)
	if r := n.Retry; r != nil {
		out.Retry = domain.RetryState{Limit: r.Limit, Pending: r.Pending, Used: r.Used, Day: r.Day}
		out.Retry.Avoid, errs[3] = decodeKeys(r.Avoid, dict)
	}
	return out, errors.Join(errs[:]...)
}

// snapshotMarket is the market as the save holds it.
func snapshotMarket(m *domain.Market) *MarketSnapshot {
	s := m.State()
	snap := &MarketSnapshot{NextID: s.NextID}
	for _, r := range s.Requests {
		snap.Requests = append(snap.Requests, RequestSnapshot{ID: r.ID, Card: r.Card.ID, Finish: r.Card.Finish, At: r.At, Asked: r.Asked, Direct: r.Direct})
	}
	for _, l := range s.Listings {
		snap.Listings = append(snap.Listings, ListingSnapshot{ID: l.ID, Card: l.Card.ID, Finish: l.Card.Finish, Price: l.Price, At: l.At})
	}
	for _, a := range s.Answers {
		snap.Answers = append(snap.Answers, answerSnapshot(a))
	}
	for _, p := range s.Pending {
		snap.Pending = append(snap.Pending, PendingSnapshot(p))
	}
	for _, r := range s.Results {
		snap.Results = append(snap.Results, ResultSnapshot{Answer: answerSnapshot(r.Answer), Outcome: outcomeName(r.Outcome), At: r.At})
	}
	return snap
}

func answerSnapshot(a domain.MarketAnswer) AnswerSnapshot {
	out := AnswerSnapshot{
		ID: a.ID, To: a.To, Kind: kindName(a.Kind), Card: a.Card.ID, Finish: a.Card.Finish, NPC: a.NPC,
		Coins: a.Coins, Cards: lineCounts(a.Cards), At: a.At, Expires: a.Expires,
	}
	if c := a.Counter; c != nil {
		out.Counter = &CounterOfferSnapshot{Coins: c.Coins, Cards: lineCounts(c.Cards), ReplyAt: c.ReplyAt}
	}
	return out
}

// restoreMarket puts the market of snap in e, over the player's collection. reset drops what the
// NPCs said and were to say, for a crowd that is new.
func restoreMarket(e *economy, snap *MarketSnapshot, reset bool) error {
	switch {
	case snap == nil:
		return nil
	case e.market == nil:
		return errors.New("counter: the save has a market, and this game has none")
	}
	s := domain.MarketState{NextID: snap.NextID}
	for _, r := range snap.Requests {
		s.Requests = append(s.Requests, domain.MarketRequest{ID: r.ID, Card: domain.CardKey{ID: r.Card, Finish: r.Finish}, At: r.At, Asked: r.Asked, Direct: r.Direct})
	}
	for _, l := range snap.Listings {
		s.Listings = append(s.Listings, domain.MarketListing{ID: l.ID, Card: domain.CardKey{ID: l.Card, Finish: l.Finish}, Price: l.Price, At: l.At})
	}
	for _, a := range snap.Answers {
		answer, err := marketAnswer(a)
		if err != nil {
			return fmt.Errorf("market: %w", err)
		}
		s.Answers = append(s.Answers, answer)
	}
	for _, p := range snap.Pending {
		s.Pending = append(s.Pending, domain.MarketPending(p))
	}
	for _, r := range snap.Results {
		answer, err := marketAnswer(r.Answer)
		if err != nil {
			return fmt.Errorf("market: %w", err)
		}
		outcome, ok := outcomeOf(r.Outcome)
		if !ok {
			return fmt.Errorf("market: result of answer %d: unknown outcome %q", r.Answer.ID, r.Outcome)
		}
		s.Results = append(s.Results, domain.MarketResult{Answer: answer, Outcome: outcome, At: r.At})
	}
	if reset {
		s.Answers, s.Pending = nil, nil
	}
	return e.market.Restore(s)
}

func marketAnswer(a AnswerSnapshot) (domain.MarketAnswer, error) {
	kind, ok := kindOf(a.Kind)
	if !ok {
		return domain.MarketAnswer{}, fmt.Errorf("answer %d: unknown kind %q", a.ID, a.Kind)
	}
	out := domain.MarketAnswer{
		ID: a.ID, To: a.To, Kind: kind, Card: domain.CardKey{ID: a.Card, Finish: a.Finish}, NPC: a.NPC,
		Coins: a.Coins, Cards: countLines(a.Cards), At: a.At, Expires: a.Expires,
	}
	if c := a.Counter; c != nil {
		out.Counter = &domain.MarketCounter{Coins: c.Coins, Cards: countLines(c.Cards), ReplyAt: c.ReplyAt}
	}
	return out, nil
}

func lineCounts(lines []domain.TradeLine) []CardCount {
	var out []CardCount
	for _, l := range lines {
		out = append(out, CardCount{ID: l.Key.ID, Finish: l.Key.Finish, Quantity: l.Qty})
	}
	return out
}

func countLines(counts []CardCount) []domain.TradeLine {
	var out []domain.TradeLine
	for _, c := range counts {
		out = append(out, domain.TradeLine{Key: domain.CardKey{ID: c.ID, Finish: c.Finish}, Qty: c.Quantity})
	}
	return out
}

func kindName(k domain.AnswerKind) string {
	switch k {
	case domain.AnswerAsk:
		return "ask"
	case domain.AnswerOffer:
		return "offer"
	}
	return ""
}

func kindOf(name string) (domain.AnswerKind, bool) {
	for _, k := range []domain.AnswerKind{domain.AnswerAsk, domain.AnswerOffer} {
		if kindName(k) == name {
			return k, true
		}
	}
	return 0, false
}

func outcomeName(o domain.MarketOutcome) string {
	switch o {
	case domain.MarketAccepted:
		return "accepted"
	case domain.MarketRefused:
		return "refused"
	case domain.MarketExpired:
		return "expired"
	case domain.MarketFailed:
		return "failed"
	}
	return ""
}

func outcomeOf(name string) (domain.MarketOutcome, bool) {
	for _, o := range []domain.MarketOutcome{domain.MarketAccepted, domain.MarketRefused, domain.MarketExpired, domain.MarketFailed} {
		if outcomeName(o) == name {
			return o, true
		}
	}
	return 0, false
}
