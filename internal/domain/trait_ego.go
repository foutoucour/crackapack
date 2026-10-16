package domain

import (
	"fmt"
)

// egoTrait is the ego personality: proud, answers a decline with a follow-up offer, never concedes
// a player counter, and demands more for the cards it has held longest.
//
// It is immutable and shared across all NPCs of the archetype; per-NPC state lives on NPC.retry.
type egoTrait struct {
	// maxRetries is the number of follow-up offers per day after a decline; 0 means never retry.
	maxRetries int
	// endowPctPerDay is the margin added per whole day the oldest given card was held on the shelf.
	endowPctPerDay int
}

// retryState holds the follow-up budget for one NPC. It is owned by the NPC and written by the
// ego reactor and ProposeFromNPCs. The zero value means no retry is armed for this NPC.
//
// Per-day budget: a chain crossing midnight gets a fresh budget on the new day. The streak stays
// high after the chain ends, so using it as the limit guard would allow unlimited offers; the
// per-day reading gives "at most max_retries + 1 offers per ego per day" (one rolled offer, then
// up to limit follow-ups, then the ego holds for the rest of the day).
type retryState struct {
	// limit is the maximum follow-up offers per day; 0 means this NPC has never retried.
	limit int
	// pending is true when a decline is waiting for its follow-up.
	pending bool
	// used is the number of follow-ups made on day.
	used int
	// day is the day that used belongs to.
	day int
	// avoid is the card keys of every give the player declined in this chain. A follow-up never
	// offers a key in this list, preventing A→B→A flip-flops across the chain.
	avoid []CardKey
}

// retryStep is what ProposeFromNPCs should do with an NPC on this tick.
type retryStep int

const (
	// retryRoll: roll the offer-chance check as normal.
	retryRoll retryStep = iota
	// retryHold: skip this NPC; the budget for today is exhausted.
	retryHold
	// retryFollowUp: skip the roll and make a follow-up offer.
	retryFollowUp
)

// step decides what ProposeFromNPCs should do with this NPC on a given tick.
// streak is npc.declineStreak; day is x.day.
func (r *retryState) step(streak, day int) retryStep {
	if streak == 0 {
		// An accept reset the streak; clear all retry state.
		*r = retryState{}
		return retryRoll
	}
	// Lazy reset of the daily used counter.
	if r.day != day {
		r.day, r.used = day, 0
	}
	if r.used >= r.limit {
		// Budget exhausted for today; no more follow-ups.
		r.pending = false
		return retryHold
	}
	if !r.pending {
		return retryRoll
	}
	return retryFollowUp
}

// settle records the outcome of the proposal for this NPC:
//   - followUp && ok: a follow-up was made; pending is cleared and used is incremented.
//   - followUp && !ok: no different trade exists; pending is cleared and the budget is exhausted.
//   - !followUp && ok && limit > 0: a fresh rolled offer was made; avoid is cleared (new chain).
func (r *retryState) settle(followUp, ok bool) {
	switch {
	case followUp && ok:
		r.pending = false
		r.used++
	case followUp && !ok:
		r.pending = false
		r.used = r.limit // exhausted: hold for the day
	case !followUp && ok && r.limit > 0:
		r.avoid = nil // fresh chain: a new rolled offer starts a new avoid list
	}
}

// arm arms the retry state after a decline of an NPC's own offer: it sets the limit, marks a
// follow-up as pending, and appends the give card keys to the avoid list.
func (r *retryState) arm(limit int, gives []TradeLine) {
	r.limit = limit
	r.pending = true
	for _, l := range gives {
		r.avoid = append(r.avoid, l.Key)
	}
}

// newEgoTrait is the TraitConstructor for the ego trait. It is registered in DefaultTraits.
func newEgoTrait(params map[string]int, _ TraitEnv) (Trait, error) {
	known := map[string]bool{"max_retries": true, "endowment_pct_per_day": true}
	for k := range params {
		if !known[k] {
			return nil, fmt.Errorf("ego: unknown parameter %q", k)
		}
	}
	maxRetries := 3
	endowPctPerDay := 5
	if v, ok := params["max_retries"]; ok {
		maxRetries = v
	}
	if v, ok := params["endowment_pct_per_day"]; ok {
		endowPctPerDay = v
	}
	return &egoTrait{maxRetries: maxRetries, endowPctPerDay: endowPctPerDay}, nil
}

// Name implements Trait.
func (e *egoTrait) Name() string { return "ego" }

// MarginAdd adds endowPctPerDay per whole day the oldest given card was held on the shelf. It
// prices the oldest card (not a sum over all cards, which would grow with card count and hit the
// cap at once). Returns 0 when ctx.NPC is nil or no given card is on the shelf.
func (e *egoTrait) MarginAdd(ctx MarginContext) Percent {
	if ctx.NPC == nil {
		return 0
	}
	oldest := -1
	for _, l := range ctx.Offer.NPCGives {
		tick, ok := ctx.NPC.oldestShelfTick(l.Key)
		if !ok {
			continue
		}
		if oldest < 0 || tick < oldest {
			oldest = tick
		}
	}
	if oldest < 0 {
		return 0
	}
	days := max(0, (ctx.Tick-oldest)/TicksPerDay)
	return Percent(days * e.endowPctPerDay)
}

// VetoCounter refuses every player counter. The ego never concedes on its final word. ADR-0024
// says a vetoed counter is a decline of the NPC's original offer, so the reactor will arm a retry.
func (e *egoTrait) VetoCounter(_ Offer) bool { return true }

// ReactToDecline arms a follow-up only for declines of the NPC's own OriginNPC proposals (including
// a refused player counter, where ctx.Offer is the original OriginNPC offer). It ignores nil NPCs,
// a zero maxRetries limit, and OriginNPCCounter declines from the player.
//
// The trait arms only NPC-origin declines to prevent the avoid list from growing without bound:
// the exploiter declines thousands of player-started counters and each one would append to avoid.
func (e *egoTrait) ReactToDecline(ctx DeclineContext) {
	if ctx.NPC == nil || e.maxRetries == 0 {
		return
	}
	if ctx.Offer.Origin != OriginNPC {
		return
	}
	ctx.NPC.retry.arm(e.maxRetries, ctx.Offer.NPCGives)
}
