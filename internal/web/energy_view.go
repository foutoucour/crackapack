package web

import (
	"fmt"
	"time"

	"github.com/foutoucour/crackapack/internal/play"
)

// energyView is the gauge in the header of every page.
type energyView struct {
	Reserve    int
	ReserveCap int
	Savings    int
	// NextIn is the time until the energy next grows by a point, 0 when it cannot grow any more.
	NextIn time.Duration
}

func newEnergyView(s play.EnergyStatus) energyView {
	return energyView{Reserve: s.Reserve, ReserveCap: s.ReserveCap, Savings: s.Savings, NextIn: s.NextIn}
}

// humanDuration says d the way a player counts it: whole minutes rounded up, in hours and minutes
// past the hour, and never as zero minutes.
func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "less than a minute"
	}
	minutes := int((d + time.Minute - 1) / time.Minute)
	hours, rest := minutes/60, minutes%60
	switch {
	case hours == 0:
		return fmt.Sprintf("%d min", minutes)
	case rest == 0:
		return fmt.Sprintf("%d h", hours)
	default:
		return fmt.Sprintf("%d h %d min", hours, rest)
	}
}
