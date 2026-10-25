package web

import (
	"fmt"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

// boosterPrice is what the next booster of a set costs the player now, told as one price: the
// energy it takes, and the coins on top. The coins are the set's fee, and the top-up when the
// player lacks energy, so a player short of energy sees the same booster at another price, not
// another button.
type boosterPrice struct {
	Energy int
	Coins  domain.Coins
	// TopUp is true when the coins pay for energy the player lacks, which the form must say.
	TopUp bool
	// Short is the energy the player lacks, set with TopUp.
	Short int
	// Wait is the time until the energy alone pays for the booster, 0 when it does.
	Wait time.Duration
}

func newBoosterPrice(p play.Pack) boosterPrice {
	if p.TopUp != nil {
		return boosterPrice{Energy: p.Cost - p.TopUp.Units, Coins: p.TopUp.Total, TopUp: true, Short: p.TopUp.Units, Wait: p.Wait}
	}
	return boosterPrice{Energy: p.Cost, Coins: p.Fee, Wait: p.Wait}
}

// Text is the price as the button shows it after the energy icon: "5", or "3 + ◎80" with coins.
func (p boosterPrice) Text() string {
	if p.Coins == 0 {
		return fmt.Sprint(p.Energy)
	}
	return fmt.Sprintf("%d + %s", p.Energy, coinsText(p.Coins))
}

// Label is the price in words, for the screen reader that cannot see the energy icon.
func (p boosterPrice) Label() string {
	if p.Coins == 0 {
		return fmt.Sprintf("%d energy", p.Energy)
	}
	return fmt.Sprintf("%d energy and %s", p.Energy, coinsText(p.Coins))
}

// Hint is the line under the name of a booster the energy does not pay for yet: how much is
// missing and how long to wait for it when coins can pay for it, how long to wait when they cannot.
// It is empty when the energy pays for the booster.
func (p boosterPrice) Hint() string {
	switch {
	case p.Wait == 0:
		return ""
	case p.TopUp:
		return fmt.Sprintf("Short of %d energy · or wait %s", p.Short, humanDuration(p.Wait))
	default:
		return "Ready in " + humanDuration(p.Wait)
	}
}

// Blocked is true when the booster cannot be opened now: it waits for energy and no top-up pays
// for it.
func (p boosterPrice) Blocked() bool { return p.Wait > 0 && !p.TopUp }
