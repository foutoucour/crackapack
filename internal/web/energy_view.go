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

// energyReading is the gauge as the script that keeps it live reads it: the text of the wait is said
// by the server, so that the page holds no rule of its own about how a duration is told.
type energyReading struct {
	Reserve    int    `json:"reserve"`
	ReserveCap int    `json:"reserveCap"`
	Savings    int    `json:"savings"`
	Next       string `json:"next"`
	// Packs is the wait of each set's booster, so that the buttons that open them follow the gauge.
	Packs []packWait `json:"packs"`
}

// packWait is how long the booster of a set waits for the energy: empty when it can be opened.
type packWait struct {
	Set  string `json:"set"`
	Wait string `json:"wait"`
}

func (v energyView) reading(packs []play.Pack) energyReading {
	reading := energyReading{Reserve: v.Reserve, ReserveCap: v.ReserveCap, Savings: v.Savings, Packs: make([]packWait, 0, len(packs))}
	if v.NextIn > 0 {
		reading.Next = humanDuration(v.NextIn)
	}
	for _, p := range packs {
		wait := packWait{Set: p.SetCode}
		if p.Wait > 0 {
			wait.Wait = humanDuration(p.Wait)
		}
		reading.Packs = append(reading.Packs, wait)
	}
	return reading
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
