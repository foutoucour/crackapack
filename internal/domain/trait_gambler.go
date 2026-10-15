package domain

import "fmt"

// gamblerTrait is the gambler personality: loose and prone to tilt. Each decline in a row raises
// its offer chance by chanceStep up to maxChance, and lowers its margin extra by marginStep toward
// 0 (never below the base margin). Both reset on an accepted trade via the streak counter.
//
// It is immutable and shared across all NPCs of the archetype. It uses no per-NPC state beyond
// what the NPC already tracks (declineStreak, chanceTilt); no randomness is consumed.
type gamblerTrait struct {
	// chanceStep is the extra offer-chance percent added per streak level.
	chanceStep int
	// maxChance is the ceiling of the effective offer chance, in [0, 100].
	maxChance int
	// marginExtra is the extra margin percent at streak 0.
	marginExtra int
	// marginStep is the extra margin percent lost per streak level.
	marginStep int
}

// newGamblerTrait is the TraitConstructor for the gambler trait. It is registered in DefaultTraits.
func newGamblerTrait(params map[string]int, _ TraitEnv) (Trait, error) {
	known := map[string]bool{
		"chance_step_pct":      true,
		"max_offer_chance_pct": true,
		"margin_extra_pct":     true,
		"margin_step_pct":      true,
	}
	for k := range params {
		if !known[k] {
			return nil, fmt.Errorf("gambler: unknown parameter %q", k)
		}
	}
	chanceStep := 4
	maxChance := 30
	marginExtra := 15
	marginStep := 5
	if v, ok := params["chance_step_pct"]; ok {
		chanceStep = v
	}
	if v, ok := params["max_offer_chance_pct"]; ok {
		if v > 100 {
			return nil, fmt.Errorf("gambler: max_offer_chance_pct must be <= 100, got %d", v)
		}
		maxChance = v
	}
	if v, ok := params["margin_extra_pct"]; ok {
		marginExtra = v
	}
	if v, ok := params["margin_step_pct"]; ok {
		marginStep = v
	}
	return &gamblerTrait{
		chanceStep:  chanceStep,
		maxChance:   maxChance,
		marginExtra: marginExtra,
		marginStep:  marginStep,
	}, nil
}

// Name implements Trait.
func (g *gamblerTrait) Name() string { return "gambler" }

// MarginAdd returns max(0, marginExtra - marginStep*streak). The streak is 0 when ctx.NPC is nil,
// giving the maximum extra margin. As the streak rises the margin falls toward 0, then stays there.
// The result is always >= 0 so it never lowers the base margin. An accept resets the streak to 0,
// which raises the margin back to marginExtra with no explicit hook.
func (g *gamblerTrait) MarginAdd(ctx MarginContext) Percent {
	streak := 0
	if ctx.NPC != nil {
		streak = ctx.NPC.DeclineStreak()
	}
	return Percent(max(0, g.marginExtra-g.marginStep*streak))
}

// ReactToDecline raises the NPC's offer chance tilt. The effective chance is:
//
//	base + min(chanceStep*streak, maxChance-base)
//
// clamped so it never falls below base and never exceeds maxChance. The tilt is stored on the NPC
// so EffectiveOfferChancePct reads it; it is ignored when declineStreak is 0, so an accept resets
// it implicitly.
func (g *gamblerTrait) ReactToDecline(ctx DeclineContext) {
	if ctx.NPC == nil {
		return
	}
	n := ctx.NPC
	base := n.OfferChancePct
	ch := max(base, min(g.maxChance, base+g.chanceStep*n.DeclineStreak()))
	n.chanceTilt = ch - base
}
